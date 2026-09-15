package bff

import (
	"context"
	"net/http"
	"sort"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

func (s *server) hosts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	data := view.PageData{
		Title:       "Hosts",
		Description: "Kubernetes clusters and Docker hosts discovered across your homelab.",
		Path:        "/hosts",
		Active:      "nav-hosts",
		HostsPage:   &view.HostsPage{},
	}
	// ponytail: only observed sources; expose source inventory if empty sources must appear.
	services, err := s.catalog.Staging(ctx)
	if err != nil {
		data.Error = "Unable to load hosts. Refresh the page to try again."
		s.render(w, r, data, http.StatusServiceUnavailable)
		return
	}
	hostIndex := make(map[api.SourceRef]int)
	for _, service := range services {
		source := service.Source
		if source.Id == "" {
			continue
		}
		if index, exists := hostIndex[source]; exists {
			host := &data.HostsPage.Hosts[index]
			host.Services++
			if service.ObservedAt.After(host.ObservedAt) {
				host.ObservedAt = service.ObservedAt
			}
			continue
		}
		kind := ""
		switch source.Kind {
		case "kubernetes":
			kind = "Kubernetes cluster"
		case "container":
			kind = "Docker host"
		default:
			kind = "Unknown host"
		}
		hostIndex[source] = len(data.HostsPage.Hosts)
		data.HostsPage.Hosts = append(data.HostsPage.Hosts, view.Host{
			Name: source.Id, Kind: kind, Services: 1, ObservedAt: service.ObservedAt,
		})
	}
	sort.Slice(data.HostsPage.Hosts, func(i, j int) bool {
		a, b := data.HostsPage.Hosts[i], data.HostsPage.Hosts[j]
		if a.Kind != b.Kind {
			if a.Kind == "Kubernetes cluster" || b.Kind == "Kubernetes cluster" {
				return a.Kind == "Kubernetes cluster"
			}
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	s.render(w, r, data, http.StatusOK)
}
