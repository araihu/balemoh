package author

import (
	"bytes"
	"context"
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
	for _, backend := range []bool{false, true} {
		t.Run(map[bool]string{false: "ui", true: "backend"}[backend], func(t *testing.T) {
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
