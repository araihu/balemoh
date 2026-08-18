package catalog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type CatalogStore interface {
	Upsert(context.Context, Candidate) error
	List(context.Context, bool) ([]Candidate, error)
	DeleteUnpinned(context.Context, string) error
	SetPinned(context.Context, string, bool) (Candidate, error)
}

type Discoverer interface {
	Name() string
	Discover(context.Context) ([]Candidate, error)
}

// SourceProvider lets a discoverer identify an empty source snapshot.
// Discoverer stays small so existing adapters and test doubles remain valid.
type SourceProvider interface {
	Source() SourceRef
}

type SnapshotPublisher interface {
	Publish(context.Context, Snapshot) error
}

type SnapshotImporter interface {
	ImportSnapshot(context.Context, Snapshot) (SyncResult, error)
}

type UseCase interface {
	ListStaging(context.Context) ([]Candidate, error)
	ListHomepage(context.Context) ([]Candidate, error)
	Pin(context.Context, string) (Candidate, error)
	Unpin(context.Context, string) (Candidate, error)
	Sync(context.Context) (SyncResult, error)
}

type SyncResult struct {
	Sources    int
	Candidates int
}

type Service struct {
	store       CatalogStore
	publisher   SnapshotPublisher
	discoverers []Discoverer
}

func NewService(store CatalogStore, discoverers ...Discoverer) *Service {
	return NewServiceWithPublisher(store, nil, discoverers...)
}

func NewServiceWithPublisher(store CatalogStore, publisher SnapshotPublisher, discoverers ...Discoverer) *Service {
	return &Service{store: store, publisher: publisher, discoverers: discoverers}
}

func (s *Service) ListStaging(ctx context.Context) ([]Candidate, error) {
	return s.store.List(ctx, false)
}

func (s *Service) ListHomepage(ctx context.Context) ([]Candidate, error) {
	return s.store.List(ctx, true)
}

func (s *Service) Pin(ctx context.Context, id string) (Candidate, error) {
	return s.store.SetPinned(ctx, id, true)
}

func (s *Service) Unpin(ctx context.Context, id string) (Candidate, error) {
	return s.store.SetPinned(ctx, id, false)
}

func (s *Service) Sync(ctx context.Context) (SyncResult, error) {
	var result SyncResult
	for _, discoverer := range s.discoverers {
		result.Sources++
		candidates, err := discoverer.Discover(ctx)
		if err != nil {
			return result, fmt.Errorf("discover %s: %w", discoverer.Name(), err)
		}
		batches, err := snapshotBatches(discoverer, candidates)
		if err != nil {
			return result, fmt.Errorf("prepare snapshot from %s: %w", discoverer.Name(), err)
		}
		for _, candidate := range candidates {
			candidate = candidate.Normalize()
			if err := candidate.Validate(); err != nil {
				return result, fmt.Errorf("validate candidate from %s: %w", discoverer.Name(), err)
			}
			if err := s.store.Upsert(ctx, candidate); err != nil {
				return result, fmt.Errorf("store candidate from %s: %w", discoverer.Name(), err)
			}
			result.Candidates++
		}
		if s.publisher != nil {
			for _, snapshot := range batches {
				if err := s.publisher.Publish(ctx, snapshot); err != nil {
					return result, fmt.Errorf("publish snapshot from %s: %w", discoverer.Name(), err)
				}
			}
		}
	}
	return result, nil
}

func (s *Service) ImportSnapshot(ctx context.Context, snapshot Snapshot) (SyncResult, error) {
	snapshot = snapshot.Normalize()
	if err := snapshot.Validate(); err != nil {
		return SyncResult{}, fmt.Errorf("%w: %v", ErrInvalidSnapshot, err)
	}
	keep := make(map[string]struct{}, len(snapshot.Candidates))
	for _, candidate := range snapshot.Candidates {
		// Gateway pin state is local. SQLite upsert preserves an existing local
		// pin, while a new imported candidate starts unpinned.
		candidate.PinnedAt = nil
		if err := s.store.Upsert(ctx, candidate); err != nil {
			return SyncResult{}, fmt.Errorf("store imported candidate: %w", err)
		}
		keep[candidate.ID] = struct{}{}
	}
	known, err := s.store.List(ctx, false)
	if err != nil {
		return SyncResult{}, fmt.Errorf("list candidates for imported source reconciliation: %w", err)
	}
	for _, candidate := range known {
		if candidate.Source != snapshot.Source || candidate.PinnedAt != nil {
			continue
		}
		if _, ok := keep[candidate.ID]; ok {
			continue
		}
		if err := s.store.DeleteUnpinned(ctx, candidate.ID); err != nil {
			return SyncResult{}, fmt.Errorf("remove stale imported candidate: %w", err)
		}
	}
	return SyncResult{Sources: 1, Candidates: len(snapshot.Candidates)}, nil
}

var ErrInvalidSnapshot = errors.New("invalid federation snapshot")

func snapshotBatches(discoverer Discoverer, candidates []Candidate) ([]Snapshot, error) {
	providedSource, hasProvidedSource := discoverer.(SourceProvider)
	var source SourceRef
	if hasProvidedSource {
		source = providedSource.Source()
		source.Kind = strings.TrimSpace(source.Kind)
		source.ID = strings.TrimSpace(source.ID)
		if source.Kind == "" || source.ID == "" {
			return nil, fmt.Errorf("discoverer source identity must not be empty")
		}
	}

	groups := make(map[string]*Snapshot)
	for _, candidate := range candidates {
		candidate = candidate.Normalize()
		if hasProvidedSource && candidate.Source != source {
			return nil, fmt.Errorf("candidate source %q/%q does not match discoverer source %q/%q", candidate.Source.Kind, candidate.Source.ID, source.Kind, source.ID)
		}
		key := candidate.Source.Kind + "\x00" + candidate.Source.ID
		group := groups[key]
		if group == nil {
			group = &Snapshot{Source: candidate.Source, Candidates: []Candidate{}, ObservedAt: time.Now().UTC()}
			groups[key] = group
		}
		group.Candidates = append(group.Candidates, candidate)
	}
	if len(candidates) == 0 && hasProvidedSource {
		return []Snapshot{{Source: source, Candidates: []Candidate{}, ObservedAt: time.Now().UTC()}}, nil
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	batches := make([]Snapshot, 0, len(keys))
	for _, key := range keys {
		batches = append(batches, *groups[key])
	}
	return batches, nil
}

var _ UseCase = (*Service)(nil)
var _ SnapshotImporter = (*Service)(nil)
