package catalog

import (
	"encoding/json"
	"sort"
	"strings"
)

// relatedCandidates only joins groups with explicit, versioned discovery evidence.
// Older snapshots retain their original conservative Service projection.
func relatedCandidates(observations, groups []Candidate) []Candidate {
	parents := make([]int, len(groups))
	for i := range parents {
		parents[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		if parents[i] != i {
			parents[i] = root(parents[i])
		}
		return parents[i]
	}
	key := func(source SourceRef, ref string) string { return source.Kind + "\x00" + source.ID + "\x00" + ref }
	services := map[string]int{}
	for i, g := range groups {
		if g.Source.Kind == "kubernetes" && g.Resource.Kind == "service" {
			services[key(g.Source, g.Resource.Namespace+"/"+g.Resource.Name)] = i
		}
	}
	// Front-door ownership avoids joining two unrelated apps through a shared backend.
	owners := map[int]map[string]bool{}
	routes := map[string][]Exposure{}
	for _, c := range observations {
		var exposures []Exposure
		if json.Unmarshal([]byte(c.Metadata[ExposureMetadata]), &exposures) != nil {
			if c.Resource.Kind == "httproute" || c.Resource.Kind == "ingress" {
				for _, ref := range strings.Split(c.Metadata["kubernetes.services"], ",") {
					if i, ok := services[key(c.Source, ref)]; ok {
						if owners[i] == nil {
							owners[i] = map[string]bool{}
						}
						owners[i]["unknown:"+c.ID] = true
					}
				}
			}
			continue
		}
		routes[c.ID] = exposures
		// Any referenced backend without usable route evidence remains ambiguous.
		covered := map[string]bool{}
		for _, e := range exposures {
			if e.Scope != "" && e.Host != "" {
				covered[e.Service] = true
			}
		}
		for _, ref := range strings.Split(c.Metadata["kubernetes.services"], ",") {
			if i, ok := services[key(c.Source, ref)]; ok && !covered[ref] {
				if owners[i] == nil {
					owners[i] = map[string]bool{}
				}
				owners[i]["unknown:"+c.ID] = true
			}
		}

		for _, e := range exposures {
			if i, ok := services[key(c.Source, e.Service)]; ok {
				if owners[i] == nil {
					owners[i] = map[string]bool{}
				}
				address := e.Scope + "\x00" + e.Host
				if e.Scope == "" || e.Host == "" {
					address = "unknown:" + c.ID
				}
				owners[i][address] = true
			}
		}
	}
	exclusivelyOwned := func(i int, address string) bool {
		if len(owners[i]) == 0 {
			return false
		}
		for owner := range owners[i] {
			if !scopeAddressSubset(owner, address) {
				return false
			}
		}
		return true
	}
	primary := map[int]bool{}
	primaryURLs := map[int]map[string]bool{}
	for _, c := range observations {
		byAddress := map[string][]Exposure{}
		for _, e := range routes[c.ID] {
			if e.Service != "" {
				byAddress[e.Scope+"\x00"+e.Host] = append(byAddress[e.Scope+"\x00"+e.Host], e)
			}
		}
		for address, es := range byAddress {
			anchor := -1
			for _, e := range es {
				if e.Path == "/" && e.Match == "PathPrefix" {
					if i, ok := services[key(c.Source, e.Service)]; ok {
						if anchor != -1 && anchor != i {
							anchor = -2
							break
						}
						anchor = i
					}
				}
			}
			if anchor < 0 {
				continue
			}
			primary[anchor] = true
			if primaryURLs[anchor] == nil {
				primaryURLs[anchor] = map[string]bool{}
			}
			for _, e := range es {
				if e.Path == "/" && e.Service == groups[anchor].Resource.Namespace+"/"+groups[anchor].Resource.Name {
					primaryURLs[anchor]["//"+e.Host+"/"] = true
				}
			}
			for _, e := range es {
				if i, ok := services[key(c.Source, e.Service)]; ok && exclusivelyOwned(i, address) && exclusivelyOwned(anchor, address) {
					parents[root(i)] = root(anchor)
				}
			}
		}
	}
	// Equal selectors and target-port sets in one source/namespace identify Service aliases.
	backends := map[string]int{}
	for i, g := range groups {
		if g.Resource.Kind == "service" && g.Metadata[BackendMetadata] != "" {
			signature := key(g.Source, g.Metadata[BackendMetadata])
			if prior, ok := backends[signature]; ok {
				a, b := map[string]bool{}, map[string]bool{}
				for member, addresses := range owners {
					for address := range addresses {
						if root(member) == root(i) {
							a[address] = true
						}
						if root(member) == root(prior) {
							b[address] = true
						}
					}
				}
				compatible := len(a) == 0 || len(b) == 0
				if len(a) == len(b) {
					compatible = true
					for address := range a {
						if !b[address] {
							compatible = false
						}
					}
				}
				if compatible {
					parents[root(i)] = root(prior)
				}
			} else {
				backends[signature] = i
			}
		}
	}
	// Redirect-only rows attach only when destination resolves to one scoped backend group.
	for i, g := range groups {
		es := routes[g.ID]
		if g.Resource.Kind != "httproute" || len(es) == 0 {
			continue
		}
		destination := -1
		valid := true
		for _, e := range es {
			if e.RedirectHost == "" {
				valid = false
				break
			}
			best := -1
			bestScore := -1
			ambiguous := false
			for _, c := range observations {
				if c.Source != g.Source {
					continue
				}
				for _, target := range routes[c.ID] {
					if !scopeAddressSubset(e.Scope+"\x00"+e.RedirectHost, target.Scope+"\x00"+target.Host) || target.Service == "" {
						continue
					}
					score := -1
					if target.Match == "Exact" && target.Path == e.RedirectPath {
						score = 100000 + len(target.Path)
					}
					if target.Match == "PathPrefix" && (target.Path == "/" || e.RedirectPath == target.Path || strings.HasPrefix(e.RedirectPath, strings.TrimSuffix(target.Path, "/")+"/")) {
						score = len(target.Path)
					}
					index, ok := services[key(g.Source, target.Service)]
					if !ok || score < 0 {
						continue
					}
					index = root(index)
					if score > bestScore {
						best = index
						bestScore = score
						ambiguous = false
					} else if score == bestScore && best != index {
						ambiguous = true
					}
				}
			}
			if best < 0 || ambiguous || (destination >= 0 && destination != best) {
				valid = false
				break
			}
			destination = best
		}
		if valid && destination >= 0 {
			parents[root(i)] = root(destination)
		}
	}
	members := map[int][]int{}
	for i := range groups {
		members[root(i)] = append(members[root(i)], i)
	}
	result := make([]Candidate, 0, len(members))
	for _, indices := range members {
		sort.Slice(indices, func(a, b int) bool {
			i, j := indices[a], indices[b]
			if primary[i] != primary[j] {
				return primary[i]
			}
			if (groups[i].Resource.Kind == "service") != (groups[j].Resource.Kind == "service") {
				return groups[i].Resource.Kind == "service"
			}
			return groups[i].Resource.Namespace+"/"+groups[i].Resource.Name < groups[j].Resource.Namespace+"/"+groups[j].Resource.Name
		})
		representative := indices[0]
		g := groups[representative]
		if len(indices) == 1 {
			result = append(result, g)
			continue
		}
		g.Resources = nil
		g.Endpoints = nil
		g.Images = nil
		seenResources := map[ResourceRef]bool{}
		seenEndpoints := map[Endpoint]bool{}
		seenImages := map[string]bool{}
		for _, i := range indices {
			member := groups[i]
			if g.PinnedAt == nil && member.PinnedAt != nil {
				g.PinnedAt = member.PinnedAt
			}
			resources := member.Resources
			if len(resources) == 0 {
				resources = []ResourceObservation{{Resource: member.Resource, Endpoints: member.Endpoints, Images: member.Images}}
			}
			for _, r := range resources {
				if !seenResources[r.Resource] {
					g.Resources = append(g.Resources, r)
					seenResources[r.Resource] = true
				}
			}
			for _, e := range member.Endpoints {
				if !seenEndpoints[e] {
					g.Endpoints = append(g.Endpoints, e)
					seenEndpoints[e] = true
				}
			}
			for _, image := range member.Images {
				if !seenImages[image] {
					g.Images = append(g.Images, image)
					seenImages[image] = true
				}
			}
		}
		sort.SliceStable(g.Endpoints, func(i, j int) bool {
			return primaryURLs[representative][g.Endpoints[i].URL] && !primaryURLs[representative][g.Endpoints[j].URL]
		})
		result = append(result, g)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DisplayName != result[j].DisplayName {
			return result[i].DisplayName < result[j].DisplayName
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// A route attached to a subset of the same listeners shares that routing context.
func scopeAddressSubset(a, b string) bool {
	if a == b {
		return true
	}
	aScope, aHost, ok := strings.Cut(a, "\x00")
	if !ok {
		return false
	}
	bScope, bHost, ok := strings.Cut(b, "\x00")
	if !ok || aHost != bHost {
		return false
	}
	var as, bs []string
	if json.Unmarshal([]byte(aScope), &as) != nil || json.Unmarshal([]byte(bScope), &bs) != nil || len(as) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, scope := range bs {
		seen[scope] = true
	}
	for _, scope := range as {
		if !seen[scope] {
			return false
		}
	}
	return true
}
