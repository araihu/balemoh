package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/notifications"
	push "github.com/araihu/balemoh/internal/application/webpush"
)

func pushDatabase(t *testing.T) (*sql.DB, *WebPushStore, *notifications.Service, string) {
	t.Helper()
	path := t.TempDir() + "/push.db"
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	store := NewWebPushStore(db)
	if err = store.Configure(t.Context(), "public", true); err != nil {
		t.Fatal(err)
	}
	return db, store, notifications.NewService(NewNotificationStore(db)), path
}
func savePush(t *testing.T, store *WebPushStore, id, endpoint string) {
	t.Helper()
	if err := store.Save(t.Context(), id, push.Subscription{Endpoint: endpoint, Keys: push.Keys{P256dh: "key", Auth: "auth"}}, time.Now().Add(90*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
}
func publishPush(t *testing.T, service *notifications.Service, event string) notifications.Receipt {
	t.Helper()
	receipt, err := service.Publish(t.Context(), notifications.Source{Kind: notifications.SourceExternal, ID: "monitor"}, testMessage(event, "Push event", time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}
func TestPushQueueDurableAtomicAndNoInboxReplay(t *testing.T) {
	db, store, service, path := pushDatabase(t)
	publishPush(t, service, "baseline")
	savePush(t, store, "device", "https://push.example/device")
	if _, err := store.Next(t.Context(), time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("old inbox queued: %v", err)
	}
	first := publishPush(t, service, "new")
	original, err := service.Get(t.Context(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(t.Context(), notifications.Source{Kind: notifications.SourceExternal, ID: "monitor"}, testMessage("new", "Push event", original.OccurredAt)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	store = NewWebPushStore(reopened)
	d, err := store.Next(t.Context(), time.Now())
	if err != nil || d.NotificationID != first.ID || d.Attempts != 1 {
		t.Fatalf("restarted queue: %+v %v", d, err)
	}
	if _, err = store.Next(t.Context(), time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("lease did not exclude concurrent send")
	}
	if err = store.Complete(t.Context(), d, 201, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Next(t.Context(), time.Now().Add(time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("duplicate publish queued again")
	}
}
func TestPushQueueRollbackReadClearAndDisable(t *testing.T) {
	db, store, service, _ := pushDatabase(t)
	savePush(t, store, "device", "https://push.example/device")
	if _, err := db.Exec(`CREATE TRIGGER fail_push BEFORE INSERT ON web_push_deliveries BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := service.Publish(t.Context(), notifications.Source{Kind: notifications.SourceExternal, ID: "monitor"}, testMessage("failed", "Push event", time.Now()))
	if err == nil {
		t.Fatal("publish accepted without durable delivery")
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count); err != nil || count != 0 {
		t.Fatal("inbox publish was not rolled back")
	}
	if _, err = db.Exec("DROP TRIGGER fail_push"); err != nil {
		t.Fatal(err)
	}
	receipt := publishPush(t, service, "read")
	if err = service.MarkRead(t.Context(), receipt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Next(t.Context(), time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("read message sent")
	}
	publishPush(t, service, "clear")
	if err = service.ClearAll(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Next(t.Context(), time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("cleared message sent")
	}
	if err = store.Configure(t.Context(), "", false); err != nil {
		t.Fatal(err)
	}
	publishPush(t, service, "disabled")
	if _, err = store.Next(t.Context(), time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("disabled push queued")
	}
}
func TestPushOwnershipRetryExpiryAndKeyRotation(t *testing.T) {
	_, store, service, _ := pushDatabase(t)
	now := time.Now()
	savePush(t, store, "device", "https://push.example/device")
	if err := store.Save(t.Context(), "other", push.Subscription{Endpoint: "https://push.example/device"}, now.Add(time.Hour)); !errors.Is(err, push.ErrConflict) {
		t.Fatal("other device stole endpoint")
	}
	publishPush(t, service, "retry")
	d, err := store.Next(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(t.Context(), d, 503, now); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Next(t.Context(), now); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("retry did not back off")
	}
	d, err = store.Next(t.Context(), now.Add(3*time.Minute))
	if err != nil || d.Attempts != 2 {
		t.Fatal("retry missing")
	}
	// A stale 410 from the replaced endpoint must not delete its replacement.
	savePush(t, store, "device", "https://push.example/replacement")
	if err = store.Complete(t.Context(), d, 410, now); err != nil {
		t.Fatal(err)
	}
	exists, err := store.Exists(t.Context(), "device", now)
	if err != nil || !exists {
		t.Fatal("stale send removed replacement")
	}
	publishPush(t, service, "gone")
	d, err = store.Next(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(t.Context(), d, 410, now); err != nil {
		t.Fatal(err)
	}
	exists, _ = store.Exists(t.Context(), "device", now)
	if exists {
		t.Fatal("expired endpoint retained")
	}
	savePush(t, store, "device", "https://push.example/device")
	publishPush(t, service, "expires")
	if _, err = store.Next(t.Context(), now.Add(25*time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired delivery retained")
	}
	if err = store.Configure(t.Context(), "rotated-public-key", true); err != nil {
		t.Fatal(err)
	}
	exists, _ = store.Exists(t.Context(), "device", now)
	if exists {
		t.Fatal("old VAPID subscription retained after rotation")
	}
}
func TestPushLeaseRecoversAfterWorkerStops(t *testing.T) {
	_, store, service, _ := pushDatabase(t)
	savePush(t, store, "device", "https://push.example/device")
	publishPush(t, service, "lease")
	now := time.Now()
	d, err := store.Next(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.Next(context.Background(), now.Add(2*time.Minute))
	if err != nil || again.NotificationID != d.NotificationID || again.Attempts != 2 {
		t.Fatalf("lease recovery: %+v %v", again, err)
	}
}
