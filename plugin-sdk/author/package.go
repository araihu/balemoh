package author

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Package builds declared roles into a new immutable directory. Nothing is installed.
func Package(ctx context.Context, source, output, modfile string, targets []string) (*manifest.Package, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	if output == source || strings.HasPrefix(source, output+string(os.PathSeparator)) {
		return nil, errors.New("output cannot contain source")
	}
	if _, err = os.Stat(output); !os.IsNotExist(err) {
		return nil, errors.New("package output already exists")
	}
	raw, err := os.ReadFile(filepath.Join(source, "plugin.json"))
	if err != nil {
		return nil, err
	}
	m, _, err := manifest.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, errors.New("target required")
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	parent := filepath.Dir(output)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	temp, err := os.MkdirTemp(parent, ".plugin-package-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	backend := len(m.Queries) > 0
	m.Targets = map[string]manifest.Target{}
	for _, target := range targets {
		parts := strings.Split(target, "/")
		if len(parts) != 2 || strings.ContainsAny(target, ".\\% \t\r\n") {
			return nil, errors.New("invalid target")
		}
		value := manifest.Target{UI: "bin/" + parts[0] + "_" + parts[1] + "/ui"}
		if backend {
			value.Backend = "bin/" + parts[0] + "_" + parts[1] + "/backend"
		}
		m.Targets[target] = value
		for _, role := range []string{"ui", "backend"} {
			if role == "backend" && !backend {
				continue
			}
			name := value.UI
			if role == "backend" {
				name = value.Backend
			}
			dest := filepath.Join(temp, name)
			if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return nil, err
			}
			args := []string{"build", "-trimpath", "-o", dest}
			if modfile != "" {
				args = append(args, "-modfile", modfile)
			}
			args = append(args, "./cmd/"+role)
			command := exec.CommandContext(ctx, "go", args...)
			command.Dir = source
			command.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0", "GOOS="+parts[0], "GOARCH="+parts[1])
			command.Stderr = os.Stderr
			if err = command.Run(); err != nil {
				return nil, errors.New("plugin build failed")
			}
		}
	}
	files := []string{m.ConnectionSchema}
	for _, w := range m.Widgets {
		files = append(files, w.OptionsSchema)
	}
	for _, q := range m.Queries {
		files = append(files, q.InputSchema, q.OutputSchema)
	}
	for _, a := range m.Assets {
		files = append(files, a.File)
	}
	copied := map[string]bool{}
	var copyFile func(string) error
	copyFile = func(name string) error {
		if copied[name] {
			return nil
		}
		if !filepath.IsLocal(name) || strings.ContainsAny(name, "\\%") {
			return errors.New("invalid package path")
		}
		copied[name] = true
		file, e := root.Open(name)
		if e != nil {
			return e
		}
		info, e := file.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Size() > 5<<20 {
			file.Close()
			return errors.New("invalid declared file")
		}
		data, e := io.ReadAll(io.LimitReader(file, (5<<20)+1))
		file.Close()
		if e != nil || len(data) > 5<<20 {
			return errors.New("declared file exceeds bound")
		}
		destination := filepath.Join(temp, name)
		if e = os.MkdirAll(filepath.Dir(destination), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(destination, data, 0644); e != nil {
			return e
		}
		if strings.HasSuffix(name, ".json") {
			var v any
			if json.Unmarshal(data, &v) != nil {
				return errors.New("invalid schema JSON")
			}
			var walk func(any) error
			walk = func(v any) error {
				switch x := v.(type) {
				case map[string]any:
					for k, v := range x {
						if k == "$ref" {
							ref, ok := v.(string)
							if !ok {
								return errors.New("invalid schema reference")
							}
							path := strings.Split(ref, "#")[0]
							if path != "" {
								if strings.Contains(path, ":") {
									return errors.New("non-local schema reference")
								}
								if e := copyFile(filepath.ToSlash(filepath.Join(filepath.Dir(name), path))); e != nil {
									return e
								}
							}
						} else if e := walk(v); e != nil {
							return e
						}
					}
				case []any:
					for _, v := range x {
						if e := walk(v); e != nil {
							return e
						}
					}
				}
				return nil
			}
			if e = walk(v); e != nil {
				return e
			}
		}
		return nil
	}
	for _, name := range files {
		if err = copyFile(name); err != nil {
			return nil, err
		}
	}
	raw, _ = json.MarshalIndent(m, "", "  ")
	if err = os.WriteFile(filepath.Join(temp, "plugin.json"), append(raw, '\n'), 0644); err != nil {
		return nil, err
	}
	p, err := manifest.Load(temp)
	if err != nil {
		return nil, err
	}
	if _, err = p.ContentDigest(); err != nil {
		return nil, err
	}
	if err = os.Chmod(temp, 0755); err != nil {
		return nil, err
	}
	if err = os.Rename(temp, output); err != nil {
		return nil, err
	}
	return manifest.Load(output)
}
func Check(ctx context.Context, dir string) error {
	p, err := manifest.Load(dir)
	if err != nil {
		return err
	}
	if err = pluginruntime.Supported(p.Manifest); err != nil {
		return err
	}
	if _, err = p.ContentDigest(); err != nil {
		return err
	}
	child, err := pluginruntime.Open(ctx, p)
	if err != nil {
		return err
	}
	defer child.Close()
	if err = child.CheckHealth(ctx); err != nil {
		return err
	}
	if len(p.Manifest.Queries) > 0 {
		b, e := pluginruntime.OpenBackend(ctx, p)
		if e != nil {
			return e
		}
		defer b.Close()
		return b.CheckHealth(ctx)
	}
	return nil
}
