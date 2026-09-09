package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araihu/balemoh/client/iconassets"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
	"github.com/araihu/balemoh/internal/storage/sqlc"
)

type CatalogStore struct {
	db      *sql.DB
	queries *sqlc.Queries
}

type discoveredServiceRow struct {
	ID                string
	SourceKind        string
	SourceID          string
	ResourceKind      string
	ResourceNamespace string
	ResourceName      string
	DisplayName       string
	Description       string
	MetadataJson      string
	ImagesJson        string
	ObservedAt        string
	PinnedAt          sql.NullString
}

func discoveredServiceRowFromGet(row sqlc.GetDiscoveredServiceRow) discoveredServiceRow {
	return discoveredServiceRow{
		ID:                row.ID,
		SourceKind:        row.SourceKind,
		SourceID:          row.SourceID,
		ResourceKind:      row.ResourceKind,
		ResourceNamespace: row.ResourceNamespace,
		ResourceName:      row.ResourceName,
		DisplayName:       row.DisplayName,
		Description:       row.Description,
		MetadataJson:      row.MetadataJson,
		ImagesJson:        row.ImagesJson,
		ObservedAt:        row.ObservedAt,
		PinnedAt:          row.PinnedAt,
	}
}

func discoveredServiceRowFromList(row sqlc.ListDiscoveredServicesRow) discoveredServiceRow {
	return discoveredServiceRow{
		ID:                row.ID,
		SourceKind:        row.SourceKind,
		SourceID:          row.SourceID,
		ResourceKind:      row.ResourceKind,
		ResourceNamespace: row.ResourceNamespace,
		ResourceName:      row.ResourceName,
		DisplayName:       row.DisplayName,
		Description:       row.Description,
		MetadataJson:      row.MetadataJson,
		ImagesJson:        row.ImagesJson,
		ObservedAt:        row.ObservedAt,
		PinnedAt:          row.PinnedAt,
	}
}

func discoveredServiceRowFromPinnedList(row sqlc.ListPinnedDiscoveredServicesRow) discoveredServiceRow {
	return discoveredServiceRow{
		ID:                row.ID,
		SourceKind:        row.SourceKind,
		SourceID:          row.SourceID,
		ResourceKind:      row.ResourceKind,
		ResourceNamespace: row.ResourceNamespace,
		ResourceName:      row.ResourceName,
		DisplayName:       row.DisplayName,
		Description:       row.Description,
		MetadataJson:      row.MetadataJson,
		ImagesJson:        row.ImagesJson,
		ObservedAt:        row.ObservedAt,
		PinnedAt:          row.PinnedAt,
	}
}

func NewCatalogStore(db *sql.DB) catalog.CatalogStore {
	return &CatalogStore{db: db, queries: sqlc.New(db)}
}

func (s *CatalogStore) Upsert(ctx context.Context, candidate catalog.Candidate) error {
	candidate = candidate.Normalize()
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("validate candidate: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.upsertCandidate(ctx, s.queries.WithTx(tx), candidate); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog upsert: %w", err)
	}
	return nil
}

