package view

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/araihu/goshtoso/components/table"
)

const pinSelectionSubmit = `const selected = $el.querySelectorAll("input[id^='user']:checked"); selected.forEach((checkbox) => { const field = document.createElement("input"); field.type = "hidden"; field.name = "service_id"; field.value = checkbox.id.slice(4); $el.appendChild(field); });`

// PageData is the presentation model for the two operator-facing catalog
// pages. It deliberately contains no generated API types.
type PageData struct {
	Title       string
	Description string
	Active      string
	Services    []Service
	Staging     bool
	Error       string
	Notice      string
}

type Service struct {
	ID          string
	DisplayName string
	Description string
	Source      string
	Resource    string
	Namespace   string
	Pinned      bool
	Endpoints   []Endpoint
	Images      []string
}

type Endpoint struct {
	Name       string
	URL        string
	Protocol   string
	Port       int32
	Provenance string
}

func serviceTableRows(services []Service, staging bool) []table.Row {
	rows := make([]table.Row, 0, len(services))
	for _, service := range services {
		var actions templ.Component
		if !staging {
			actions = ServiceActions(service)
		}
		rows = append(rows, table.Row{
			ID: service.ID,
			Cells: map[string]table.Cell{
				"service":  {Component: ServiceIdentity(service)},
				"source":   {Text: service.Source, Code: true},
				"resource": {Text: resourceLabel(service)},
				"endpoint": {Component: ServiceEndpoints(service)},
				"images":   {Component: ServiceImages(service)},
				"status":   {Component: ServiceStatus(service)},
			},
			Actions: actions,
		})
	}
	return rows
}

func serviceActionURL(id, action string) string {
	if strings.TrimSpace(id) == "" {
		return "/staging"
	}
	return "/staging/services/" + url.PathEscape(id) + "/" + action
}

func resourceLabel(service Service) string {
	if service.Namespace == "" {
		return service.Resource
	}
	return service.Namespace + "/" + service.Resource
}

func endpointLabel(endpoint Endpoint) string {
	if endpoint.URL != "" {
		return endpoint.URL
	}
	if endpoint.Name != "" {
		return endpoint.Name
	}
	if endpoint.Port != 0 {
		return fmt.Sprintf("%s:%d", endpoint.Protocol, endpoint.Port)
	}
	if endpoint.Protocol != "" {
		return endpoint.Protocol
	}
	return "observed endpoint"
}

func serviceHostname(service Service) string {
	return endpointHostname(serviceHostnameURL(service))
}

func serviceHostnameURL(service Service) string {
	for _, endpoint := range service.Endpoints {
		href := endpointHref(endpoint.URL)
		if href != "" && endpointHostname(href) != "" {
			return href
		}
	}
	return ""
}

func endpointHostname(raw string) string {
	href := endpointHref(raw)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func endpointHref(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		if !strings.HasPrefix(raw, "//") {
			return ""
		}
		parsed, err = url.Parse("http:" + raw)
		if err != nil || parsed.Host == "" {
			return ""
		}
	}
	if parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return raw
}

func endpointSummary(endpoint Endpoint) string {
	parts := []string{endpointLabel(endpoint)}
	if endpoint.Protocol != "" && endpoint.Name != "" {
		parts = append(parts, endpoint.Protocol)
	}
	if endpoint.Port != 0 {
		parts = append(parts, fmt.Sprintf("%d", endpoint.Port))
	}
	return strings.Join(parts, " · ")
}

func imagesSummary(images []string) string {
	return strings.Join(images, ", ")
}
