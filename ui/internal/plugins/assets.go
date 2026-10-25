package plugins

import (
	"context"
	"errors"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"io"
	"os"
	"strings"
)

type Asset struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

func (r *Registry) ResponseAssets(plugin, connection string, ids []string) ([]Asset, error) {
	out := []Asset{}
	if len(ids) == 0 {
		return out, nil
	}
	e := r.find(plugin, connection)
	if e == nil || e.Problem != nil {
		return nil, ErrNotFound
	}
	digest := e.Installation.PackageDigest
	if digest == "" {
		digest = e.Package.Digest
	}
	for _, id := range ids {
		found := false
		for _, a := range e.Package.Manifest.Assets {
			if a.ID == id {
				out = append(out, Asset{id, a.Kind, "/plugins/assets/" + plugin + "/" + digest + "/" + a.File})
				found = true
			}
		}
		if !found {
			return nil, errors.New("undeclared response asset")
		}
	}
	return out, nil
}
func (r *Registry) ReadAsset(plugin, digest, path string) ([]byte, string, error) {
	if strings.ContainsAny(path, "%\\") {
		return nil, "", ErrNotFound
	}
	for _, e := range r.Entries() {
		expected := e.Installation.PackageDigest
		if expected == "" && e.Package != nil {
			expected = e.Package.Digest
		}
		if e.Package == nil || e.Package.Manifest.ID != plugin || expected != digest || e.Problem != nil {
			continue
		}
		for _, a := range e.Package.Manifest.Assets {
			if a.File != path {
				continue
			}
			root, err := os.OpenRoot(e.Package.Root)
			if err != nil {
				return nil, "", err
			}
			file, err := root.Open(path)
			root.Close()
			if err != nil {
				return nil, "", err
			}
			data, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
			file.Close()
			if err != nil || len(data) > 5<<20 {
				return nil, "", ErrUnavailable
			}
			return data, a.MediaType, nil
		}
	}
	return nil, "", ErrNotFound
}
func (r *Registry) PageRequest(ctx context.Context, plugin, connection, path, method string, query, form map[string][]string, body []byte) (ui.Response, error) {
	page, req, _, err := r.ResolvePage(plugin, connection, path, query)
	if err != nil {
		return ui.Response{}, err
	}
	allowed := false
	for _, m := range page.Methods {
		if m == method {
			allowed = true
		}
	}
	if !allowed {
		return ui.Response{}, ErrNotFound
	}
	e := r.find(plugin, connection)
	if e.Client() == nil {
		return ui.Response{}, ErrUnavailable
	}
	req.Method = method
	req.Form = form
	req.Body = body
	return r.handle(ctx, e, req, "")
}
