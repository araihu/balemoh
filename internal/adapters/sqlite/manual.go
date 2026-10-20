package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
)

// CreateManual stores the record and its presentation atomically. A replay
// returns the current record without overwriting later edits or visibility.
func (s *CatalogStore) CreateManual(ctx context.Context, input catalog.ManualService) (catalog.Candidate, error) {
	input.Edit = input.Edit.Normalize()
	if err := input.Validate(); err != nil {
		return catalog.Candidate{}, err
	}
	c := catalog.NewCandidate(catalog.SourceRef{Kind: catalog.ManualSourceKind, ID: "local"}, catalog.ResourceRef{Kind: "service", Name: strings.ToLower(input.RequestID)}, time.Now().UTC())
	c.DisplayName, c.Description = input.Edit.DisplayName, input.Edit.Description
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return catalog.Candidate{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.queries.WithTx(tx)
	row, err := q.GetDiscoveredService(ctx, c.ID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := s.upsertCandidate(ctx, q, c); err != nil {
			return catalog.Candidate{}, err
		}
		if err := s.saveEditTx(ctx, tx, c.ID, input.Edit); err != nil {
			return catalog.Candidate{}, err
		}
		row, err = q.GetDiscoveredService(ctx, c.ID)
	}
	if err != nil {
		return catalog.Candidate{}, err
	}
	result, err := s.candidateFromRow(ctx, q, discoveredServiceRowFromGet(row))
	if err != nil {
		return catalog.Candidate{}, err
	}
	if err := tx.Commit(); err != nil {
		return catalog.Candidate{}, err
	}
	return result, nil
}
