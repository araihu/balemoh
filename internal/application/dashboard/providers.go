package dashboard

import (
	"context"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/application/telemetry"
)

type Catalog interface {
	ListHomepage(context.Context) ([]catalog.Candidate, error)
	Pin(context.Context, string) (catalog.Candidate, error)
	Unpin(context.Context, string) (catalog.Candidate, error)
}
type ServiceProvider struct{ Catalog Catalog }

func (ServiceProvider) Kind() string { return "service" }
func (p ServiceProvider) Selected(ctx context.Context) ([]Widget, error) {
	services, err := p.Catalog.ListHomepage(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Widget, 0, len(services))
	for _, service := range services {
		items = append(items, Widget{Kind: "service-links", SourceID: service.ID, Title: service.DisplayName, Data: service})
	}
	return items, nil
}
func (p ServiceProvider) SetSelected(ctx context.Context, id string, enabled bool) error {
	if enabled {
		_, err := p.Catalog.Pin(ctx, id)
		return err
	}
	_, err := p.Catalog.Unpin(ctx, id)
	return err
}

type Selections interface {
	List(context.Context, string) ([]string, error)
	Set(context.Context, string, string, bool) error
}
type Hosts interface {
	List(context.Context, time.Time) ([]telemetry.Host, error)
}
type HostProvider struct {
	Hosts      Hosts
	Selections Selections
}

func (HostProvider) Kind() string { return "host" }

// HostMetrics intentionally omits samples and workloads from homepage snapshots.
type HostMetrics struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	ObservedAt  *time.Time `json:"observedAt"`
	CPU         *float64   `json:"cpuPercent"`
	MemoryUsed  *int64     `json:"memoryUsedBytes"`
	MemoryTotal *int64     `json:"memoryTotalBytes"`
}

func (p HostProvider) Selected(ctx context.Context) ([]Widget, error) {
	ids, err := p.Selections.List(ctx, p.Kind())
	if err != nil {
		return nil, err
	}
	items := make([]Widget, 0, len(ids))
	if len(ids) == 0 {
		return items, nil
	}
	hosts, err := p.Hosts.List(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	byID := map[string]telemetry.Host{}
	for _, h := range hosts {
		byID[h.ID] = h
	}
	for _, id := range ids {
		w := Widget{Kind: "host-metrics", SourceID: id, Title: "Unavailable host"}
		if h, ok := byID[id]; ok {
			w.Title = h.Name
			w.Data = HostMetrics{ID: h.ID, Name: h.Name, Kind: h.Kind, Status: h.Status, ObservedAt: h.ObservedAt, CPU: h.CPUPercent, MemoryUsed: h.MemoryUsedBytes, MemoryTotal: h.MemoryTotalBytes}
		} else {
			w.Error = "Host no longer reports telemetry."
		}
		items = append(items, w)
	}
	return items, nil
}
func (p HostProvider) SetSelected(ctx context.Context, id string, enabled bool) error {
	if enabled {
		hosts, err := p.Hosts.List(ctx, time.Now())
		if err != nil {
			return err
		}
		found := false
		for _, h := range hosts {
			if h.ID == id {
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}
	}
	return p.Selections.Set(ctx, p.Kind(), id, enabled)
}
