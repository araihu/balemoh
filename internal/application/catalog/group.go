package catalog

import (
	"sort"
	"strings"
)

// groupCandidates projects observations around stable Kubernetes Service IDs.
// Edges never merge Services: shared routes and pods can belong to several groups.
func groupCandidates(candidates []Candidate) []Candidate {
	type key struct {
		source          SourceRef
		namespace, name string
	}
	groups := make(map[key]*Candidate)
	for _, candidate := range candidates {
		if candidate.Source.Kind != "kubernetes" || candidate.Resource.Kind != "service" {
			continue
		}
		group := candidate
		group.Endpoints = nil
		group.Images = nil
		group.Resources = nil
		groups[key{candidate.Source, candidate.Resource.Namespace, candidate.Resource.Name}] = &group
	}
	result := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		matched := false
		if candidate.Source.Kind == "kubernetes" {
			refs := strings.Split(candidate.Metadata["kubernetes.services"], ",")
			if candidate.Resource.Kind == "service" {
				refs = []string{candidate.Resource.Namespace + "/" + candidate.Resource.Name}
			}
			seen := make(map[key]bool)
			for _, ref := range refs {
				namespace, name, ok := strings.Cut(strings.TrimSpace(ref), "/")
				if !ok {
					continue
				}
				k := key{candidate.Source, namespace, name}
				if group := groups[k]; group != nil && !seen[k] {
					seen[k] = true
					matched = true
					group.Resources = append(group.Resources, ResourceObservation{Resource: candidate.Resource, Endpoints: candidate.Endpoints, Images: candidate.Images})
					group.Endpoints = append(group.Endpoints, groupMemberEndpoints(candidate, group.Resource)...)
					group.Images = append(group.Images, candidate.Images...)
				}
			}
		}
		// Preserve pre-grouping pins as explicit rows until the user unpins them.
		if !matched || (candidate.PinnedAt != nil && candidate.Resource.Kind != "service") {
			result = append(result, candidate)
		}
	}
	for _, group := range groups {
		sort.Slice(group.Resources, func(i, j int) bool {
			a, b := group.Resources[i].Resource, group.Resources[j].Resource
			return a.Kind+"/"+a.Namespace+"/"+a.Name < b.Kind+"/"+b.Namespace+"/"+b.Name
		})
		endpoints := make([]Endpoint, 0, len(group.Endpoints))
		seen := make(map[Endpoint]bool)
		for _, endpoint := range group.Endpoints {
			if !seen[endpoint] {
				endpoints = append(endpoints, endpoint)
				seen[endpoint] = true
			}
		}
		sort.Slice(endpoints, func(i, j int) bool {
			a, b := endpoints[i], endpoints[j]
			if a.URL != b.URL {
				return a.URL > b.URL
			}
			if a.Port != b.Port {
				return a.Port < b.Port
			}
			return a.Name+a.Protocol+a.Provenance < b.Name+b.Protocol+b.Provenance
		})
		group.Endpoints = endpoints
		sort.Strings(group.Images)
		images := make([]string, 0, len(group.Images))
		for _, image := range group.Images {
			if len(images) == 0 || images[len(images)-1] != image {
				images = append(images, image)
			}
		}
		group.Images = images
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DisplayName != result[j].DisplayName {
			return result[i].DisplayName < result[j].DisplayName
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// Route endpoint names identify their backend Service. Retain full route
// evidence in Resources, but expose only attributable addresses on the group.
func groupMemberEndpoints(candidate Candidate, service ResourceRef) []Endpoint {
	if candidate.Resource.Kind != "httproute" && candidate.Resource.Kind != "ingress" {
		return candidate.Endpoints
	}
	for _, ref := range strings.Split(candidate.Metadata["kubernetes.services"], ",") {
		namespace, name, ok := strings.Cut(strings.TrimSpace(ref), "/")
		if ok && name == service.Name && namespace != service.Namespace {
			return nil
		}
	}
	endpoints := make([]Endpoint, 0)
	for _, endpoint := range candidate.Endpoints {
		if endpoint.Name == service.Name {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}
