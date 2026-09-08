package catalog

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestServiceGroupsResourceEvidenceWithoutMergingSharedBackends(t *testing.T) {
	source := SourceRef{Kind: "kubernetes", ID: "cluster-a"}
	observation := func(kind, namespace, name, refs string) Candidate {
		c := NewCandidate(source, ResourceRef{Kind: kind, Namespace: namespace, Name: name}, time.Now())
		c.Metadata["kubernetes.services"] = refs
		return c
	}
	app := observation("service", "apps", "appflowy", "")
	other := observation("service", "apps", "other", "")
	route := observation("httproute", "apps", "shared", "apps/appflowy,apps/other,apps/appflowy")
	route.Endpoints = []Endpoint{{Name: "appflowy", URL: "//appflowy.example/", Port: 8080, Provenance: "kubernetes.httproute"}}
	pod := observation("pod", "apps", "appflowy-abc", "apps/appflowy")
	pod.Images = []string{"app:v1", "app:v1"}
	orphan := observation("httproute", "apps", "orphan", "apps/missing")
	foreign := observation("pod", "apps", "foreign", "apps/appflowy")
	foreign.Source.ID = "cluster-b"
	foreign.ID = StableID(foreign.Source, foreign.Resource)
	namespace := observation("service", "elsewhere", "appflowy", "")
	store := &fakeStore{candidates: map[string]Candidate{}}
	for _, c := range []Candidate{app, other, route, pod, orphan, foreign, namespace} {
		store.candidates[c.ID] = c
	}
	service := NewService(store)
	groups, err := service.ListStaging(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 5 {
		t.Fatalf("groups=%d, want 5", len(groups))
	}
	find := func(id string) Candidate {
		for _, c := range groups {
			if c.ID == id {
				return c
			}
		}
		t.Fatalf("missing %s", id)
		return Candidate{}
	}
	group := find(app.ID)
	if len(group.Resources) != 3 || !reflect.DeepEqual(group.Images, []string{"app:v1"}) || len(group.Endpoints) != 1 {
		t.Fatalf("group=%#v", group)
	}
	if len(find(other.ID).Endpoints) != 0 {
		t.Fatal("shared route leaked another backend address")
	}
	if len(find(other.ID).Resources) != 2 || len(find(namespace.ID).Resources) != 1 {
		t.Fatal("shared resource merged distinct Services")
	}
	if len(store.candidates[app.ID].Images) != 0 {
		t.Fatal("projection mutated stored observation")
	}
	if _, err := service.Pin(context.Background(), app.ID); err != nil {
		t.Fatal(err)
	}
	// Real store persists pin state independently of resource observations.
	pinned := store.candidates[app.ID]
	now := time.Now()
	pinned.PinnedAt = &now
	store.candidates[app.ID] = pinned
	delete(store.candidates, pod.ID)
	replacement := observation("pod", "apps", "appflowy-def", "apps/appflowy")
	replacement.Images = []string{"app:v2"}
	store.candidates[replacement.ID] = replacement
	homepage, err := service.ListHomepage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(homepage) != 1 || homepage[0].ID != app.ID || !reflect.DeepEqual(homepage[0].Images, []string{"app:v2"}) || len(homepage[0].Resources) != 3 {
		t.Fatalf("homepage after rollout=%#v", homepage)
	}
	// Previously pinned raw resources remain visible, never silently lose a pin.
	route.PinnedAt = &now
	store.candidates[route.ID] = route
	homepage, err = service.ListHomepage(context.Background())
	if err != nil || len(homepage) != 2 {
		t.Fatalf("legacy pin lost: %#v %v", homepage, err)
	}
}
