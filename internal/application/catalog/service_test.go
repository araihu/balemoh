package catalog

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	candidates map[string]Candidate
	listPinned []bool
	upserts    []Candidate
	setPins    []struct {
		id     string
		pinned bool
	}
}

func (f *fakeStore) Upsert(_ context.Context, candidate Candidate) error {
	if f.candidates == nil {
		f.candidates = make(map[string]Candidate)
	}
	f.upserts = append(f.upserts, candidate)
	f.candidates[candidate.ID] = candidate
	return nil
}

func (f *fakeStore) List(_ context.Context, pinned bool) ([]Candidate, error) {
	f.listPinned = append(f.listPinned, pinned)
	result := make([]Candidate, 0, len(f.candidates))
	for _, candidate := range f.candidates {
		if pinned && candidate.PinnedAt == nil {
			continue
		}
		result = append(result, candidate)
	}
	return result, nil
}

func (f *fakeStore) SetPinned(_ context.Context, id string, pinned bool) (Candidate, error) {
	f.setPins = append(f.setPins, struct {
		id     string
		pinned bool
	}{id: id, pinned: pinned})
	candidate, ok := f.candidates[id]
	if !ok {
		return Candidate{}, ErrNotFound
	}
	if pinned {
		now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
		candidate.PinnedAt = &now
	} else {
		candidate.PinnedAt = nil
	}
	f.candidates[id] = candidate
	return candidate, nil
}

type fakeDiscoverer struct {
	name       string
	candidates []Candidate
	err        error
}

func (f fakeDiscoverer) Name() string { return f.name }

func (f fakeDiscoverer) Discover(context.Context) ([]Candidate, error) {
	return f.candidates, f.err
}

func testCandidate(name string) Candidate {
	return NewCandidate(
		SourceRef{Kind: "docker", ID: "host-1"},
		ResourceRef{Kind: "container", Name: name},
		time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	)
}

func TestServiceListsStagingAndHomepageThroughStore(t *testing.T) {
	store := &fakeStore{candidates: map[string]Candidate{
		"one": {ID: "one", PinnedAt: nil},
		"two": {ID: "two", PinnedAt: func() *time.Time {
			value := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
			return &value
		}()},
	}}
	service := NewService(store)

	staging, err := service.ListStaging(context.Background())
	if err != nil {
		t.Fatalf("ListStaging() error = %v", err)
	}
	if len(staging) != 2 {
		t.Fatalf("ListStaging() length = %d, want 2", len(staging))
	}

	homepage, err := service.ListHomepage(context.Background())
	if err != nil {
		t.Fatalf("ListHomepage() error = %v", err)
	}
	if len(homepage) != 1 || homepage[0].ID != "two" {
		t.Fatalf("ListHomepage() = %#v, want only pinned candidate", homepage)
	}
	if !reflect.DeepEqual(store.listPinned, []bool{false, true}) {
		t.Fatalf("store list filters = %#v, want [false true]", store.listPinned)
	}
}

func TestServicePinAndUnpinDelegateDesiredState(t *testing.T) {
	candidate := testCandidate("whoami")
	store := &fakeStore{candidates: map[string]Candidate{candidate.ID: candidate}}
	service := NewService(store)

	if _, err := service.Pin(context.Background(), candidate.ID); err != nil {
		t.Fatalf("Pin() error = %v", err)
	}
	if _, err := service.Unpin(context.Background(), candidate.ID); err != nil {
		t.Fatalf("Unpin() error = %v", err)
	}

	want := []struct {
		id     string
		pinned bool
	}{
		{id: candidate.ID, pinned: true},
		{id: candidate.ID, pinned: false},
	}
	if !reflect.DeepEqual(store.setPins, want) {
		t.Fatalf("pin operations = %#v, want %#v", store.setPins, want)
	}
}

func TestServiceSyncValidatesAndUpsertsDiscoveries(t *testing.T) {
	first := testCandidate("whoami")
	second := testCandidate("grafana")
	store := &fakeStore{}
	service := NewService(store,
		fakeDiscoverer{name: "docker-local", candidates: []Candidate{first}},
		fakeDiscoverer{name: "kubernetes-local", candidates: []Candidate{second}},
	)

	result, err := service.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Sources != 2 || result.Candidates != 2 {
		t.Fatalf("Sync() result = %#v, want sources=2 candidates=2", result)
	}
	if len(store.upserts) != 2 {
		t.Fatalf("upsert count = %d, want 2", len(store.upserts))
	}
}

func TestServiceSyncReturnsNamedDiscovererError(t *testing.T) {
	wantErr := errors.New("socket unavailable")
	service := NewService(&fakeStore{}, fakeDiscoverer{name: "docker-local", err: wantErr})

	_, err := service.Sync(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Sync() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "docker-local") {
		t.Fatalf("Sync() error = %q, want discoverer name", err)
	}
}
