package sqlite

import (
	"context"
	"github.com/araihu/balemoh/internal/application/catalog"
)

func (s *CatalogStore) Lifecycle(ctx context.Context, id, action string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.queries.WithTx(tx)
	rows, err := q.ListDiscoveredServices(ctx)
	if err != nil {
		return err
	}
	var observations []catalog.Candidate
	for _, row := range rows {
		c, err := s.candidateFromRow(ctx, q, discoveredServiceRowFromList(row))
		if err != nil {
			return err
		}
		observations = append(observations, c)
	}
	ids, err := catalog.LifecycleTargets(observations, id, action)
	if err != nil {
		return err
	}
	for _, member := range ids {
		var statement string
		switch action {
		case "hide":
			statement = "UPDATE discovered_services SET hidden = 1 WHERE id = ?"
		case "show":
			statement = "UPDATE discovered_services SET hidden = 0 WHERE id = ?"
		case "purge":
			statement = "DELETE FROM discovered_services WHERE id = ? AND missing = 1"
		}
		if _, err := tx.ExecContext(ctx, statement, member); err != nil {
			return err
		}
	}
	return tx.Commit()
}
