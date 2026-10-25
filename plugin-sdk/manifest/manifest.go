// Package manifest validates installed plugin declarations before execution.
package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	Protocol  = "balemoh.plugins/v1alpha1"
	UIProfile = "balemoh.goshtoso/v1alpha1"
	MaxBytes  = 256 << 10
)

var (
	pluginID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9-]*)+$`)
	localID  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	queryID  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)*$`)
	version  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

type Manifest struct {
	ManifestVersion  int               `json:"manifestVersion"`
	ID               string            `json:"id"`
	Version          string            `json:"version"`
	Name             string            `json:"name"`
	Protocol         string            `json:"protocol"`
	UIProfile        string            `json:"uiProfile"`
	Targets          map[string]Target `json:"targets"`
	ConnectionSchema string            `json:"connectionSchema"`
	Secrets          []Secret          `json:"secrets"`
	Queries          []Query           `json:"queries"`
	Pages            []Page            `json:"pages"`
	Navigation       []Navigation      `json:"navigation"`
	Widgets          []Widget          `json:"widgets"`
	Assets           []Asset           `json:"assets"`
	Capabilities     Capabilities      `json:"capabilities"`
}
type Target struct {
	UI      string `json:"ui"`
	Backend string `json:"backend,omitempty"`
}
type Secret struct {
	ID       string `json:"id"`
	Required bool   `json:"required"`
}

