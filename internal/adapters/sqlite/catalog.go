package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/storage/sqlc"
)

type CatalogStore struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewCatalogStore(db *sql.DB) catalog.CatalogStore {
	return &CatalogStore{db: db, queries: sqlc.New(db)}
}

func (s *CatalogStore) Upsert(ctx context.Context, candidate catalog.Candidate) error {
	candidate = candidate.Normalize()
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("validate candidate: %w", err)
	}
	metadata, err := json.Marshal(candidate.Metadata)
	if err != nil {
		return fmt.Errorf("marshal candidate metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := s.queries.WithTx(tx)
	if err := queries.UpsertDiscoveredService(ctx, sqlc.UpsertDiscoveredServiceParams{
		ID:                candidate.ID,
		SourceKind:        candidate.Source.Kind,
		SourceID:          candidate.Source.ID,
		ResourceKind:      candidate.Resource.Kind,
		ResourceNamespace: candidate.Resource.Namespace,
		ResourceName:      candidate.Resource.Name,
		DisplayName:       candidate.DisplayName,
		Description:       candidate.Description,
		MetadataJson:      string(metadata),
		ObservedAt:        candidate.ObservedAt.UTC().Format(time.RFC3339Nano),
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		return fmt.Errorf("upsert candidate: %w", err)
	}
	if err := queries.DeleteServiceEndpoints(ctx, candidate.ID); err != nil {
		return fmt.Errorf("replace candidate endpoints: %w", err)
	}
	for _, endpoint := range candidate.Endpoints {
		if err := queries.InsertServiceEndpoint(ctx, sqlc.InsertServiceEndpointParams{
			ServiceID:  candidate.ID,
			Name:       endpoint.Name,
			Url:        endpoint.URL,
			Port:       int64(endpoint.Port),
			Protocol:   endpoint.Protocol,
			Provenance: endpoint.Provenance,
		}); err != nil {
			return fmt.Errorf("insert candidate endpoint: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog upsert: %w", err)
	}
	return nil
}

func (s *CatalogStore) List(ctx context.Context, pinned bool) ([]catalog.Candidate, error) {
	var (
		rows []sqlc.DiscoveredService
		err  error
	)
	if pinned {
		rows, err = s.queries.ListPinnedDiscoveredServices(ctx)
	} else {
		rows, err = s.queries.ListDiscoveredServices(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("list catalog candidates: %w", err)
	}

	result := make([]catalog.Candidate, 0, len(rows))
	for _, row := range rows {
		candidate, err := s.candidateFromRow(ctx, s.queries, row)
		if err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, nil
}

func (s *CatalogStore) SetPinned(ctx context.Context, id string, pinned bool) (catalog.Candidate, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return catalog.Candidate{}, catalog.ErrNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var (
		affected int64
		err      error
	)
	if pinned {
		affected, err = s.queries.PinDiscoveredService(ctx, sqlc.PinDiscoveredServiceParams{
			PinnedAt:  sql.NullString{String: now, Valid: true},
			UpdatedAt: now,
			ID:        id,
		})
	} else {
		affected, err = s.queries.UnpinDiscoveredService(ctx, sqlc.UnpinDiscoveredServiceParams{
			UpdatedAt: now,
			ID:        id,
		})
	}
	if err != nil {
		return catalog.Candidate{}, fmt.Errorf("set candidate pin state: %w", err)
	}
	if affected == 0 {
		return catalog.Candidate{}, catalog.ErrNotFound
	}
	row, err := s.queries.GetDiscoveredService(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return catalog.Candidate{}, catalog.ErrNotFound
		}
		return catalog.Candidate{}, fmt.Errorf("read pinned candidate: %w", err)
	}
	return s.candidateFromRow(ctx, s.queries, row)
}

func (s *CatalogStore) candidateFromRow(ctx context.Context, queries *sqlc.Queries, row sqlc.DiscoveredService) (catalog.Candidate, error) {
	metadata := make(map[string]string)
	if err := json.Unmarshal([]byte(row.MetadataJson), &metadata); err != nil {
		return catalog.Candidate{}, fmt.Errorf("decode metadata for candidate %q: %w", row.ID, err)
	}
	observedAt, err := time.Parse(time.RFC3339Nano, row.ObservedAt)
	if err != nil {
		return catalog.Candidate{}, fmt.Errorf("decode observation time for candidate %q: %w", row.ID, err)
	}

	candidate := catalog.Candidate{
		ID:          row.ID,
		Source:      catalog.SourceRef{Kind: row.SourceKind, ID: row.SourceID},
		Resource:    catalog.ResourceRef{Kind: row.ResourceKind, Namespace: row.ResourceNamespace, Name: row.ResourceName},
		DisplayName: row.DisplayName,
		Description: row.Description,
		Metadata:    metadata,
		Endpoints:   []catalog.Endpoint{},
		ObservedAt:  observedAt,
	}
	if row.PinnedAt.Valid {
		pinnedAt, err := time.Parse(time.RFC3339Nano, row.PinnedAt.String)
		if err != nil {
			return catalog.Candidate{}, fmt.Errorf("decode pin time for candidate %q: %w", row.ID, err)
		}
		candidate.PinnedAt = &pinnedAt
	}

	endpoints, err := queries.ListServiceEndpoints(ctx, row.ID)
	if err != nil {
		return catalog.Candidate{}, fmt.Errorf("list endpoints for candidate %q: %w", row.ID, err)
	}
	for _, endpoint := range endpoints {
		candidate.Endpoints = append(candidate.Endpoints, catalog.Endpoint{
			Name:       endpoint.Name,
			URL:        endpoint.Url,
			Port:       int(endpoint.Port),
			Protocol:   endpoint.Protocol,
			Provenance: endpoint.Provenance,
		})
	}
	if err := candidate.Validate(); err != nil {
		return catalog.Candidate{}, fmt.Errorf("validate stored candidate %q: %w", row.ID, err)
	}
	return candidate, nil
}

var _ catalog.CatalogStore = (*CatalogStore)(nil)
