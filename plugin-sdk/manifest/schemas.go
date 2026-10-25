package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateJSON uses only package-local schema references and bounded input.
func (p Package) ValidateJSON(name string, data []byte) error {
	if len(data) > 1<<20 || !safePath(name) {
		return errors.New("invalid schema input")
	}
	root, err := os.OpenRoot(p.Root)
	if err != nil {
		return errors.New("invalid package")
	}
	defer root.Close()
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(localLoader{root})
	s, err := c.Compile("https://balemoh.invalid/" + name)
	if err != nil {
		return errors.New("invalid package schema")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var value any
	if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("invalid JSON input")
	}
	if s.Validate(value) != nil {
		return errors.New("schema validation failed")
	}
	return nil
}

type localLoader struct{ root *os.Root }

func (l localLoader) Load(location string) (any, error) {
	u, err := url.Parse(location)
	if err != nil || u.Scheme != "https" || u.Host != "balemoh.invalid" || u.RawQuery != "" {
		return nil, errors.New("schema references must be package-local")
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !safePath(name) {
		return nil, errors.New("invalid schema path")
	}
	f, err := l.root.Open(name)
	if err != nil {
		return nil, errors.New("cannot open schema")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil || len(data) > MaxBytes {
		return nil, errors.New("schema exceeds limit")
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return nil, errors.New("invalid schema JSON")
	}
	return value, nil
}
func validateSchemas(root *os.Root, m Manifest) error {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(localLoader{root})
	paths := []string{m.ConnectionSchema}
	for _, w := range m.Widgets {
		paths = append(paths, w.OptionsSchema)
	}
	for _, q := range m.Queries {
		paths = append(paths, q.InputSchema, q.OutputSchema)
	}
	for _, p := range paths {
		if _, err := c.Compile("https://balemoh.invalid/" + p); err != nil {
			return errors.New("invalid or non-local package schema")
		}
	}
	return nil
}
