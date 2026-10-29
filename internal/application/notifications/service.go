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
	BindService(context.Context, Source, string, string, time.Time) (Receipt, error)
	Publish(context.Context, Source, Message, []byte, time.Time, time.Time) (Receipt, error)
	List(context.Context, ListOptions) (Page, error)
	Get(context.Context, string) (Notification, error)
	MarkRead(context.Context, string, time.Time) error
	ClearAll(context.Context) error
	MarkAllRead(context.Context, time.Time, time.Time) error
	PruneExpired(context.Context, time.Time) (int64, error)
}

// Service owns notification validation and the core application contract.
type Service struct {
	store         Store
	now           func() time.Time
	retentionDays int
}

func NewService(store Store) *Service {
	return NewServiceWithClock(store, time.Now)
}

func NewServiceWithClock(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now, retentionDays: DefaultRetentionDays}
}

// NewServiceWithRetention configures one policy shared by HTTP, plugins and cleanup.
func NewServiceWithRetention(store Store, days int, now func() time.Time) (*Service, error) {
	if days < 1 || days > MaxRetentionDays {
		return nil, ErrInvalid
	}
	s := NewServiceWithClock(store, now)
	s.retentionDays = days
	return s, nil
}

func (s *Service) cutoff() time.Time {
	return s.now().UTC().Truncate(time.Second).Add(-time.Duration(s.retentionDays) * 24 * time.Hour)
}

// PruneExpired permanently removes expired read and unread rows and identities.
func (s *Service) PruneExpired(ctx context.Context) (int64, error) {
	if s == nil || s.store == nil {
		return 0, ErrUnavailable
	}
	return s.store.PruneExpired(ctx, s.cutoff())
}

func (s *Service) MarkAllRead(ctx context.Context) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	return s.store.MarkAllRead(ctx, s.now().UTC(), s.cutoff())
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
	envelopeBytes, err := EnvelopeBytes(message)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}
	if envelopeBytes > MaxEnvelopeBytes {
		return Receipt{}, ErrTooLarge
	}
	canonical, err := CanonicalPayload(source, message)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}
	return s.store.Publish(ctx, source, message, canonical, now, s.cutoff())
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
	options.Since = s.cutoff()
	page, err := s.store.List(ctx, options)
	page.RetentionDays = s.retentionDays
	return page, err
}

func (s *Service) Get(ctx context.Context, id string) (Notification, error) {
	if s == nil || s.store == nil {
		return Notification{}, ErrUnavailable
	}
	if strings.TrimSpace(id) == "" || len(id) > 128 {
		return Notification{}, ErrNotFound
	}
	item, err := s.store.Get(ctx, strings.TrimSpace(id))
	if err == nil && item.ReceivedAt.Unix() < s.cutoff().Unix() {
		return Notification{}, ErrNotFound
	}
	return item, err
}

func (s *Service) MarkRead(ctx context.Context, id string) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	if strings.TrimSpace(id) == "" || len(id) > 128 {
		return ErrNotFound
	}
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.store.MarkRead(ctx, strings.TrimSpace(id), s.now().UTC())
}

// ClearAll permanently removes the inbox and its deduplication identities.
// Publishers committing after the atomic clear retain their messages.
func (s *Service) ClearAll(ctx context.Context) error {
	if s == nil || s.store == nil {
		return ErrUnavailable
	}
	return s.store.ClearAll(ctx)
}

var _ interface {
	Publish(context.Context, Source, Message) (Receipt, error)
	List(context.Context, ListOptions) (Page, error)
	Get(context.Context, string) (Notification, error)
	MarkRead(context.Context, string) error
	ClearAll(context.Context) error
} = (*Service)(nil)

// BindService enriches only an existing, source-owned event. It deliberately
// does not publish: startup baselines must not create or revive notifications.
func (s *Service) BindService(ctx context.Context, source Source, eventID, serviceID string) (Receipt, error) {
	if s == nil || s.store == nil {
		return Receipt{}, ErrUnavailable
	}
	source = source.Normalize()
	if err := source.Validate(); err != nil {
		return Receipt{}, err
	}
	if err := validateText(eventID, "event ID", MaxEventIDBytes, false); err != nil {
		return Receipt{}, err
	}
	if err := validateServiceID(serviceID); err != nil || serviceID == "" {
		return Receipt{}, ErrInvalid
	}
	return s.store.BindService(ctx, source, eventID, serviceID, s.cutoff())
}
