package bff

import (
	"sort"
	"strings"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

// Catalog resource references are discovery evidence. Display names and pod
// prefixes are never used to decide which application owns a workload.
func hostApplications(detail api.HostDetail, services []api.ServiceCandidate) []view.HostApplication {
	type resourceKey struct{ kind, namespace, name string }
	key := func(r api.ResourceRef) resourceKey {
		return resourceKey{r.Kind, pointerValue(r.Namespace), r.Name}
	}
	composeKey := func(s api.ServiceCandidate) string {
		project, service := s.Metadata["compose.project"], s.Metadata["compose.service"]
		if project == "" || service == "" {
			return ""
		}
		return project + "/" + service
	}
	composes := map[string]bool{}
	for _, s := range services {
		if s.Source == detail.Host.Source && s.Resource.Kind == "compose-service" {
			composes[composeKey(s)] = true
		}
	}
	matched := map[resourceKey]bool{}
	apps := []view.HostApplication{}
	for _, s := range services {
		if s.Source != detail.Host.Source || s.Resource.Kind == "pod" {
			continue
		}
		if s.Resource.Kind == "container" && composeKey(s) != "" && composes[composeKey(s)] {
			continue
		}
		refs := map[resourceKey]bool{key(s.Resource): true}
		if s.Resources != nil {
			for _, member := range *s.Resources {
				refs[key(member.Resource)] = true
			}
		}
		if s.Resource.Kind == "compose-service" {
			for _, member := range services {
				if member.Source == s.Source && member.Resource.Kind == "container" && composeKey(member) == composeKey(s) {
					refs[key(member.Resource)] = true
				}
			}
		}
		app := view.HostApplication{Service: mapService(s), Missing: s.Missing || s.Status == "missing", ObservedAt: s.ObservedAt}
		// Docker discovery lists running containers; telemetry also lists stopped
		// containers. Only that contract allows missing discovery evidence to
		// associate current inventory. Historical Kubernetes refs prove no ownership.
		stoppedContainer := detail.Host.Kind == "docker" && s.Source.Kind == "container" && (s.Resource.Kind == "container" || s.Resource.Kind == "compose-service")
		if !app.Missing || stoppedContainer {
			for _, workload := range detail.Host.Workloads {
				kind := "pod"
				if detail.Host.Kind == "docker" {
					kind = "container"
				}
				k := resourceKey{kind, workload.Namespace, workload.Name}
				if refs[k] {
					app.Workloads = append(app.Workloads, hostWorkloadView(workload, detail.Host))
					matched[k] = true
				}
			}
		}
		if stoppedContainer && len(app.Workloads) > 0 {
			app.Missing = false
		}
		// A node's catalog is scoped by actual workload membership, not merely
		// by its cluster source. Historical placement is not available.
		if detail.Host.ParentId != "" && len(app.Workloads) == 0 {
			continue
		}
		sortWorkloads(app.Workloads)
		apps = append(apps, app)
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Service.DisplayName)+apps[i].Service.ID < strings.ToLower(apps[j].Service.DisplayName)+apps[j].Service.ID
	})
	unassociated := view.HostApplication{Service: view.Service{ID: "unassociated", DisplayName: "Unassociated workloads"}, Unassociated: true}
	for _, workload := range detail.Host.Workloads {
		kind := "pod"
		if detail.Host.Kind == "docker" {
			kind = "container"
		}
		if !matched[resourceKey{kind, workload.Namespace, workload.Name}] {
			unassociated.Workloads = append(unassociated.Workloads, hostWorkloadView(workload, detail.Host))
		}
	}
	if len(unassociated.Workloads) > 0 {
		sortWorkloads(unassociated.Workloads)
		apps = append(apps, unassociated)
	}
	return apps
}

func hostWorkloadView(w api.HostWorkload, host api.Host) view.HostWorkload {
	node := pointerValue(w.Node)
	if node == "" && host.ParentId != "" {
		node = host.Name
	} else if host.Kind == "docker" {
		node = host.Name
	}
	var images []string
	if w.Images != nil {
		images = *w.Images
	}
	return view.HostWorkload{Name: w.Name, Namespace: w.Namespace, State: w.State, Node: node, Ready: w.Ready, Reason: pointerValue(w.Reason), Images: images, Restarts: w.Restarts, Docker: host.Kind == "docker"}
}

func sortWorkloads(workloads []view.HostWorkload) {
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].Namespace+"/"+workloads[i].Name < workloads[j].Namespace+"/"+workloads[j].Name
	})
}
