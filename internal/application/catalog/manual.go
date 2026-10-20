package catalog

import (
	"context"
	"encoding/hex"
)

const ManualSourceKind = "manual"

// ManualService uses a client-generated key so retries create at most one record.
// The key is scoped to manual/local and cannot select a discovered service.
type ManualService struct {
	RequestID string
	Edit      Edit
}

func (m ManualService) Validate() error {
	key, err := hex.DecodeString(m.RequestID)
	if err != nil || len(key) != 16 || m.Edit.Address == "" {
		return ErrInvalidEdit
	}
	return m.Edit.Validate()
}

func (s *Service) CreateManual(ctx context.Context, input ManualService) (Candidate, error) {
	input.Edit = input.Edit.Normalize()
	if err := input.Validate(); err != nil {
		return Candidate{}, err
	}
	return s.store.CreateManual(ctx, input)
}
