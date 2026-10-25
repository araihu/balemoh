package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"
	"time"
	"uuid"

	"github.com/araihu/balemoh/internal/application/dashboard"
)

type PlacementBody struct {
	PluginID       string          `json:"pluginId"`
	ConnectionID   string          `json:"connectionId"`
	ContributionID string          `json:"contributionId"`
	Options        json.RawMessage `json:"options"`
	Enabled        bool            `json:"enabled"`
}
type Placement struct {
	PlacementBody
	PlacementID string    `json:"placementId"`
	Revision    string    `json:"revision"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
type Definition struct {
	PluginID       string          `json:"pluginId"`
	ContributionID string          `json:"contributionId"`
	Title          string          `json:"title"`
	OptionsSchema  json.RawMessage `json:"optionsSchema"`
}
type PlacementStore interface {
	Placements(context.Context) ([]Placement, error)
	Placement(context.Context, string) (Placement, error)
	PutPlacement(context.Context, string, PlacementBody, string) (Placement, bool, error)
	DeletePlacement(context.Context, string, string) error
}
type PlacementValidator interface {
	Definitions() []Definition
	ValidateOptions(string, string, json.RawMessage) error
}

func CanonicalOptions(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return nil, Fail("invalid_argument")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value map[string]any
	if d.Decode(&value) != nil || value == nil || d.Decode(new(any)) != io.EOF {
		return nil, Fail("invalid_argument")
	}
	result, err := json.Marshal(value)
	return result, err
}
func SamePlacement(a, b PlacementBody) bool {
	x, e := CanonicalOptions(a.Options)
	y, f := CanonicalOptions(b.Options)
	return e == nil && f == nil && a.PluginID == b.PluginID && a.ConnectionID == b.ConnectionID && a.ContributionID == b.ContributionID && a.Enabled == b.Enabled && equalOptions(x, y)
}
func (s *Service) Definitions() []Definition {
	if v, ok := s.runtime.(PlacementValidator); ok {
		return v.Definitions()
	}
	return []Definition{}
}
func (s *Service) Placements(ctx context.Context) ([]Placement, error) {
	if v, ok := s.store.(PlacementStore); ok {
		return v.Placements(ctx)
	}
	return nil, Fail("unavailable")
}
func (s *Service) PutPlacement(ctx context.Context, id string, b PlacementBody, match string) (Placement, bool, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Placement{}, false, Fail("invalid_argument")
	}
	store, ok := s.store.(PlacementStore)
	if !ok {
		return Placement{}, false, Fail("unavailable")
	}
	old, err := store.Placement(ctx, id)
	if err != nil && ErrorCode(err) != "not_found" {
		return Placement{}, false, err
	}
	// Removal/disable remains available when a package or connection disappears.
	disabling := err == nil && !b.Enabled && old.PluginID == b.PluginID && old.ConnectionID == b.ConnectionID && old.ContributionID == b.ContributionID && equalOptions(old.Options, b.Options)
	if !disabling {
		c, e := s.store.Get(ctx, b.PluginID, b.ConnectionID)
		if e != nil {
			return Placement{}, false, e
		}
		if !c.Enabled {
			return Placement{}, false, Fail("conflict")
		}
		v, ok := s.runtime.(PlacementValidator)
		if !ok {
			return Placement{}, false, Fail("unavailable")
		}
		if e = v.ValidateOptions(b.PluginID, b.ContributionID, b.Options); e != nil {
			return Placement{}, false, e
		}
	}
	b.Options, err = CanonicalOptions(b.Options)
	if err != nil {
		return Placement{}, false, err
	}
	return store.PutPlacement(ctx, id, b, match)
}
func (s *Service) DeletePlacement(ctx context.Context, id, match string) error {
	if v, ok := s.store.(PlacementStore); ok {
		return v.DeletePlacement(ctx, id, match)
	}
	return Fail("unavailable")
}

// Provider reads only local placement records and cached observation metadata.
type Provider struct{ Service *Service }

func (Provider) Kind() string                                      { return "plugin" }
func (p Provider) SetSelected(context.Context, string, bool) error { return dashboard.ErrSource }
func (p Provider) Selected(ctx context.Context) ([]dashboard.Widget, error) {
	rows, err := p.Service.Placements(ctx)
	if err != nil {
		return nil, err
	}
	out := []dashboard.Widget{}
	definitions := p.Service.Definitions()
	for _, v := range rows {
		if !v.Enabled {
			continue
		}
		title := "Unavailable plugin widget"
		for _, d := range definitions {
			if d.PluginID == v.PluginID && d.ContributionID == v.ContributionID {
				title = d.Title
			}
		}
		status, _ := p.Service.runtime.CachedStatus(v.PluginID, v.ConnectionID)
		c, e := p.Service.store.Get(ctx, v.PluginID, v.ConnectionID)
		available := e == nil && c.Enabled && status.Ready
		observation := ""
		if status.ObservationRevision != nil {
			observation = *status.ObservationRevision
		}
		data := map[string]any{"pluginId": v.PluginID, "connectionId": v.ConnectionID, "contributionId": v.ContributionID, "placementId": v.PlacementID, "placementRevision": v.Revision, "observationRevision": observation, "manifestDigest": c.ManifestDigest, "available": available}
		problem := ""
		if !available {
			problem = "Plugin unavailable"
		}
		out = append(out, dashboard.Widget{SourceID: v.PlacementID, Kind: "plugin-widget", Presentation: "plugin-ui", Title: title, Data: data, Error: problem})
	}
	return out, nil
}

func equalOptions(a, b []byte) bool {
	decode := func(raw []byte) (any, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		if d.Decode(new(any)) != io.EOF {
			return nil, Fail("invalid_argument")
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
