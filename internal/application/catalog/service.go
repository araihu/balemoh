package catalog

import (
	"context"
	"fmt"
)

type CatalogStore interface {
	Upsert(context.Context, Candidate) error
	List(context.Context, bool) ([]Candidate, error)
	SetPinned(context.Context, string, bool) (Candidate, error)
}

type Discoverer interface {
	Name() string
	Discover(context.Context) ([]Candidate, error)
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
	discoverers []Discoverer
}

func NewService(store CatalogStore, discoverers ...Discoverer) *Service {
	return &Service{store: store, discoverers: discoverers}
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
	}
	return result, nil
}

var _ UseCase = (*Service)(nil)
