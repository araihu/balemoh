package plugins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/google/uuid"
	openapitypes "github.com/oapi-codegen/runtime/types"
)

// These data records are independent of rendering and contain no credentials.
type Definition struct {
	PluginID       string          `json:"pluginId"`
	ContributionID string          `json:"contributionId"`
	Title          string          `json:"title"`
	OptionsSchema  json.RawMessage `json:"optionsSchema"`
}
type PlacementRecord struct {
	PluginID       string          `json:"pluginId"`
	ConnectionID   string          `json:"connectionId"`
	ContributionID string          `json:"contributionId"`
	PlacementID    string          `json:"placementId"`
	Revision       string          `json:"revision"`
	Options        json.RawMessage `json:"options"`
	Enabled        bool            `json:"enabled"`
}
type WidgetCatalog struct {
	Definitions []Definition      `json:"definitions"`
	Placements  []PlacementRecord `json:"placements"`
}

func (s *Source) Widgets(ctx context.Context) (WidgetCatalog, error) {
	response, err := s.client.GetPluginWidgets(ctx)
	if err != nil {
		return WidgetCatalog{}, backend.Fail("unavailable")
	}
	var result WidgetCatalog
	err = readResponse(response, &result)
	return result, err
}
func (r *Registry) WidgetCatalog(ctx context.Context) (WidgetCatalog, error) {
	if r == nil || r.source == nil {
		return WidgetCatalog{}, ErrUnavailable
	}
	return r.source.Widgets(ctx)
}
func (r *Registry) SavePlacement(ctx context.Context, p PlacementRecord, create bool) (PlacementRecord, error) {
	if r == nil || r.source == nil || !uuidPattern.MatchString(p.PlacementID) {
		return PlacementRecord{}, ErrNotFound
	}
	// The generated client owns route construction and the private credential.
	raw, _ := json.Marshal(map[string]any{"pluginId": p.PluginID, "connectionId": p.ConnectionID, "contributionId": p.ContributionID, "options": p.Options, "enabled": p.Enabled})
	var match *string
	if !create {
		v := `"` + p.Revision + `"`
		match = &v
	}
	id, err := parseID(p.PlacementID)
	if err != nil {
		return PlacementRecord{}, err
	}
	response, err := r.source.client.PutPluginWidgetWithBody(ctx, id, &api.PutPluginWidgetParams{IfMatch: match}, "application/json", strings.NewReader(string(raw)))
	if err != nil {
		return PlacementRecord{}, ErrUnavailable
	}
	if response.StatusCode == 201 {
		response.StatusCode = 200
	}
	var result PlacementRecord
	err = readResponse(response, &result)
	if err != nil && response.StatusCode == 412 {
		current, e := r.source.Widgets(ctx)
		if e == nil {
			for _, v := range current.Placements {
				if v.PlacementID == p.PlacementID && sameDesired(v, p) {
					return v, nil
				}
			}
		}
	}
	return result, err
}
func sameDesired(a, b PlacementRecord) bool {
	return a.PluginID == b.PluginID && a.ConnectionID == b.ConnectionID && a.ContributionID == b.ContributionID && a.Enabled == b.Enabled && equalOptions(a.Options, b.Options)
}

