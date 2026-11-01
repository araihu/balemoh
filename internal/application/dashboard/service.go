// Package dashboard composes selected content independently of its visual component.
package dashboard

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrSource   = errors.New("unsupported widget source")
	ErrNotFound = errors.New("widget source not found")
	ErrLimit    = errors.New("dashboard selection limit reached")
)

type Widget struct {
	ID           string
	Kind         string
	Presentation string
	SourceKind   string
	SourceID     string
	Title        string
	Data         any
	Error        string
}
type Snapshot struct {
	Items          []Widget
	Unavailable    []string
	LayoutRevision string
	Groups         []DashboardGroup
	LayoutError    error
}
type Provider interface {
	Kind() string
	Selected(context.Context) ([]Widget, error)
	SetSelected(context.Context, string, bool) error
}
type Service struct {
	providers []Provider
	layouts   *Layouts
}

func New(providers ...Provider) *Service {
	seen := map[string]bool{}
	for _, p := range providers {
		if p == nil || p.Kind() == "" || seen[p.Kind()] {
			panic("invalid dashboard provider registration")
		}
		seen[p.Kind()] = true
	}
	return &Service{providers: append([]Provider(nil), providers...)}
}
func (s *Service) WithLayouts(layouts *Layouts) *Service { s.layouts = layouts; return s }
func (s *Service) Snapshot(ctx context.Context) Snapshot { return s.SnapshotView(ctx, false) }
func (s *Service) SnapshotView(ctx context.Context, defaults bool) Snapshot {
	var state LayoutState
	var eligible []Widget
	if s.layouts != nil {
		var err error
		if defaults {
			state.Document = DefaultLayout()
			eligible, err = s.layouts.Store.ReadDefault(ctx)
		} else {
			state, eligible, err = s.layouts.Store.Read(ctx, true)
		}
		if err != nil {
			return Snapshot{LayoutError: err}
		}
	}
	result := Snapshot{Items: []Widget{}, Unavailable: []string{}}
	for _, p := range s.providers {
		items, err := p.Selected(ctx)
		if err != nil {
			result.Unavailable = append(result.Unavailable, p.Kind())
			continue
		}
		for _, item := range items {
			item.SourceKind = p.Kind()
			item.ID = p.Kind() + "-" + item.SourceID
			if item.Presentation == "" {
				item.Presentation = "card"
			}
			result.Items = append(result.Items, item)
		}
	}
	if s.layouts != nil {
		observed := map[string]Widget{}
		for _, w := range result.Items {
			observed[w.ID] = w
		}
		result.Items = []Widget{}
		for _, w := range eligible {
			if data, ok := observed[w.ID]; ok {
				w.Data = data.Data
				w.Error = data.Error
			} else {
				w.Error = "Source data is unavailable."
			}
			result.Items = append(result.Items, w)
		}
		resolved, _ := ResolveLayout(state.Document, eligible)
		result.Groups = ActiveLayoutGroups(resolved)
		if !defaults {
			result.LayoutRevision = state.Revision
		}
	}
	return result
}
func (s *Service) SetSelected(ctx context.Context, kind, id string, enabled bool) error {
	if id == "" || len(id) > 128 {
		return fmt.Errorf("%w: invalid identity", ErrSource)
	}
	for _, p := range s.providers {
		if p.Kind() == kind {
			return p.SetSelected(ctx, id, enabled)
		}
	}
	return ErrSource
}
