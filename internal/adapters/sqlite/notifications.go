package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/araihu/balemoh/internal/application/notifications"
	"github.com/araihu/balemoh/internal/storage/sqlc"
)

type NotificationStore struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewNotificationStore(db *sql.DB) notifications.Store {
	return &NotificationStore{db: db, queries: sqlc.New(db)}
}

func (s *NotificationStore) Publish(ctx context.Context, source notifications.Source, message notifications.Message, canonical []byte, receivedAt time.Time) (notifications.Receipt, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return notifications.Receipt{}, fmt.Errorf("begin notification publish: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)

	existing, err := queries.GetNotificationByIdentity(ctx, sqlc.GetNotificationByIdentityParams{
		SourceKind: source.Kind, SourceID: source.ID, ConnectionID: source.ConnectionID, EventID: message.EventID,
	})
	switch {
	case err == nil:
		if existing.PayloadJson != string(canonical) {
			return notifications.Receipt{}, notifications.ErrConflict
		}
		createdAt, parseErr := time.Parse(time.RFC3339Nano, existing.ReceivedAt)
		if parseErr != nil {
			return notifications.Receipt{}, fmt.Errorf("decode notification receipt time: %w", parseErr)
		}
		if err := tx.Commit(); err != nil {
			return notifications.Receipt{}, fmt.Errorf("commit notification replay: %w", err)
		}
		return notifications.Receipt{ID: existing.ID, CreatedAt: createdAt, Accepted: false}, nil
	case !errors.Is(err, sql.ErrNoRows):
		return notifications.Receipt{}, fmt.Errorf("read notification identity: %w", err)
	}

	count, err := queries.CountNotifications(ctx)
	if err != nil {
		return notifications.Receipt{}, fmt.Errorf("count notifications: %w", err)
	}
	if count >= notifications.MaxInboxSize {
		return notifications.Receipt{}, notifications.ErrCapacity
	}

	messageDestination, err := json.Marshal(message.Destination.Normalize())
	if err != nil {
		return notifications.Receipt{}, fmt.Errorf("marshal notification destination: %w", err)
	}
	id := uuid.New().String()
	receivedAt = receivedAt.UTC()
	row, err := queries.InsertNotification(ctx, sqlc.InsertNotificationParams{
		ID:              id,
		SourceKind:      source.Kind,
		SourceID:        source.ID,
		ConnectionID:    source.ConnectionID,
		EventID:         message.EventID,
		Severity:        string(message.Severity),
		Title:           message.Title,
		Body:            message.Body,
		OccurredAt:      message.OccurredAt.UTC().Format(time.RFC3339Nano),
		ReceivedAt:      receivedAt.Format(time.RFC3339Nano),
		DestinationJson: string(messageDestination),
		PayloadJson:     string(canonical),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return notifications.Receipt{}, err
		}
		return notifications.Receipt{}, fmt.Errorf("insert notification: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return notifications.Receipt{}, fmt.Errorf("commit notification publish: %w", err)
	}
	if row.ID == "" {
		return notifications.Receipt{}, fmt.Errorf("%w: empty inserted notification ID", notifications.ErrUnavailable)
	}
	return notifications.Receipt{ID: row.ID, CreatedAt: receivedAt, Accepted: true}, nil
}

func (s *NotificationStore) List(ctx context.Context, options notifications.ListOptions) (notifications.Page, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return notifications.Page{}, fmt.Errorf("begin notification list: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)

	limit := options.Limit
	if limit < 1 || limit > notifications.MaxPageSize {
		return notifications.Page{}, notifications.ErrInvalid
	}
	if options.Before > uint64(1<<63-1) {
		return notifications.Page{}, notifications.ErrInvalid
	}
	var before any
	var sequence int64
	if options.Before > 0 {
		sequence = int64(options.Before)
		before = sequence
	}
	rows, err := queries.ListNotifications(ctx, sqlc.ListNotificationsParams{
		Column1: before, Sequence: sequence, Column3: boolInt(options.UnreadOnly), Limit: int64(limit + 1),
	})
	if err != nil {
		return notifications.Page{}, fmt.Errorf("list notifications: %w", err)
	}
	unread, err := queries.CountUnreadNotifications(ctx)
	if err != nil {
		return notifications.Page{}, fmt.Errorf("count unread notifications: %w", err)
	}
	page := notifications.Page{Notifications: make([]notifications.Notification, 0, min(len(rows), limit)), UnreadCount: int(unread)}
	if len(rows) > limit {
		rows = rows[:limit]
		page.NextBefore = uint64(rows[len(rows)-1].Sequence)
	}
	for _, row := range rows {
		notification, err := notificationFromRow(row)
		if err != nil {
			return notifications.Page{}, err
		}
		page.Notifications = append(page.Notifications, notification)
	}
	if err := tx.Commit(); err != nil {
		return notifications.Page{}, fmt.Errorf("commit notification list: %w", err)
	}
	return page, nil
}

func (s *NotificationStore) Get(ctx context.Context, id string) (notifications.Notification, error) {
	row, err := s.queries.GetNotificationByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return notifications.Notification{}, notifications.ErrNotFound
	}
	if err != nil {
		return notifications.Notification{}, fmt.Errorf("get notification: %w", err)
	}
	return notificationFromRow(row)
}

func (s *NotificationStore) MarkRead(ctx context.Context, id string, readAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin notification read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)
	if _, err := queries.GetNotificationByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return notifications.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("read notification: %w", err)
	}
	if err := queries.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{ReadAt: sql.NullString{String: readAt.UTC().Format(time.RFC3339Nano), Valid: true}, ID: id}); err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit notification read: %w", err)
	}
	return nil
}

// A single DELETE provides an atomic boundary against concurrent publishers.
// Keep sqlite_sequence intact so stale page cursors cannot alias new messages.
func (s *NotificationStore) ClearAll(ctx context.Context) error {
	if err := s.queries.ClearNotifications(ctx); err != nil {
		return fmt.Errorf("clear notifications: %w", err)
	}
	return nil
}

func notificationFromRow(row sqlc.Notification) (notifications.Notification, error) {
	occurredAt, err := time.Parse(time.RFC3339Nano, row.OccurredAt)
	if err != nil {
		return notifications.Notification{}, fmt.Errorf("decode notification occurred time: %w", err)
	}
	receivedAt, err := time.Parse(time.RFC3339Nano, row.ReceivedAt)
	if err != nil {
		return notifications.Notification{}, fmt.Errorf("decode notification received time: %w", err)
	}
	var destination notifications.Destination
	if row.DestinationJson != "" && row.DestinationJson != "null" {
		if err := json.Unmarshal([]byte(row.DestinationJson), &destination); err != nil {
			return notifications.Notification{}, fmt.Errorf("decode notification destination: %w", err)
		}
	}
	var readAt *time.Time
	if row.ReadAt.Valid {
		value, err := time.Parse(time.RFC3339Nano, row.ReadAt.String)
		if err != nil {
			return notifications.Notification{}, fmt.Errorf("decode notification read time: %w", err)
		}
		readAt = &value
	}
	return notifications.Notification{
		ID:      row.ID,
		Source:  notifications.Source{Kind: row.SourceKind, ID: row.SourceID, ConnectionID: row.ConnectionID},
		EventID: row.EventID, Severity: notifications.Severity(row.Severity), Title: row.Title, Body: row.Body,
		OccurredAt: occurredAt, ReceivedAt: receivedAt, Destination: destination, ReadAt: readAt,
	}, nil
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ notifications.Store = (*NotificationStore)(nil)