func (s *Secret) UnmarshalJSON(data []byte) error {
	var value struct {
		ID       string `json:"id"`
		Required *bool  `json:"required"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&value) != nil || value.Required == nil {
		return errors.New("invalid secret declaration")
	}
	s.ID, s.Required = value.ID, *value.Required
	return nil
}

type Query struct {
	ID           string `json:"id"`
	InputSchema  string `json:"inputSchema"`
	OutputSchema string `json:"outputSchema"`
	Capability   string `json:"capability"`
}
type Page struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Path         string   `json:"path"`
	Methods      []string `json:"methods"`
	Capabilities []string `json:"capabilities"`
}
type Navigation struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Page     string       `json:"page,omitempty"`
	Children []Navigation `json:"children,omitempty"`
}
type Widget struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	OptionsSchema string   `json:"optionsSchema"`
	Capabilities  []string `json:"capabilities"`
}
type Asset struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	MediaType string `json:"mediaType"`
	Kind      string `json:"kind"`
}
type Capabilities struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

type Package struct {
	Manifest Manifest
	Digest   string
	Root     string
}

func Decode(r io.Reader) (Manifest, string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil || len(data) > MaxBytes {
		return Manifest{}, "", errors.New("manifest exceeds limit or cannot be read")
	}
	var m Manifest
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, "", fmt.Errorf("decode manifest: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return m, "", errors.New("manifest must contain one JSON object")
	}
	if err := m.Validate(); err != nil {
		return m, "", err
	}
	sum := sha256.Sum256(data)
	return m, hex.EncodeToString(sum[:]), nil
}

func (m Manifest) Validate() error {
	if m.ManifestVersion != 1 || m.Protocol != Protocol || m.UIProfile != UIProfile {
		return errors.New("incompatible manifest, protocol or UI profile")
	}
	if !pluginID.MatchString(m.ID) || len(m.ID) > 128 || !version.MatchString(m.Version) || strings.TrimSpace(m.Name) == "" {
		return errors.New("invalid plugin identity")
	}
	if len(m.Targets) == 0 || m.Secrets == nil || m.Queries == nil || m.Pages == nil || m.Navigation == nil || m.Widgets == nil || m.Assets == nil || m.Capabilities.Required == nil || m.Capabilities.Optional == nil {
		return errors.New("required manifest fields are missing")
	}
	if len(m.Pages) > 32 || len(m.Widgets) > 16 || len(m.Pages)+len(m.Widgets) == 0 {
		return errors.New("invalid contribution count")
	}
	if !safePath(m.ConnectionSchema) {
		return errors.New("invalid connection schema path")
	}
	for platform, t := range m.Targets {
		if !regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`).MatchString(platform) || !safePath(t.UI) || (t.Backend != "" && !safePath(t.Backend)) || (len(m.Queries) > 0 && t.Backend == "") {
			return errors.New("invalid executable target")
		}
	}
	caps := map[string]bool{}
	for _, c := range append(append([]string{}, m.Capabilities.Required...), m.Capabilities.Optional...) {
		if c == "" || caps[c] {
			return errors.New("duplicate or empty capability")
		}
		caps[c] = true
	}
	checkCaps := func(cs []string) bool {
		for _, c := range cs {
			if !caps[c] {
				return false
			}
		}
		return true
	}
	if len(m.Pages) > 0 && !caps["page.content/v1"] || len(m.Navigation) > 0 && !caps["sidebar.navigation/v1"] || len(m.Widgets) > 0 && !caps["home.widget/v1"] {
		return errors.New("undeclared extension point")
	}
	seen := map[string]bool{}
	patterns := []string{}
	for _, p := range m.Pages {
		if !validLocal(p.ID) || seen[p.ID] || strings.TrimSpace(p.Title) == "" || len(p.Title) > 256 || len(p.Description) > 2048 || !validRoute(p.Path) || len(p.Methods) == 0 || p.Capabilities == nil || !checkCaps(p.Capabilities) {
			return errors.New("invalid page contribution")
		}
		for _, method := range p.Methods {
			if method != "GET" && method != "POST" {
				return errors.New("unsupported page method")
			}
		}
		for _, prior := range patterns {
			if overlap(prior, p.Path) {
				return errors.New("ambiguous page routes")
			}
		}
		seen[p.ID] = true
		patterns = append(patterns, p.Path)
	}
	navIDs := map[string]bool{}
	var nav func([]Navigation, int) error
	nav = func(items []Navigation, depth int) error {
		for _, n := range items {
			if depth > 2 || !validLocal(n.ID) || navIDs[n.ID] || strings.TrimSpace(n.Title) == "" || (n.Page == "") == (len(n.Children) == 0) {
				return errors.New("invalid navigation contribution")
			}
			navIDs[n.ID] = true
			if n.Page != "" && !seen[n.Page] {
				return errors.New("navigation references unknown page")
			}
			if err := nav(n.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := nav(m.Navigation, 1); err != nil {
		return err
	}
	seen = map[string]bool{}
	for _, w := range m.Widgets {
		if !validLocal(w.ID) || seen[w.ID] || strings.TrimSpace(w.Title) == "" || !safePath(w.OptionsSchema) || w.Capabilities == nil || !checkCaps(w.Capabilities) {
			return errors.New("invalid widget contribution")
		}
		seen[w.ID] = true
	}
	seen = map[string]bool{}
	for _, q := range m.Queries {
		if !queryID.MatchString(q.ID) || len(q.ID) > 64 || seen[q.ID] || !safePath(q.InputSchema) || !safePath(q.OutputSchema) || !caps[q.Capability] {
			return errors.New("invalid query declaration")
		}
		seen[q.ID] = true
	}
	seen = map[string]bool{}
	for _, s := range m.Secrets {
		if !validLocal(s.ID) || seen[s.ID] {
			return errors.New("invalid secret declaration")
		}
		seen[s.ID] = true
	}
	seen = map[string]bool{}
	for _, a := range m.Assets {
		if !validLocal(a.ID) || seen[a.ID] || !safePath(a.File) || (a.Kind != "module" && a.Kind != "style") || a.MediaType == "" {
			return errors.New("invalid asset declaration")
		}
		seen[a.ID] = true
	}
	return nil
}

func Load(root string) (*Package, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, errors.New("invalid package root")
	}
	root = absolute
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, errors.New("open plugin package")
	}
	defer r.Close()
	f, err := r.Open("plugin.json")
	if err != nil {
		return nil, errors.New("open plugin manifest")
	}
	defer f.Close()
	m, digest, err := Decode(f)
	if err != nil {
		return nil, err
	}
	files := []string{m.ConnectionSchema}
	for _, t := range m.Targets {
		files = append(files, t.UI)
		if t.Backend != "" {
			files = append(files, t.Backend)
		}
	}
	for _, w := range m.Widgets {
		files = append(files, w.OptionsSchema)
	}
	for _, q := range m.Queries {
		files = append(files, q.InputSchema, q.OutputSchema)
	}
	for _, a := range m.Assets {
		files = append(files, a.File)
	}
	for _, name := range files {
		info, err := r.Stat(name)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("missing or invalid package file %q", name)
		}
	}
	if err := validateSchemas(r, m); err != nil {
		return nil, err
	}
	for _, w := range m.Widgets {
		raw, err := r.ReadFile(w.OptionsSchema)
		if err != nil {
			return nil, err
		}
		if _, err = FormFields(raw); err != nil {
			return nil, err
		}
	}
	for _, a := range m.Assets {
		info, err := r.Stat(a.File)
		if err != nil || info.Size() > 5<<20 {
			return nil, errors.New("asset exceeds size limit")
		}
		if a.Kind == "style" && a.MediaType != "text/css" || a.Kind == "module" && (a.MediaType != "text/javascript" && a.MediaType != "application/javascript") {
			return nil, errors.New("unsupported asset media type")
		}
	}
	return &Package{Manifest: m, Digest: digest, Root: root}, nil
}

