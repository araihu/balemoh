package sqlite

import (
	"context"
	"database/sql"
	"errors"
	push "github.com/araihu/balemoh/internal/application/webpush"
	"time"
)

type WebPushStore struct{ db *sql.DB }

func NewWebPushStore(db *sql.DB) *WebPushStore { return &WebPushStore{db: db} }
func (s *WebPushStore) Configure(ctx context.Context, key string, enabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous string
	if err = tx.QueryRowContext(ctx, "SELECT public_key FROM web_push_state WHERE singleton=1").Scan(&previous); err != nil {
		return err
	}
	if key != "" && key != previous {
		if _, err = tx.ExecContext(ctx, "DELETE FROM web_push_subscriptions"); err != nil {
			return err
		}
	}
	if key == "" {
		key = previous
	}
	if _, err = tx.ExecContext(ctx, "UPDATE web_push_state SET public_key=?, enabled=? WHERE singleton=1", key, enabled); err != nil {
		return err
	}
	if !enabled {
		if _, err = tx.ExecContext(ctx, "DELETE FROM web_push_deliveries"); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *WebPushStore) Save(ctx context.Context, id string, sub push.Subscription, expires time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner string
	err = tx.QueryRowContext(ctx, "SELECT id FROM web_push_subscriptions WHERE endpoint=?", sub.Endpoint).Scan(&owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && owner != id {
		return push.ErrConflict
	}
	var count int
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*), EXISTS(SELECT 1 FROM web_push_subscriptions WHERE id=?) FROM web_push_subscriptions", id).Scan(&count, &exists); err != nil {
		return err
	}
	if count >= 128 && !exists {
		return push.ErrCapacity
	}
	// Renewals retain pending jobs; an endpoint/key replacement invalidates them.
	if _, err = tx.ExecContext(ctx, `DELETE FROM web_push_deliveries WHERE subscription_id=? AND EXISTS(SELECT 1 FROM web_push_subscriptions WHERE id=? AND (endpoint<>? OR p256dh<>? OR auth<>?))`, id, id, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO web_push_subscriptions(id,endpoint,p256dh,auth,expires_at) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET endpoint=excluded.endpoint,p256dh=excluded.p256dh,auth=excluded.auth,expires_at=excluded.expires_at`, id, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth, expires.Unix())
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *WebPushStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM web_push_subscriptions WHERE id=?", id)
	return err
}
func (s *WebPushStore) Exists(ctx context.Context, id string, now time.Time) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM web_push_subscriptions WHERE id=? AND expires_at>?)", id, now.Unix()).Scan(&exists)
	return exists, err
}
func (s *WebPushStore) Next(ctx context.Context, now time.Time) (push.Delivery, error) {
	// Clear expired jobs, read/deleted messages, and expired subscriptions first.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return push.Delivery{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM web_push_subscriptions WHERE expires_at<=?", now.Unix()); err != nil {
		return push.Delivery{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM web_push_deliveries WHERE expires_at<=? OR notification_id IN(SELECT id FROM notifications WHERE read_at IS NOT NULL)`, now.Unix()); err != nil {
		return push.Delivery{}, err
	}
	var d push.Delivery
	err = tx.QueryRowContext(ctx, `UPDATE web_push_deliveries SET lease_until=?,attempts=attempts+1 WHERE rowid=(SELECT rowid FROM web_push_deliveries WHERE next_attempt<=? AND lease_until<=? ORDER BY next_attempt,rowid LIMIT 1) RETURNING subscription_id,notification_id,attempts`, now.Add(time.Minute).Unix(), now.Unix(), now.Unix()).Scan(&d.SubscriptionID, &d.NotificationID, &d.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return d, err
		}
		return d, sql.ErrNoRows
	}
	if err != nil {
		return d, err
	}
	err = tx.QueryRowContext(ctx, `SELECT s.endpoint,s.p256dh,s.auth,n.title,n.body FROM web_push_subscriptions s JOIN notifications n ON n.id=? WHERE s.id=?`, d.NotificationID, d.SubscriptionID).Scan(&d.Subscription.Endpoint, &d.Subscription.Keys.P256dh, &d.Subscription.Keys.Auth, &d.Title, &d.Body)
	if err != nil {
		return d, err
	}
	return d, tx.Commit()
}
func (s *WebPushStore) Complete(ctx context.Context, d push.Delivery, status int, now time.Time) error {
	if status == 404 || status == 410 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM web_push_subscriptions WHERE id=? AND endpoint=? AND p256dh=? AND auth=? AND EXISTS(SELECT 1 FROM web_push_deliveries WHERE subscription_id=? AND notification_id=? AND attempts=?)`, d.SubscriptionID, d.Subscription.Endpoint, d.Subscription.Keys.P256dh, d.Subscription.Keys.Auth, d.SubscriptionID, d.NotificationID, d.Attempts)
		return err
	}
	if (status >= 200 && status < 300) || d.Attempts >= 8 || (status >= 400 && status < 500 && status != 408 && status != 429) {
		_, err := s.db.ExecContext(ctx, "DELETE FROM web_push_deliveries WHERE subscription_id=? AND notification_id=? AND attempts=?", d.SubscriptionID, d.NotificationID, d.Attempts)
		return err
	}
	delay := time.Duration(1<<min(d.Attempts, 6)) * time.Minute
	_, err := s.db.ExecContext(ctx, "UPDATE web_push_deliveries SET next_attempt=?,lease_until=0 WHERE subscription_id=? AND notification_id=? AND attempts=?", now.Add(delay).Unix(), d.SubscriptionID, d.NotificationID, d.Attempts)
	return err
}

var _ push.Store = (*WebPushStore)(nil)
