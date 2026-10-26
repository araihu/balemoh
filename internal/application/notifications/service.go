package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Store is the durable notification port. Implementations must resolve an
// existing identity before capacity checks and keep publish identity and
// capacity decisions in one transaction.
type Store interface {
	Publish(context.Context, Source, Message, []byte, time.Time) (Receipt, error)
	List(context.Context, ListOptions) (Page, error)
	Get(context.Context, string) (Notification, error)
	MarkRead(context.Context, string, time.Time) error
}

// Service owns notification validation and the core application contract.
type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return NewServiceWithClock(store, time.Now)
}

func NewServiceWithClock(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

// Publish durably accepts one message or returns an identity/capacity error.
// Identical retries return the original receipt with Accepted=false.
func (s *Service) Publish(ctx context.Context, source Source, message Message) (Receipt, error) {
	if s == nil || s.store == nil {
		return Receipt{}, ErrUnavailable
	}
	source = source.Normalize()
	message = message.Normalize()
	now := s.now().UTC()
	if err := source.Validate(); err != nil {
		return Receipt{}, err
	}
	if err := message.Validate(now); err != nil {
		return Receipt{}, err
	}
	if err := validateDestinationScope(source, message.Destination); err != nil {
		return Receipt{}, err
	}
	canonical, err := CanonicalPayload(source, message)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}
	if len(canonical) > MaxEnvelopeBytes {
		return Receipt{}, ErrTooLarge
	}
	return s.store.Publish(ctx, source, message, canonical, now)
}

func validateDestinationScope(source Source, destination Destination) error {
	if destination.Kind != SourcePlugin {
		return nil
	}
	if source.Kind != SourcePlugin || destination.PluginID != source.ID || destination.ConnectionID != source.ConnectionID {
		return fmt.Errorf("%w: plugin destination must match publishing source", ErrInvalid)
	}
	return nil
}

func (s *Service) List(ctx context.Context, options ListOptions) (Page, error) {
	if s == nil || s.store == nil {
		return Page{}, ErrUnavailable
	}
	if options.Limit == 0 {
		options.Limit = MaxPageSize
	}
	if options.Limit < 1 || options.Limit > MaxPageSize {
		return Page{}, ErrInvalid
	}
	return s.store.List(ctx, options)
}

func (s *Service) Get(ctx context.Context, id string) (Notification, error) {
	if s == nil || s.store == nil {
		return Notification{}, ErrUnavailable
	}
	if strings.TrimSpace(id) == "" || len(id) > 128 {
		return Notification{}, ErrNotFound
	}
	return s.store.Get(ctx, strings.TrimSpace(id))
}

func (s *Service) MarkRead(ctx context.Context, id string) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	if strings.TrimSpace(id) == "" || len(id) > 128 {
		return ErrNotFound
	}
	return s.store.MarkRead(ctx, strings.TrimSpace(id), s.now().UTC())
}

var _ interface {
	Publish(context.Context, Source, Message) (Receipt, error)
	List(context.Context, ListOptions) (Page, error)
	Get(context.Context, string) (Notification, error)
	MarkRead(context.Context, string) error
} = (*Service)(nil)
