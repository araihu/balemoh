package catalog

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func relationCandidate(kind, name string) Candidate {
	return NewCandidate(SourceRef{Kind: "kubernetes", ID: "test"}, ResourceRef{Kind: kind, Namespace: "apps", Name: name}, time.Now())
}
func relationRoute(name, scope, host string, backends ...string) Candidate {
	c := relationCandidate("httproute", name)
	var evidence []Exposure
	for i, backend := range backends {
		path := "/"
		if i > 0 {
			path = "/" + backend
		}
		evidence = append(evidence, Exposure{Scope: scope, Host: host, Path: path, Match: "PathPrefix", Service: "apps/" + backend, Port: 80})
		if i > 0 {
			c.Metadata["kubernetes.services"] += ","
		}
		c.Metadata["kubernetes.services"] += "apps/" + backend
		c.Endpoints = append(c.Endpoints, Endpoint{Name: backend, URL: "//" + host + path})
	}
	raw, _ := json.Marshal(evidence)
	c.Metadata[ExposureMetadata] = string(raw)
	return c
}
func TestRelatedRouteGroupsAndIsolation(t *testing.T) {
	web, api, shared := relationCandidate("service", "web"), relationCandidate("service", "api"), relationCandidate("service", "shared")
	route := relationRoute("app", "public/https", "app.test", "web", "api", "shared")
	route.Endpoints = append(route.Endpoints, Endpoint{Name: "web", URL: "//app.test/z"})
	groups := groupCandidates([]Candidate{web, api, shared, route})
	if len(groups) != 1 || groups[0].ID != web.ID || len(groups[0].Resources) != 4 {
		t.Fatalf("groups=%#v", groups)
	}
	if groups[0].Endpoints[0].URL != "//app.test/" {
		t.Fatal("root address is not primary")
	}
	other := relationCandidate("service", "other")
	otherRoute := relationRoute("other", "public/https", "other.test", "other", "shared")
	groups = groupCandidates([]Candidate{web, api, shared, route, other, otherRoute})
	if len(groups) != 3 {
		t.Fatalf("shared backend joined independent apps: %d groups", len(groups))
	}
}
func TestRelatedAliasesPreservePinAndDoNotJoinVirtualHosts(t *testing.T) {
	a, b := relationCandidate("service", "app"), relationCandidate("service", "app-lb")
	a.Metadata[BackendMetadata] = "same selector and target port"
	b.Metadata[BackendMetadata] = a.Metadata[BackendMetadata]
	now := time.Now()
	b.PinnedAt = &now
	groups := groupCandidates([]Candidate{a, b})
	if len(groups) != 1 || groups[0].ID != a.ID || groups[0].PinnedAt == nil {
		t.Fatalf("alias pin lost: %#v", groups)
	}
	groups = groupCandidates([]Candidate{a, b, relationRoute("a", "public", "a.test", "app"), relationRoute("b", "public", "b.test", "app-lb")})
	if len(groups) != 2 {
		t.Fatal("distinct virtual hosts merged")
	}
	b.Metadata[BackendMetadata] = "different target port"
	if len(groupCandidates([]Candidate{a, b})) != 2 {
		t.Fatal("different target ports merged")
	}
}
func TestRedirectUsesScopeAndLongestMatchingPath(t *testing.T) {
	web, admin := relationCandidate("service", "web"), relationCandidate("service", "admin")
	route := relationRoute("app", "public", "app.test", "web")
	private := relationRoute("private", "internal", "app.test", "admin")
	redirect := relationCandidate("httproute", "redirect")
	raw, _ := json.Marshal([]Exposure{{Scope: "public", Host: "old.test", Path: "/", Match: "PathPrefix", RedirectHost: "app.test", RedirectPath: "/"}})
	redirect.Metadata[ExposureMetadata] = string(raw)
	groups := groupCandidates([]Candidate{web, admin, route, private, redirect})
	if len(groups) != 2 {
		t.Fatalf("redirect not attached: %d", len(groups))
	}
	for _, g := range groups {
		if g.ID == admin.ID && len(g.Resources) != 2 {
			t.Fatal("redirect attached to wrong listener")
		}
	}
	raw, _ = json.Marshal([]Exposure{{Scope: "unknown", Host: "old.test", Path: "/", Match: "PathPrefix", RedirectHost: "app.test", RedirectPath: "/"}})
	redirect.Metadata[ExposureMetadata] = string(raw)
	if len(groupCandidates([]Candidate{web, admin, route, private, redirect})) != 3 {
		t.Fatal("unknown listener merged")
	}
}

func TestUnpinClearsPinnedServiceAlias(t *testing.T) {
	a, b := relationCandidate("service", "app"), relationCandidate("service", "app-lb")
	a.Metadata[BackendMetadata] = "same backend"
	b.Metadata[BackendMetadata] = "same backend"
	now := time.Now()
	b.PinnedAt = &now
	store := &fakeStore{candidates: map[string]Candidate{a.ID: a, b.ID: b}}
	service := NewService(store)
	if _, err := service.Unpin(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	home, err := service.ListHomepage(context.Background())
	if err != nil || len(home) != 0 {
		t.Fatalf("alias stayed pinned: %#v %v", home, err)
	}
}

func TestOverlappingListenerSetsDoNotBlockOneRouteGroup(t *testing.T) {
	web, api := relationCandidate("service", "web"), relationCandidate("service", "api")
	both, _ := json.Marshal([]string{"internal/https", "public/https"})
	public, _ := json.Marshal([]string{"public/https"})
	route := relationRoute("app", string(both), "app.test", "web", "api")
	extra := relationRoute("deny", string(public), "app.test", "web")
	if groups := groupCandidates([]Candidate{web, api, route, extra}); len(groups) != 1 {
		t.Fatalf("one route split across %d groups", len(groups))
	}
}

func TestUnknownRouteEvidencePreventsSharedBackendMerge(t *testing.T) {
	web, api := relationCandidate("service", "web"), relationCandidate("service", "api")
	route := relationRoute("app", "public", "app.test", "web", "api")
	unknown := relationCandidate("httproute", "conditional")
	unknown.Metadata["kubernetes.services"] = "apps/api"
	if len(groupCandidates([]Candidate{web, api, route, unknown})) != 2 {
		t.Fatal("backend with unknown routing was merged")
	}
}
