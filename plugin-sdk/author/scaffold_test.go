package author

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitPackageIndependentConsumers(t *testing.T) {
	sdk, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name                   string
		backend, publisherOnly bool
	}{{name: "ui"}, {name: "backend", backend: true}, {name: "publisher-only", publisherOnly: true}} {
		t.Run(fixture.name, func(t *testing.T) {
			backend := fixture.backend
			root := t.TempDir()
			source := filepath.Join(root, "source")
			if err := Init(source, "example.fixture", "example.com/fixture", backend); err != nil {
				t.Fatal(err)
			}
			manifestData, err := os.ReadFile(filepath.Join(source, "plugin.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(manifestData), "notifications.publish/v1") {
				t.Fatal("scaffold granted notification publication by default")
			}
			if backend {
				backendData, err := os.ReadFile(filepath.Join(source, "cmd/backend/main.go"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(backendData), "publishOptionalExample") {
					t.Fatal("backend scaffold omitted optional publisher example")
				}
			}
			if err := Init(source, "example.fixture", "example.com/fixture", backend); err == nil {
				t.Fatal("overwrote nonempty project")
			}
			if fixture.publisherOnly {
				var m manifest.Manifest
				if err := json.Unmarshal(manifestData, &m); err != nil {
					t.Fatal(err)
				}
				m.Capabilities.Optional = append(m.Capabilities.Optional, "notifications.publish/v1")
				target := runtime.GOOS + "/" + runtime.GOARCH
				m.Targets[target] = manifest.Target{UI: "bin/ui", Backend: "bin/backend"}
				data, err := json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "plugin.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(source, "cmd/backend"), 0755); err != nil {
					t.Fatal(err)
				}
				probeBackend := strings.Replace(backendSource, "func main(){", `func main(){if _,err:=os.Stat("fail-backend-check");err==nil{_ = os.WriteFile("backend-check-attempted",[]byte("1"),0600);os.Exit(1)};`, 1)
				if err := os.WriteFile(filepath.Join(source, "cmd/backend/main.go"), []byte(probeBackend), 0644); err != nil {
					t.Fatal(err)
				}
			}
			run := func(args ...string) {
				t.Helper()
				cmd := exec.CommandContext(t.Context(), "go", args...)
				cmd.Dir = source
				cmd.Env = append(os.Environ(), "GOWORK=off")
				if output, e := cmd.CombinedOutput(); e != nil {
					t.Fatalf("go %s: %v %s", strings.Join(args, " "), e, output)
				}
			}
			mod := filepath.Join(root, "development.mod")
			original, _ := os.ReadFile(filepath.Join(source, "go.mod"))
			if !bytes.Contains(original, []byte("github.com/araihu/balemoh/plugin-sdk "+SDKVersion)) || bytes.Contains(original, []byte("replace ")) {
				t.Fatal("scaffold must pin the released SDK without a local replacement")
			}
			if err = os.WriteFile(mod, original, 0600); err != nil {
				t.Fatal(err)
			}
			run("mod", "edit", "-modfile", mod, "-replace", "github.com/araihu/balemoh/plugin-sdk="+sdk)
			run("run", "-mod=mod", "-modfile", mod, "github.com/a-h/templ/cmd/templ", "generate")
			run("mod", "tidy", "-modfile", mod)
			run("test", "-modfile", mod, "./...")
			p, e := Package(t.Context(), source, filepath.Join(root, "package"), mod, []string{runtime.GOOS + "/" + runtime.GOARCH})
			if e != nil {
				t.Fatal(e)
			}
			if e = Check(context.Background(), p.Root); e != nil {
				t.Fatal(e)
			}
			if fixture.publisherOnly {
				target := p.Manifest.Targets[runtime.GOOS+"/"+runtime.GOARCH]
				if len(p.Manifest.Queries) != 0 || target.Backend == "" {
					t.Fatal("publisher-only backend was not packaged")
				}
				if err := os.WriteFile(filepath.Join(p.Root, "fail-backend-check"), []byte("1"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := Check(t.Context(), p.Root); err == nil {
					t.Fatal("publisher-only backend failure was ignored")
				}
				if _, err := os.Stat(filepath.Join(p.Root, "backend-check-attempted")); err != nil {
					t.Fatalf("publisher-only backend was not started by check: %v", err)
				}
			}
			after, _ := os.ReadFile(filepath.Join(source, "go.mod"))
			if !bytes.Equal(original, after) || bytes.Contains(after, []byte(sdk)) {
				t.Fatal("exported module contaminated")
			}
			if _, e = Package(t.Context(), source, p.Root, mod, []string{runtime.GOOS + "/" + runtime.GOARCH}); e == nil {
				t.Fatal("overwrote immutable package")
			}
		})
	}
	if e := Init(filepath.Join(t.TempDir(), "bad"), "../escape", "example.com/fixture", false); e == nil {
		t.Fatal("invalid plugin ID accepted")
	}
}