func (p Package) Executable() (string, error) {
	return p.ExecutableFor("ui")
}
func (p Package) ExecutableFor(role string) (string, error) {
	t, ok := p.Manifest.Targets[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", errors.New("plugin has no executable for current platform")
	}
	r, err := os.OpenRoot(p.Root)
	if err != nil {
		return "", err
	}
	defer r.Close()
	name := t.UI
	if role == "backend" {
		name = t.Backend
	} else if role != "ui" {
		return "", errors.New("invalid executable role")
	}
	if name == "" {
		return "", errors.New("missing executable role")
	}
	i, err := r.Stat(name)
	if err != nil || i.Mode()&0111 == 0 {
		return "", errors.New("plugin UI file is not executable")
	}
	return p.Root + string(os.PathSeparator) + name, nil
}

func (m Manifest) Query(id string) (Query, bool) {
	for _, q := range m.Queries {
		if q.ID == id {
			return q, true
		}
	}
	return Query{}, false
}

func (m Manifest) Page(id string) (Page, bool) {
	for _, p := range m.Pages {
		if p.ID == id {
			return p, true
		}
	}
	return Page{}, false
}

// Declares reports whether a host facility participates in negotiation.
func (m Manifest) Declares(capability string) bool {
	for _, list := range [][]string{m.Capabilities.Required, m.Capabilities.Optional} {
		for _, c := range list {
			if c == capability {
				return true
			}
		}
	}
	return false
}
func (m Manifest) Widget(id string) (Widget, bool) {
	for _, w := range m.Widgets {
		if w.ID == id {
			return w, true
		}
	}
	return Widget{}, false
}
func validLocal(s string) bool { return len(s) <= 64 && localID.MatchString(s) }
func safePath(s string) bool {
	return fs.ValidPath(s) && s != "." && path.Clean(s) == s && !strings.ContainsAny(s, "\\%:?#")
}
func validRoute(s string) bool {
	if s == "/" {
		return true
	}
	if !strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") || strings.ContainsAny(s, "%?#\\") {
		return false
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(s[1:], "/") {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			p := part[1 : len(part)-1]
			if !validLocal(p) || seen[p] {
				return false
			}
			seen[p] = true
		} else if !validLocal(part) {
			return false
		}
	}
	return true
}
func overlap(a, b string) bool {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if as[i] != bs[i] && !strings.HasPrefix(as[i], "{") && !strings.HasPrefix(bs[i], "{") {
			return false
		}
	}
	return true
}
