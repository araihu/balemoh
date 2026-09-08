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
	candidates   map[string]Candidate
	listPinned   []bool
	upserts      []Candidate
	replacements []Snapshot
	replaceErr   error
	setPins      []struct {
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

func (f *fakeStore) ReplaceSourceSnapshot(_ context.Context, snapshot Snapshot) (SnapshotApplyResult, error) {
	if f.replaceErr != nil {
		return SnapshotApplyResult{}, f.replaceErr
	}
	if f.candidates == nil {
		f.candidates = make(map[string]Candidate)
	}
	f.replacements = append(f.replacements, snapshot)
	keep := make(map[string]struct{}, len(snapshot.Candidates))
	for _, candidate := range snapshot.Candidates {
		if existing, ok := f.candidates[candidate.ID]; ok && existing.PinnedAt != nil {
			candidate.PinnedAt = existing.PinnedAt
		}
		f.candidates[candidate.ID] = candidate
		keep[candidate.ID] = struct{}{}
	}
	for id, candidate := range f.candidates {
		if candidate.Source != snapshot.Source || candidate.PinnedAt != nil {
			continue
		}
		if _, ok := keep[id]; !ok {
			delete(f.candidates, id)
		}
	}
	return SnapshotApplyResult{Applied: true, Candidates: len(snapshot.Candidates)}, nil
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

type fakeSourceDiscoverer struct {
	fakeDiscoverer
	source SourceRef
}

func (f fakeSourceDiscoverer) Source() SourceRef { return f.source }

type fakePublisher struct {
	snapshots []Snapshot
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, snapshot Snapshot) error {
	f.snapshots = append(f.snapshots, snapshot)
	return f.err
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
	if !reflect.DeepEqual(store.listPinned, []bool{false, false}) {
		t.Fatalf("store list filters = %#v, want [false false]", store.listPinned)
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

func TestServiceSyncPublishesSourceSnapshotIncludingEmptySource(t *testing.T) {
	first := testCandidate("whoami")
	publisher := &fakePublisher{}
	service := NewServiceWithPublisher(&fakeStore{}, publisher,
		fakeSourceDiscoverer{
			fakeDiscoverer: fakeDiscoverer{name: "docker-local", candidates: []Candidate{first}},
			source:         first.Source,
		},
		fakeSourceDiscoverer{
			fakeDiscoverer: fakeDiscoverer{name: "kubernetes-local"},
			source:         SourceRef{Kind: "kubernetes", ID: "cluster-1"},
		},
	)

	result, err := service.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Sources != 2 || result.Candidates != 1 {
		t.Fatalf("Sync() result = %#v, want sources=2 candidates=1", result)
	}
	if len(publisher.snapshots) != 2 {
		t.Fatalf("published snapshots = %d, want 2", len(publisher.snapshots))
	}
	if publisher.snapshots[0].Source != first.Source || len(publisher.snapshots[0].Candidates) != 1 {
		t.Fatalf("first snapshot = %#v, want source and one candidate", publisher.snapshots[0])
	}
	if publisher.snapshots[1].Source != (SourceRef{Kind: "kubernetes", ID: "cluster-1"}) || len(publisher.snapshots[1].Candidates) != 0 {
		t.Fatalf("empty snapshot = %#v, want registered empty source", publisher.snapshots[1])
	}
}

func TestServiceImportSnapshotDropsRemotePinState(t *testing.T) {
	candidate := testCandidate("remote")
	pinnedAt := time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC)
	candidate.PinnedAt = &pinnedAt
	store := &fakeStore{}
	service := NewService(store)

	result, err := service.ImportSnapshot(context.Background(), Snapshot{
		Source:     candidate.Source,
		Candidates: []Candidate{candidate},
		ObservedAt: candidate.ObservedAt,
	})
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if result.Sources != 1 || result.Candidates != 1 {
		t.Fatalf("ImportSnapshot() result = %#v, want sources=1 candidates=1", result)
	}
	if len(store.replacements) != 1 || store.replacements[0].Candidates[0].PinnedAt != nil {
		t.Fatalf("imported replacement = %#v, want local unpinned state", store.replacements)
	}
}

func TestServiceImportSnapshotReconcilesSourceWithoutDeletingPins(t *testing.T) {
	source := SourceRef{Kind: "container", ID: "remote-host"}
	stale := NewCandidate(source, ResourceRef{Kind: "container", Name: "gone"}, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	pinned := NewCandidate(source, ResourceRef{Kind: "container", Name: "pinned"}, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	pinnedAt := time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC)
	pinned.PinnedAt = &pinnedAt
	unrelated := testCandidate("unrelated")
	current := NewCandidate(source, ResourceRef{Kind: "container", Name: "current"}, time.Date(2026, 8, 17, 14, 0, 0, 0, time.UTC))
	store := &fakeStore{candidates: map[string]Candidate{
		stale.ID:     stale,
		pinned.ID:    pinned,
		unrelated.ID: unrelated,
	}}
	service := NewService(store)

	_, err := service.ImportSnapshot(context.Background(), Snapshot{
		Source:     source,
		Candidates: []Candidate{current},
		ObservedAt: current.ObservedAt,
	})
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if _, ok := store.candidates[stale.ID]; ok {
		t.Fatal("stale unpinned candidate remains")
	}
	if _, ok := store.candidates[pinned.ID]; !ok {
		t.Fatal("pinned candidate was deleted")
	}
	if _, ok := store.candidates[unrelated.ID]; !ok {
		t.Fatal("candidate from unrelated source was deleted")
	}
	if len(store.replacements) != 1 || len(store.replacements[0].Candidates) != 1 || store.replacements[0].Candidates[0].ID != current.ID {
		t.Fatalf("replacements = %#v, want only current candidate", store.replacements)
	}
}

func TestServiceImportSnapshotRejectsMixedSourcesBeforeWriting(t *testing.T) {
	first := testCandidate("first")
	second := testCandidate("second")
	second.Source = SourceRef{Kind: "kubernetes", ID: "other"}
	second.ID = StableID(second.Source, second.Resource)
	store := &fakeStore{}
	service := NewService(store)

	_, err := service.ImportSnapshot(context.Background(), Snapshot{
		Source:     first.Source,
		Candidates: []Candidate{first, second},
		ObservedAt: first.ObservedAt,
	})
	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("ImportSnapshot() error = %v, want invalid snapshot", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("upserts = %d, want no writes for invalid snapshot", len(store.upserts))
	}
}
