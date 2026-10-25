package author

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araihu/balemoh/plugin-sdk/credential"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"uuid"
)

type DevOptions struct {
	Source, Host, SDK, Fixtures string
	Watch                       bool
}

func Dev(ctx context.Context, o DevOptions, out io.Writer) error {
	source, err := filepath.Abs(o.Source)
	if err != nil {
		return err
	}
	host, err := filepath.Abs(o.Host)
	if err != nil {
		return err
	}
	probe := exec.CommandContext(ctx, host, "--plugin-host-info")
	raw, err := probe.Output()
	if err != nil {
		return errors.New("host does not report plugin compatibility")
	}
	var info struct{ Protocol, UIProfile string }
	if json.Unmarshal(raw, &info) != nil || info.Protocol != manifest.Protocol || info.UIProfile != manifest.UIProfile {
		return errors.New("incompatible development host")
	}
	dir, err := os.MkdirTemp("", "balemoh-plugin-dev-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	modfile := ""
	if o.SDK != "" {
		sdk, e := filepath.Abs(o.SDK)
		if e != nil {
			return e
		}
		data, e := os.ReadFile(filepath.Join(source, "go.mod"))
		if e != nil {
			return e
		}
		modfile = filepath.Join(dir, "development.mod")
		if e = os.WriteFile(modfile, data, 0600); e != nil {
			return e
		}
		cmd := exec.CommandContext(ctx, "go", "mod", "edit", "-modfile", modfile, "-replace", "github.com/araihu/balemoh/plugin-sdk="+sdk)
		cmd.Dir = source
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if e = cmd.Run(); e != nil {
			return e
		}
	}
	token := strings.ReplaceAll(uuid.NewV4().String()+uuid.NewV4().String(), "-", "")
	tokenFile := filepath.Join(dir, "service-token")
	if err = os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		return err
	}
	fixtures := map[string]map[string]json.RawMessage{}
	if o.Fixtures != "" {
		data, e := os.ReadFile(o.Fixtures)
		if e != nil || len(data) > 1<<20 || json.Unmarshal(data, &fixtures) != nil {
			return errors.New("invalid preview fixtures")
		}
	}
	fingerprint, err := sourceDigest(source)
	if err != nil {
		return err
	}
	generation := 0
	for {
		packageDir := filepath.Join(dir, fmt.Sprintf("package-%d", generation))
		generation++
		args := []string{"run", "-mod=mod"}
		if modfile != "" {
			args = append(args, "-modfile", modfile)
		}
		args = append(args, "github.com/a-h/templ/cmd/templ", "generate")
		generate := exec.CommandContext(ctx, "go", args...)
		generate.Dir = source
		generate.Env = append(os.Environ(), "GOWORK=off")
		generate.Stderr = os.Stderr
		if err = generate.Run(); err != nil {
			return errors.New("author templ generation failed")
		}
		if modfile != "" {
			tidy := exec.CommandContext(ctx, "go", "mod", "tidy", "-modfile", modfile)
			tidy.Dir = source
			tidy.Env = append(os.Environ(), "GOWORK=off")
			tidy.Stderr = os.Stderr
			if err = tidy.Run(); err != nil {
				return err
			}
		}
		p, e := Package(ctx, source, packageDir, modfile, []string{runtime.GOOS + "/" + runtime.GOARCH})
		if e != nil {
			return e
		}
		digest, e := p.ContentDigest()
		if e != nil {
			return e
		}
		uiListener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			return e
		}
		uiAddress := uiListener.Addr().String()
		uiListener.Close()
		uiURL := "http://" + uiAddress
		apiListener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			return e
		}
		fixture := newPreview(p, token, uiURL, fixtures)
		apiServer := &http.Server{Handler: fixture}
		go apiServer.Serve(apiListener)
		config := filepath.Join(dir, "ui.json")
		data, _ := json.Marshal(map[string]any{"installations": []any{map[string]any{"package": packageDir, "manifestDigest": p.Digest, "packageDigest": digest, "connectionKey": "preview", "previewWidgets": []any{}}}})
		if e = os.WriteFile(config, data, 0600); e != nil {
			apiServer.Close()
			return e
		}
		process := exec.Command(host)
		process.Dir = dir
		process.Env = []string{"PATH=" + os.Getenv("PATH"), "BALEMOH_UI_HTTP_ADDR=" + uiAddress, "BALEMOH_UI_API_BASE_URL=http://" + apiListener.Addr().String(), "BALEMOH_UI_PLUGIN_CONFIG=" + config, "BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE=" + tokenFile, "BALEMOH_UI_ICON_CACHE_DIR=" + filepath.Join(dir, "icons"), "BALEMOH_UI_ICON_STARTUP_TIMEOUT=10ms"}
		process.Stderr = os.Stderr
		if e = process.Start(); e != nil {
			apiServer.Close()
			return e
		}
		done := make(chan error, 1)
		go func() { done <- process.Wait() }()
		fmt.Fprintf(out, "Preview %s\nStates %s/preview/{loading,empty,fresh,stale,error}\n", uiURL, "http://"+apiListener.Addr().String())
		ticker := time.NewTicker(500 * time.Millisecond)
		restart := false
		for !restart {
			select {
			case <-ctx.Done():
				restart = true
			case e = <-done:
				ticker.Stop()
				apiServer.Close()
				if ctx.Err() != nil {
					return nil
				}
				return e
			case <-ticker.C:
				if o.Watch {
					next, e := sourceDigest(source)
					if e != nil {
						continue
					}
					if next != fingerprint {
						fingerprint = next
						restart = true
					}
				}
			}
		}
		ticker.Stop()
		_ = process.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = process.Process.Kill()
			<-done
		}
		apiServer.Close()
		if ctx.Err() != nil {
			return nil
		}
	}
}
func sourceDigest(root string) (string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "dist" || d.Name() == "bin") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_templ.go") {
			return nil
		}
		if d.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, p := range files {
		hash.Write([]byte(p))
		data, e := os.ReadFile(p)
		if e != nil {
			return "", e
		}
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type preview struct {
	mu               sync.Mutex
	pkg              *manifest.Package
	token, ui, state string
	fixtures         map[string]map[string]json.RawMessage
	revision         uint64
	observed         time.Time
}

const previewConnection = "00000000-0000-4000-8000-000000000001"

func newPreview(p *manifest.Package, token, ui string, f map[string]map[string]json.RawMessage) *preview {
	return &preview{pkg: p, token: token, ui: ui, state: "fresh", fixtures: f, revision: 1, observed: time.Now().UTC()}
}
func (p *preview) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	encode := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if strings.HasPrefix(r.URL.Path, "/preview/") {
		state := strings.TrimPrefix(r.URL.Path, "/preview/")
		if !strings.Contains("|loading|empty|fresh|stale|error|", "|"+state+"|") {
			http.NotFound(w, r)
			return
		}
		p.mu.Lock()
		p.state = state
		p.revision++
		if state == "fresh" || state == "empty" {
			p.observed = time.Now().UTC()
		}
		p.mu.Unlock()
		http.Redirect(w, r, p.ui, 303)
		return
	}
	if r.URL.Path == "/healthz" {
		encode(map[string]string{"status": "ok"})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/plugin") && !credential.MatchBearer(r.Header.Get("Authorization"), p.token) {
		w.WriteHeader(401)
		return
	}
	p.mu.Lock()
	state, revision, observed := p.state, fmt.Sprint(p.revision), p.observed
	p.mu.Unlock()
	queries := []string{}
	for _, q := range p.pkg.Manifest.Queries {
		queries = append(queries, q.ID)
	}
	connection := map[string]any{"id": previewConnection, "key": "preview", "name": "Development fixture", "enabled": true, "revision": "1", "ready": true, "appliedRevision": "1", "queries": queries, "state": "fresh", "observationRevision": revision, "observedAt": observed, "lastAttemptAt": observed, "lastSuccessAt": observed, "lastConfigureAttemptAt": nil}
	if r.URL.Path == "/api/v1/plugins" {
		encode(map[string]any{"plugins": []any{map[string]any{"id": p.pkg.Manifest.ID, "version": p.pkg.Manifest.Version, "manifestDigest": p.pkg.Digest, "connections": []any{connection}}}, "problems": []any{}})
		return
	}
	definitions := []any{}
	placements := []map[string]any{}
	widgets := []any{}
	for i := 0; i < 2 && len(p.pkg.Manifest.Widgets) > 0; i++ {
		d := p.pkg.Manifest.Widgets[0]
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+2)
		options := map[string]any{}
		value := map[string]any{"pluginId": p.pkg.Manifest.ID, "connectionId": previewConnection, "contributionId": d.ID, "placementId": id, "revision": "1", "enabled": true, "options": options}
		placements = append(placements, value)
		ref := map[string]any{"pluginId": p.pkg.Manifest.ID, "connectionId": previewConnection, "contributionId": d.ID, "placementId": id, "placementRevision": "1", "observationRevision": revision, "manifestDigest": p.pkg.Digest, "available": true}
		widgets = append(widgets, map[string]any{"id": "plugin-" + id, "kind": "plugin-widget", "presentation": "plugin-ui", "sourceKind": "plugin", "sourceId": id, "title": d.Title, "data": ref})
	}
	if r.URL.Path == "/api/v1/plugin-widgets" {
		for _, d := range p.pkg.Manifest.Widgets {
			raw, _ := os.ReadFile(filepath.Join(p.pkg.Root, d.OptionsSchema))
			definitions = append(definitions, map[string]any{"pluginId": p.pkg.Manifest.ID, "contributionId": d.ID, "title": d.Title, "optionsSchema": json.RawMessage(raw)})
		}
		encode(map[string]any{"definitions": definitions, "placements": placements})
		return
	}
	if strings.Contains(r.URL.Path, "/queries/") {
		if state == "loading" {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(6 * time.Second):
			}
		}
		if state == "error" {
			w.WriteHeader(503)
			encode(map[string]string{"code": "unavailable"})
			return
		}
		operation := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		data := p.fixtures[state][operation]
		if len(data) == 0 {
			message := "Independent backend fixture"
			if state == "empty" {
				message = ""
			}
			data, _ = json.Marshal(map[string]string{"message": message})
		}
		encode(map[string]any{"data": data, "state": func() string {
			if state == "stale" {
				return "stale"
			}
			return "fresh"
		}(), "observationRevision": revision, "observedAt": observed, "lastSuccessAt": observed})
		return
	}
	if r.URL.Path == "/api/v1/dashboard" {
		encode(map[string]any{"items": widgets, "unavailable": []any{}})
		return
	}
	if r.URL.Path == "/api/v1/staging/services" || r.URL.Path == "/api/v1/services" {
		encode([]any{})
		return
	}
	http.NotFound(w, r)
}
