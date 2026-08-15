package sqlite

import (
	"context"

	"github.com/araihu/balemoh/internal/application/health"
	"github.com/araihu/balemoh/internal/storage/sqlc"
)

type Pinger struct {
	queries *sqlc.Queries
}

func NewPinger(q *sqlc.Queries) health.Pinger { return Pinger{queries: q} }

func (p Pinger) Ping(ctx context.Context) error {
	_, err := p.queries.Ping(ctx)
	return err
}

var _ health.Pinger = Pinger{}