func (r *Registry) DeletePlacement(ctx context.Context, id, revision string) error {
	if r == nil || r.source == nil {
		return ErrUnavailable
	}
	key, err := parseID(id)
	if err != nil {
		return err
	}
	match := `"` + revision + `"`
	response, err := r.source.client.DeletePluginWidget(ctx, key, &api.DeletePluginWidgetParams{IfMatch: &match})
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 204 {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode == 412 {
		return backend.Fail("conflict")
	}
	return backend.Fail("unavailable")
}
func (r *Registry) CurrentPlacement(ctx context.Context, id string) (PlacementRecord, error) {
	c, err := r.WidgetCatalog(ctx)
	if err != nil {
		return PlacementRecord{}, err
	}
	for _, p := range c.Placements {
		if p.PlacementID == id {
			return p, nil
		}
	}
	return PlacementRecord{}, ErrNotFound
}

type cachedRender struct {
	Key      string
	Response ui.Response
	At       time.Time
}

func (r *Registry) PersistentWidget(ctx context.Context, id string) (ui.Response, error) {
	p, err := r.CurrentPlacement(ctx, id)
	if err != nil {
		return ui.Response{}, err
	}
	if !p.Enabled {
		return ui.Response{}, ErrNotFound
	}
	e := r.find(p.PluginID, p.ConnectionID)
	if e == nil {
		return ui.Response{}, ErrUnavailable
	}
	w, ok := e.Package.Manifest.Widget(p.ContributionID)
	if !ok || e.Package.ValidateJSON(w.OptionsSchema, p.Options) != nil {
		return ui.Response{}, ErrUnavailable
	}
	grants, metadata, err := r.source.granted(ctx, e)
	if err != nil {
		return ui.Response{}, err
	}
	req := ui.ApplyDisplay(ctx, request(e, "widget", p.ContributionID, p.PlacementID))
	// Diagnostic attempt/success times can change without a new observation.
	// Cache authorization and data identity, not the descriptor's diagnostic fields.
	granted := make([]string, 0, len(grants))
	for id := range grants {
		granted = append(granted, id)
	}
	sort.Strings(granted)
	raw, _ := json.Marshal(granted)
	grantDigest := sha256.Sum256(raw)
	observation := ""
	if metadata.ObservationRevision != nil {
		observation = *metadata.ObservationRevision
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s/%x/%s/%s/%s/%s", e.Installation.PackageDigest, e.Package.Digest, p.ConnectionID, metadata.Revision, p.PlacementID, p.Revision, observation, grantDigest, req.Locale, req.Timezone, req.Theme, req.ColorScheme)
	child := e.Client()
	if child == nil || child.Exited() {
		return r.retained(id, key, ErrUnavailable)
	}
	r.mu.RLock()
	cached, ok := r.cache[id]
	r.mu.RUnlock()
	if ok && cached.Key == key && !e.Package.Manifest.Declares(ui.ServiceCatalogCapability) {
		return cached.Response, nil
	}
	response, err := child.HandleWithServices(ctx, req, scopedServices{source: r.source, entry: e, grants: grants, requestID: req.RequestID, placement: ui.Placement{ID: p.PlacementID, ContributionID: p.ContributionID, Options: p.Options, Revision: p.Revision}})
	if err != nil {
		return r.retained(id, key, err)
	}
	if response.Status == 200 {
		r.mu.Lock()
		if r.cache == nil {
			r.cache = map[string]cachedRender{}
		}
		if _, ok := r.cache[id]; !ok && len(r.cache) >= 128 {
			var oldest string
			var at time.Time
			for id, v := range r.cache {
				if oldest == "" || v.At.Before(at) {
					oldest, at = id, v.At
				}
			}
			delete(r.cache, oldest)
		}
		r.cache[id] = cachedRender{key, response, time.Now()}
		r.mu.Unlock()
	}
	return response, nil
}
func (r *Registry) retained(id, key string, err error) (ui.Response, error) {
	r.mu.RLock()
	cached, ok := r.cache[id]
	r.mu.RUnlock()
	if !ok || cached.Key != key {
		return ui.Response{}, err
	}
	response := cached.Response
	response.Status = 503
	response.HTML = append([]byte(`<p role="alert">Plugin rendering unavailable. Showing last rendered content.</p>`), cached.Response.HTML...)
	if len(response.HTML) > ui.MaxHTML {
		return ui.Response{}, err
	}
	return response, nil
}

func (r *Registry) Persistent() bool                            { return r != nil && r.source != nil }
func (r *Registry) Connection(plugin, connection string) *Entry { return r.find(plugin, connection) }

func parseID(id string) (openapitypes.UUID, error) { return uuid.Parse(id) }

func (r *Registry) Fragment(ctx context.Context, plugin, connection, contribution, region string, q url.Values) (ui.Response, error) {
	e := r.find(plugin, connection)
	if e == nil || e.Client() == nil {
		return ui.Response{}, ErrUnavailable
	}
	if _, ok := e.Package.Manifest.Page(contribution); !ok || !regexp.MustCompile(`^[a-z0-9-]{1,64}$`).MatchString(region) {
		return ui.Response{}, ErrNotFound
	}
	req := request(e, "page", contribution, "")
	req.Mode = "fragment"
	req.Region = region
	req.Query = q
	return r.handle(ctx, e, req, "")
}

// The BFF uses the same exact JSON value contract without importing host application internals.
func equalOptions(a, b []byte) bool {
	decode := func(raw []byte) (any, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		if d.Decode(new(any)) != io.EOF {
			return nil, backend.Fail("invalid_argument")
		}
		return normalizedValue(v), nil
	}
	x, e := decode(a)
	y, f := decode(b)
	return e == nil && f == nil && reflect.DeepEqual(x, y)
}

type numberValue struct{ Value string }

func normalizedValue(v any) any {
	switch x := v.(type) {
	case json.Number:
		r, ok := new(big.Rat).SetString(string(x))
		if ok {
			return numberValue{r.RatString()}
		}
		return x
	case map[string]any:
		for k, v := range x {
			x[k] = normalizedValue(v)
		}
		return x
	case []any:
		for i, v := range x {
			x[i] = normalizedValue(v)
		}
		return x
	default:
		return x
	}
}