func (s *CatalogStore) upsertCandidate(ctx context.Context, queries *sqlc.Queries, candidate catalog.Candidate) error {
	existing, err := queries.GetDiscoveredService(ctx, candidate.ID)
	switch {
	case err == nil:
		existingObservedAt, parseErr := time.Parse(time.RFC3339Nano, existing.ObservedAt)
		if parseErr != nil {
			return fmt.Errorf("decode existing observation time for candidate %q: %w", candidate.ID, parseErr)
		}
		if !candidate.ObservedAt.After(existingObservedAt) {
			return nil
		}
	case errors.Is(err, sql.ErrNoRows):
		// Candidate is new.
	default:
		return fmt.Errorf("read existing candidate: %w", err)
	}

	metadata, err := json.Marshal(candidate.Metadata)
	if err != nil {
		return fmt.Errorf("marshal candidate metadata: %w", err)
	}
	images, err := json.Marshal(candidate.Images)
	if err != nil {
		return fmt.Errorf("marshal candidate images: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
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
		ImagesJson:        string(images),
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
	return nil
}

func (s *CatalogStore) ReplaceSourceSnapshot(ctx context.Context, snapshot catalog.Snapshot) (catalog.SnapshotApplyResult, error) {
	snapshot = snapshot.Normalize()
	if err := snapshot.Validate(); err != nil {
		return catalog.SnapshotApplyResult{}, fmt.Errorf("validate source snapshot: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return catalog.SnapshotApplyResult{}, fmt.Errorf("begin source snapshot replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)

	existing, err := queries.GetDiscoverySourceSnapshot(ctx, sqlc.GetDiscoverySourceSnapshotParams{
		SourceKind: snapshot.Source.Kind,
		SourceID:   snapshot.Source.ID,
	})
	if err == nil {
		existingObservedAt, parseErr := time.Parse(time.RFC3339Nano, existing.ObservedAt)
		if parseErr != nil {
			return catalog.SnapshotApplyResult{}, fmt.Errorf("decode source watermark: %w", parseErr)
		}
		if !snapshot.ObservedAt.After(existingObservedAt) {
			if err := tx.Commit(); err != nil {
				return catalog.SnapshotApplyResult{}, fmt.Errorf("commit unchanged source snapshot: %w", err)
			}
			return catalog.SnapshotApplyResult{Candidates: len(snapshot.Candidates)}, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return catalog.SnapshotApplyResult{}, fmt.Errorf("read source watermark: %w", err)
	}

	keepIDs := make([]string, 0, len(snapshot.Candidates))
	for _, candidate := range snapshot.Candidates {
		candidate.PinnedAt = nil
		if err := s.upsertCandidate(ctx, queries, candidate); err != nil {
			return catalog.SnapshotApplyResult{}, fmt.Errorf("upsert source candidate: %w", err)
		}
		keepIDs = append(keepIDs, candidate.ID)
	}
	if err := deleteAbsentUnpinnedCandidates(ctx, tx, snapshot.Source, keepIDs); err != nil {
		return catalog.SnapshotApplyResult{}, err
	}
	if err := queries.UpsertDiscoverySourceSnapshot(ctx, sqlc.UpsertDiscoverySourceSnapshotParams{
		SourceKind: snapshot.Source.Kind,
		SourceID:   snapshot.Source.ID,
		ObservedAt: snapshot.ObservedAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return catalog.SnapshotApplyResult{}, fmt.Errorf("store source watermark: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return catalog.SnapshotApplyResult{}, fmt.Errorf("commit source snapshot replacement: %w", err)
	}
	return catalog.SnapshotApplyResult{Applied: true, Candidates: len(snapshot.Candidates)}, nil
}

func deleteAbsentUnpinnedCandidates(ctx context.Context, tx *sql.Tx, source catalog.SourceRef, keepIDs []string) error {
	query := `DELETE FROM discovered_services WHERE source_kind = ? AND source_id = ? AND pinned_at IS NULL`
	args := []any{source.Kind, source.ID}
	if len(keepIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keepIDs)), ",")
		query += " AND id NOT IN (" + placeholders + ")"
		for _, id := range keepIDs {
			args = append(args, id)
		}
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("remove stale source candidates: %w", err)
	}
	return nil
}

func (s *CatalogStore) List(ctx context.Context, pinned bool) ([]catalog.Candidate, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin catalog read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)

	var rows []discoveredServiceRow
	if pinned {
		pinnedRows, listErr := queries.ListPinnedDiscoveredServices(ctx)
		err = listErr
		for _, row := range pinnedRows {
			rows = append(rows, discoveredServiceRowFromPinnedList(row))
		}
	} else {
		allRows, listErr := queries.ListDiscoveredServices(ctx)
		err = listErr
		for _, row := range allRows {
			rows = append(rows, discoveredServiceRowFromList(row))
		}
	}
	if err != nil {
		return nil, fmt.Errorf("list catalog candidates: %w", err)
	}

	result := make([]catalog.Candidate, 0, len(rows))
	for _, row := range rows {
		candidate, err := s.candidateFromRow(ctx, queries, row)
		if err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit catalog read: %w", err)
	}
	return result, nil
}

func (s *CatalogStore) SetPinned(ctx context.Context, id string, pinned bool, related ...string) (catalog.Candidate, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return catalog.Candidate{}, catalog.ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return catalog.Candidate{}, fmt.Errorf("begin catalog pin update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)
	for _, currentID := range append([]string{id}, related...) {
		row, err := queries.GetDiscoveredService(ctx, currentID)
		if errors.Is(err, sql.ErrNoRows) {
			return catalog.Candidate{}, catalog.ErrNotFound
		}
		if err != nil {
			return catalog.Candidate{}, fmt.Errorf("read candidate pin state: %w", err)
		}

		if pinned && !row.PinnedAt.Valid {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if _, err := queries.PinDiscoveredService(ctx, sqlc.PinDiscoveredServiceParams{
				PinnedAt:  sql.NullString{String: now, Valid: true},
				UpdatedAt: now,
				ID:        currentID,
			}); err != nil {
				return catalog.Candidate{}, fmt.Errorf("set candidate pin state: %w", err)
			}
		}
		if !pinned && row.PinnedAt.Valid {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if _, err := queries.UnpinDiscoveredService(ctx, sqlc.UnpinDiscoveredServiceParams{
				UpdatedAt: now,
				ID:        currentID,
			}); err != nil {
				return catalog.Candidate{}, fmt.Errorf("clear candidate pin state: %w", err)
			}
		}

	}

	row, err := queries.GetDiscoveredService(ctx, id)
	if err != nil {
		return catalog.Candidate{}, fmt.Errorf("read updated candidate pin state: %w", err)
	}
	candidate, err := s.candidateFromRow(ctx, queries, discoveredServiceRowFromGet(row))
	if err != nil {
		return catalog.Candidate{}, err
	}
	if err := tx.Commit(); err != nil {
		return catalog.Candidate{}, fmt.Errorf("commit catalog pin update: %w", err)
	}
	return candidate, nil
}

func (s *CatalogStore) candidateFromRow(ctx context.Context, queries *sqlc.Queries, row discoveredServiceRow) (catalog.Candidate, error) {
	metadata := make(map[string]string)
	if err := json.Unmarshal([]byte(row.MetadataJson), &metadata); err != nil {
		return catalog.Candidate{}, fmt.Errorf("decode metadata for candidate %q: %w", row.ID, err)
	}
	images := make([]string, 0)
	if err := json.Unmarshal([]byte(row.ImagesJson), &images); err != nil {
		return catalog.Candidate{}, fmt.Errorf("decode images for candidate %q: %w", row.ID, err)
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
		Images:      images,
		ObservedAt:  observedAt,
	}
	edit, err := queries.GetServiceEdit(ctx, row.ID)
	if err == nil {
		candidate.DisplayName = edit.DisplayName
		candidate.Description = edit.Description
		candidate.Address = edit.Address
		candidate.Icon = edit.IconRef
	} else if !errors.Is(err, sql.ErrNoRows) {
		return catalog.Candidate{}, fmt.Errorf("read service edit: %w", err)
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
	candidate = candidate.Normalize()
	if err := candidate.Validate(); err != nil {
		return catalog.Candidate{}, fmt.Errorf("validate stored candidate %q: %w", row.ID, err)
	}
	return candidate, nil
}

var _ catalog.CatalogStore = (*CatalogStore)(nil)

func (s *CatalogStore) SaveEdit(ctx context.Context, id string, edit catalog.Edit) error {
	edit = edit.Normalize()
	if err := edit.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.queries.WithTx(tx)
	if _, err := q.GetDiscoveredService(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return catalog.ErrNotFound
	} else if err != nil {
		return err
	}
	if err := q.SaveServiceEdit(ctx, sqlc.SaveServiceEditParams{ServiceID: id, DisplayName: edit.DisplayName, Description: edit.Description, Address: edit.Address}); err != nil {
		return err
	}
	if edit.Icon != nil {
		ref := *edit.Icon
		if ref != "" && !iconassets.Has(ref) {
			var exists int
			if err := tx.QueryRowContext(ctx, "SELECT 1 FROM uploaded_icons WHERE id = ?", ref).Scan(&exists); err != nil {
				return catalog.ErrInvalidEdit
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE service_edits SET icon_ref = ? WHERE service_id = ?", ref, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
