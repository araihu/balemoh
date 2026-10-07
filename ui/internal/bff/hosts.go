package bff

import (
	"context"
	"errors"
	"net/http"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

type HostCatalog interface {
	Hosts(context.Context) ([]api.Host, error)
	Host(context.Context, string, string) (api.HostDetail, error)
}

func (c *APIClient) Hosts(ctx context.Context) ([]api.Host, error) {
	response, err := c.client.GetHostsWithResponse(ctx)
	if err != nil {
		return nil, &upstreamError{operation: "hosts", cause: err}
	}
	if response.JSON200 == nil {
		return nil, &upstreamError{operation: "hosts", status: response.StatusCode()}
	}
	return response.JSON200.Hosts, nil
}
func (c *APIClient) Host(ctx context.Context, id, window string) (api.HostDetail, error) {
	value := api.GetHostParamsRange(window)
	response, err := c.client.GetHostWithResponse(ctx, id, &api.GetHostParams{Range: &value})
	if err != nil {
		return api.HostDetail{}, &upstreamError{operation: "host", cause: err}
	}
	if response.JSON200 == nil {
		return api.HostDetail{}, &upstreamError{operation: "host", status: response.StatusCode()}
	}
	return *response.JSON200, nil
}
func (s *server) hosts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	data := view.PageData{Title: "Hosts", Description: "Kubernetes clusters and Docker hosts, their resource usage and reporting status.", Path: "/hosts", Active: "nav-hosts", HostsPage: &view.HostsPage{}}
	catalog, ok := s.catalog.(HostCatalog)
	if !ok {
		data.Error = "Unable to load hosts. Refresh the page to try again."
		s.render(w, r, data, 503)
		return
	}
	hosts, err := catalog.Hosts(ctx)
	if err != nil {
		data.Error = "Unable to load hosts. Refresh the page to try again."
		s.render(w, r, data, 503)
		return
	}
	for _, host := range hosts {
		data.HostsPage.Hosts = append(data.HostsPage.Hosts, hostView(host))
	}
	s.render(w, r, data, 200)
}
func (s *server) host(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("range")
	if window == "" {
		window = "1h"
	}
	if window != "1h" && window != "24h" {
		http.Error(w, "Choose a range of 1h or 24h.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	data := view.PageData{Title: "Host", Description: "Host resource usage, reporting status and workloads.", Path: "/hosts/" + r.PathValue("hostID"), Active: "nav-hosts", HostDetail: &view.HostDetail{Window: window}}
	catalog, ok := s.catalog.(HostCatalog)
	if !ok {
		data.Error = "Unable to load host."
		s.render(w, r, data, 503)
		return
	}
	detail, err := catalog.Host(ctx, r.PathValue("hostID"), window)
	if err != nil {
		var upstream *upstreamError
		if errors.As(err, &upstream) && upstream.status == 404 {
			http.NotFound(w, r)
			return
		}
		data.Error = "Unable to load host. Refresh the page to try again."
		s.render(w, r, data, 503)
		return
	}
	page := data.HostDetail
	page.Host = hostView(detail.Host)
	data.Title = page.Host.Name
	data.Description = "Resource usage and workloads for " + page.Host.Name + "."
	for _, h := range detail.Children {
		page.Children = append(page.Children, hostView(h))
	}
	for _, sample := range detail.Samples {
		page.Samples = append(page.Samples, view.HostSample{At: sample.At, CPU: sample.CpuPercent, Memory: memoryPercent(sample.MemoryUsedBytes, sample.MemoryTotalBytes)})
	}
	for _, workload := range detail.Host.Workloads {
		page.Workloads = append(page.Workloads, view.HostWorkload{Name: workload.Name, Namespace: workload.Namespace, State: workload.State, Restarts: workload.Restarts})
	}
	if detail.Host.ParentId == "" {
		services, err := s.catalog.Staging(ctx)
		page.ServicesAvailable = err == nil
		if err == nil {
			for _, service := range services {
				if service.Source == detail.Host.Source {
					page.Services = append(page.Services, view.HostService{ID: service.Id, Name: service.DisplayName})
				}
			}
		}
	}
	s.render(w, r, data, 200)
}
func hostView(h api.Host) view.Host {
	kind := "Unknown host"
	switch h.Kind {
	case "docker":
		kind = "Docker host"
	case "cluster":
		kind = "Kubernetes cluster"
	case "node":
		kind = "Kubernetes node"
	}
	return view.Host{ID: h.Id, Name: h.Name, Kind: kind, Status: string(h.Status), ParentID: h.ParentId, ObservedAt: h.ObservedAt, ReceivedAt: h.ReceivedAt, CPU: h.CpuPercent, Memory: memoryPercent(h.MemoryUsedBytes, h.MemoryTotalBytes), MemoryUsed: h.MemoryUsedBytes, MemoryTotal: h.MemoryTotalBytes, InventoryAvailable: h.InventoryAvailable, Ready: h.Ready}
}
func memoryPercent(used, total *int64) *float64 {
	if used == nil || total == nil || *total <= 0 {
		return nil
	}
	value := 100 * float64(*used) / float64(*total)
	return &value
}
