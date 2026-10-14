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
	Items       []Widget
	Unavailable []string
}
type Provider interface {
	Kind() string
	Selected(context.Context) ([]Widget, error)
	SetSelected(context.Context, string, bool) error
}
type Service struct{ providers []Provider }

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
func (s *Service) Snapshot(ctx context.Context) Snapshot {
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
