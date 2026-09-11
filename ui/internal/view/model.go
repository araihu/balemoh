package view

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/araihu/goshtoso/components/dropdown"
	"github.com/araihu/goshtoso/components/table"
	"github.com/araihu/goshtoso/components/tooltip"
)

func addressLabel(address string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(address, "https://"), "http://"), "//")
}

func discoveredAddress(service Service) string {
	service.Address = ""
	if addresses := groupAddresses(service); len(addresses) > 0 {
		if strings.HasPrefix(addresses[0], "//") {
			return "https:" + addresses[0]
		}
		return addresses[0]
	}
	return ""
}

func addressMenuItems(addresses []string) []dropdown.Item {
	items := make([]dropdown.Item, 0, len(addresses))
	for _, address := range addresses {
		items = append(items, dropdown.Item{Label: addressLabel(address), Href: address, Target: "_blank"})
	}
	return items
}

// PageData is the presentation model for the two operator-facing catalog
// pages. It deliberately contains no generated API types.
type PageData struct {
	Status      string
	IconPage    *IconPage
	Editor      *Service
	Path        string
	Title       string
	Description string
	Active      string
	Services    []Service
	Staging     bool
	Error       string
	Notice      string
}

type Service struct {
	Status       string
	Missing      bool
	IconRef      string
	Icon         LibraryIcon
	DefaultIcon  LibraryIcon
	Address      string
	EditError    string
	PinError     string
	PinAutofocus bool
	Resources    []Service
	ID           string
	DisplayName  string
	Description  string
	Source       string
	Resource     string
	Namespace    string
	Pinned       bool
	Endpoints    []Endpoint
	Images       []string
}

type Endpoint struct {
	Name       string
	URL        string
	Protocol   string
	Port       int32
	Provenance string
}

func serviceTableColumns(staging bool) []table.Column {
	columns := []table.Column{}
	if staging {
		columns = append(columns, table.Column{Key: "select", Label: "Pinned", HeaderSuffix: tooltip.Help("pinning-help", "How pinning works", "Check to pin a service to your homepage. Uncheck to remove it. Changes save immediately."), Width: "balemoh-col-select"})
	}
	columns = append(columns,
		table.Column{Key: "service", Label: "Service", Width: "balemoh-col-service"},
		table.Column{Key: "endpoint", Label: "Address", Width: "balemoh-col-address"},
	)
	return columns
}

func serviceTableRows(data PageData) []table.Row {
	rows := make([]table.Row, 0, len(data.Services))
	for _, service := range data.Services {
		row := table.Row{
			ID:          service.ID,
			AlpineAttrs: map[string]string{"id": "service-row-" + service.ID},
			Cells: map[string]table.Cell{
				"select":   {Component: ServiceSelection(service)},
				"service":  {Component: ServiceIdentity(service)},
				"endpoint": {Component: GroupAddresses(service)},
			},
		}
		if data.Staging {
			row.Link = EditURL(service.ID)
			row.LinkMode = table.LinkFull
		}
		rows = append(rows, row)
	}
	return rows
}

func isKubernetes(service Service) bool {
	return strings.HasPrefix(service.Source, "kubernetes/")
}

func selectionLabel(service Service) string {
	return "Pin " + service.DisplayName + " to homepage, " + resourceLabel(service)
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
	if endpoint.Name != "" {
		return endpoint.Name
	}
	if endpoint.URL != "" {
		return endpoint.URL
	}
	if endpoint.Port != 0 {
		return fmt.Sprintf("%s:%d", endpoint.Protocol, endpoint.Port)
	}
	if endpoint.Protocol != "" {
		return endpoint.Protocol
	}
	return "observed endpoint"
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

func resourceCount(service Service) string {
	n := len(service.Resources)
	if n == 0 {
		n = 1
	}
	if n == 1 {
		return "1 resource"
	}
	return fmt.Sprintf("%d resources", n)
}
func resourceMembers(service Service) []Service {
	if len(service.Resources) > 0 {
		return service.Resources
	}
	return []Service{service}
}
func groupAddresses(service Service) []string {
	if service.Address != "" {
		return []string{service.Address}
	}
	addresses := []string{}
	seen := map[string]bool{}
	for _, endpoint := range service.Endpoints {
		if href := endpointHref(endpoint.URL); href != "" && !seen[href] {
			addresses = append(addresses, href)
			seen[href] = true
		}
	}
	return addresses
}

func successToastEvent(message string) string {
	payload, _ := json.Marshal(map[string]string{"kind": "toast", "tone": "success", "message": message})
	return "$nextTick(() => $dispatch('notify', " + string(payload) + "))"
}

func (s Service) State() string {
	if s.Status == "" {
		return "live"
	}
	return s.Status
}
func lifecycleURL(id string) string { return serviceActionURL(id, "lifecycle") }
func stagingURL(state string) string {
	if state == "" {
		state = "live"
	}
	return "/staging?status=" + url.QueryEscape(state) + "#" + url.QueryEscape(state)
}
