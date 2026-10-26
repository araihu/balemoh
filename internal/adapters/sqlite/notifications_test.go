package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/notifications"
)

func openNotificationService(t *testing.T) (*notifications.Service, func()) {
	t.Helper()
	db, err := Open(context.Background(), t.TempDir()+"/balemoh.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	store := NewNotificationStore(db)
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	return notifications.NewServiceWithClock(store, func() time.Time { return now }), func() { _ = db.Close() }
}

func testMessage(eventID, title string, now time.Time) notifications.Message {
	return notifications.Message{EventID: eventID, Severity: notifications.SeverityInfo, Title: title, OccurredAt: now}
}

func TestNotificationStoreReplayConflictReadAndReopen(t *testing.T) {
	databasePath := t.TempDir() + "/balemoh.db"
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	db, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	service := notifications.NewServiceWithClock(NewNotificationStore(db), func() time.Time { return now })
	source := notifications.Source{Kind: notifications.SourceExternal, ID: "monitor"}
	message := testMessage("event-1", "<script>alert(1)</script>", now)
	first, err := service.Publish(context.Background(), source, message)
	if err != nil || !first.Accepted {
		t.Fatalf("first publish = %#v, %v", first, err)
	}
	if err := service.MarkRead(context.Background(), first.ID); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}
	replay, err := service.Publish(context.Background(), source, message)
	if err != nil || replay.Accepted || replay.ID != first.ID {
		t.Fatalf("replay = %#v, %v; want same non-accepted receipt", replay, err)
	}
	conflictMessage := message
	conflictMessage.Title = "different"
	if _, err := service.Publish(context.Background(), source, conflictMessage); !errors.Is(err, notifications.ErrConflict) {
		t.Fatalf("conflict error = %v, want ErrConflict", err)
	}
	page, err := service.List(context.Background(), notifications.ListOptions{Limit: 10})
	if err != nil || len(page.Notifications) != 1 || page.UnreadCount != 0 || page.Notifications[0].ReadAt == nil {
		t.Fatalf("page after read = %#v, %v", page, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	reopened := notifications.NewServiceWithClock(NewNotificationStore(db), func() time.Time { return now })
	got, err := reopened.Get(context.Background(), first.ID)
	if err != nil || got.Title != message.Title || got.ReadAt == nil {
		t.Fatalf("reopened notification = %#v, %v", got, err)
	}
}

func TestNotificationIdentityIncludesConnection(t *testing.T) {
	service, cleanup := openNotificationService(t)
	defer cleanup()
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	message := testMessage("same-event", "title", now)
	pluginA := notifications.Source{Kind: notifications.SourcePlugin, ID: "alerts", ConnectionID: "550e8400-e29b-41d4-a716-446655440000"}
	pluginB := notifications.Source{Kind: notifications.SourcePlugin, ID: "alerts", ConnectionID: "550e8400-e29b-41d4-a716-446655440001"}
	first, err := service.Publish(context.Background(), pluginA, message)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Publish(context.Background(), pluginB, message)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatalf("different plugin connections reused ID %q", first.ID)
	}
	if _, err := service.Publish(context.Background(), notifications.Source{Kind: notifications.SourceCore, ID: "alerts"}, message); err != nil {
		t.Fatalf("core source with empty connection rejected: %v", err)
	}
	if _, err := service.Publish(context.Background(), notifications.Source{Kind: notifications.SourceCore, ID: "alerts", ConnectionID: pluginA.ConnectionID}, nilMessage(now)); !errors.Is(err, notifications.ErrInvalid) {
		t.Fatalf("core source connection error = %v, want ErrInvalid", err)
	}
}

func nilMessage(now time.Time) notifications.Message {
	return testMessage("another-event", "title", now)
}

func TestNotificationPaginationAndUnreadFilter(t *testing.T) {
	service, cleanup := openNotificationService(t)
	defer cleanup()
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	source := notifications.Source{Kind: notifications.SourceCore, ID: "core"}
	for index := 0; index < 3; index++ {
		if _, err := service.Publish(context.Background(), source, testMessage(fmt.Sprintf("event-%d", index), fmt.Sprintf("title-%d", index), now)); err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.List(context.Background(), notifications.ListOptions{Limit: 2})
	if err != nil || len(page.Notifications) != 2 || page.NextBefore == 0 || page.UnreadCount != 3 {
		t.Fatalf("first page = %#v, %v", page, err)
	}
	page, err = service.List(context.Background(), notifications.ListOptions{Before: page.NextBefore, Limit: 2})
	if err != nil || len(page.Notifications) != 1 || page.NextBefore != 0 || page.UnreadCount != 3 {
		t.Fatalf("second page = %#v, %v", page, err)
	}
	if err := service.MarkRead(context.Background(), page.Notifications[0].ID); err != nil {
		t.Fatal(err)
	}
	unread, err := service.List(context.Background(), notifications.ListOptions{UnreadOnly: true, Limit: 50})
	if err != nil || len(unread.Notifications) != 2 || unread.UnreadCount != 2 {
		t.Fatalf("unread page = %#v, %v", unread, err)
	}
}

func TestNotificationPublishRacesDoNotDuplicate(t *testing.T) {
	service, cleanup := openNotificationService(t)
	defer cleanup()
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	source := notifications.Source{Kind: notifications.SourceExternal, ID: "race"}
	const workers = 32
	results := make(chan notifications.Receipt, workers)
	errorsCh := make(chan error, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			receipt, err := service.Publish(context.Background(), source, testMessage("same", "same", now))
			if err != nil {
				errorsCh <- err
				return
			}
			results <- receipt
		}()
	}
	group.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("racing publish error = %v", err)
	}
	var accepted int
	var id string
	for receipt := range results {
		if receipt.Accepted {
			accepted++
		}
		if id == "" {
			id = receipt.ID
		} else if receipt.ID != id {
			t.Fatalf("racing receipt IDs differ: %q and %q", id, receipt.ID)
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted publishes = %d, want 1", accepted)
	}
}

func TestNotificationCapacityAllowsReplayButRejectsNewEvent(t *testing.T) {
	service, cleanup := openNotificationService(t)
	defer cleanup()
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	source := notifications.Source{Kind: notifications.SourceCore, ID: "capacity"}
	firstMessage := testMessage("event-0", "first", now)
	first, err := service.Publish(context.Background(), source, firstMessage)
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index < notifications.MaxInboxSize; index++ {
		if _, err := service.Publish(context.Background(), source, testMessage(fmt.Sprintf("event-%d", index), "title", now)); err != nil {
			t.Fatalf("fill publish %d: %v", index, err)
		}
	}
	replay, err := service.Publish(context.Background(), source, firstMessage)
	if err != nil || replay.Accepted || replay.ID != first.ID {
		t.Fatalf("capacity replay = %#v, %v", replay, err)
	}
	if _, err := service.Publish(context.Background(), source, testMessage("event-new", "new", now)); !errors.Is(err, notifications.ErrCapacity) {
		t.Fatalf("capacity new event error = %v, want ErrCapacity", err)
	}
	if err := service.ClearAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if receipt, err := service.Publish(context.Background(), source, testMessage("event-new", "new", now)); err != nil || !receipt.Accepted {
		t.Fatalf("clear did not free capacity: %#v, %v", receipt, err)
	}
}

func TestClearNotificationsIncludesReadUnreadAndAllPages(t *testing.T) {
	service, cleanup := openNotificationService(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	var ids []string
	for i := 0; i < 55; i++ {
		source := notifications.Source{Kind: notifications.SourceCore, ID: fmt.Sprintf("source-%d", i%2)}
		receipt, err := service.Publish(ctx, source, testMessage(fmt.Sprintf("event-%d", i), "message", now))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, receipt.ID)
		if i%2 == 0 {
			if err := service.MarkRead(ctx, receipt.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	page, err := service.List(ctx, notifications.ListOptions{Limit: 50})
	if err != nil || len(page.Notifications) != 50 || page.NextBefore == 0 {
		t.Fatalf("setup page = %#v, %v", page, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := service.ClearAll(canceled); err == nil {
		t.Fatal("canceled clear succeeded")
	}
	if _, err := service.Get(ctx, ids[0]); err != nil {
		t.Fatal("canceled clear removed data", err)
	}
	for range 2 {
		if err := service.ClearAll(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, unread := range []bool{false, true} {
		page, err := service.List(ctx, notifications.ListOptions{Limit: 50, UnreadOnly: unread})
		if err != nil || len(page.Notifications) != 0 || page.UnreadCount != 0 || page.NextBefore != 0 {
			t.Fatalf("after clear = %#v, %v", page, err)
		}
	}
	for _, id := range ids {
		if _, err := service.Get(ctx, id); !errors.Is(err, notifications.ErrNotFound) {
			t.Fatalf("old message %s = %v", id, err)
		}
	}
	// Clear deliberately resets dedup identities; a subsequent publish survives
	// and cannot alias a stale pagination cursor because sequences are not reset.
	next, err := service.Publish(ctx, notifications.Source{Kind: notifications.SourceCore, ID: "source-0"}, testMessage("event-0", "message", now))
	if err != nil || !next.Accepted || next.ID == ids[0] {
		t.Fatalf("post-clear publish = %#v, %v", next, err)
	}
	page, err = service.List(ctx, notifications.ListOptions{Limit: 50})
	if err != nil || len(page.Notifications) != 1 || page.UnreadCount != 1 {
		t.Fatalf("new page = %#v, %v", page, err)
	}
	oldPage, err := service.List(ctx, notifications.ListOptions{Limit: 50, Before: 6})
	if err != nil || len(oldPage.Notifications) != 0 {
		t.Fatal("old cursor aliased new messages", err)
	}
}

func TestNotificationRetentionAndBulkRead(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir()+"/inbox.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	svc, err := notifications.NewServiceWithRetention(NewNotificationStore(db), 30, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	publish := func(id string) notifications.Receipt {
		t.Helper()
		r, e := svc.Publish(ctx, notifications.Source{Kind: notifications.SourceCore, ID: "test"}, testMessage(id, id, now))
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	oldRead := publish("old-read")
	if err := svc.MarkRead(ctx, oldRead.ID); err != nil {
		t.Fatal(err)
	}
	oldUnread := publish("old-unread")
	now = now.Add(24 * time.Hour)
	boundary := publish("boundary")
	now = now.Add(30 * 24 * time.Hour)
	// Expired rows are hidden immediately, before the asynchronous physical sweep.
	page, err := svc.List(ctx, notifications.ListOptions{})
	if err != nil || len(page.Notifications) != 1 || page.Notifications[0].ID != boundary.ID || page.UnreadCount != 1 || page.RetentionDays != 30 {
		t.Fatalf("retained page=%#v, %v", page, err)
	}
	for _, id := range []string{oldRead.ID, oldUnread.ID} {
		if _, err := svc.Get(ctx, id); !errors.Is(err, notifications.ErrNotFound) {
			t.Fatalf("expired get=%v", err)
		}
		if err := svc.MarkRead(ctx, id); !errors.Is(err, notifications.ErrNotFound) {
			t.Fatal("expired read accepted", err)
		}
	}
	var total int
	if err := db.QueryRow("SELECT count(*) FROM notifications").Scan(&total); err != nil || total != 3 {
		t.Fatal("setup physically purged before sweep", total, err)
	}
	removed, err := svc.PruneExpired(ctx)
	if err != nil || removed != 2 {
		t.Fatalf("prune=%d, %v", removed, err)
	}
	if n, err := svc.PruneExpired(ctx); err != nil || n != 0 {
		t.Fatal("repeat prune", n, err)
	}
	kept := publish("current-read")
	if err := svc.MarkAllRead(ctx); err != nil {
		t.Fatal(err)
	}
	item, err := svc.Get(ctx, kept.ID)
	if err != nil || item.ReadAt == nil {
		t.Fatal("bulk read did not persist", err)
	}
	stamp := *item.ReadAt
	now = now.Add(time.Second)
	if err := svc.MarkAllRead(ctx); err != nil {
		t.Fatal(err)
	}
	item, err = svc.Get(ctx, kept.ID)
	if err != nil || !item.ReadAt.Equal(stamp) {
		t.Fatal("bulk read rewrote existing timestamp", err)
	}
	fresh := publish("after-bulk")
	page, err = svc.List(ctx, notifications.ListOptions{UnreadOnly: true})
	if err != nil || len(page.Notifications) != 1 || page.Notifications[0].ID != fresh.ID || page.UnreadCount != 1 {
		t.Fatalf("post-bulk publication lost unread state: %#v, %v", page, err)
	}
	// An expired identity is accepted anew, rather than returning an invisible receipt.
	replay := publish("old-unread")
	if !replay.Accepted || replay.ID == oldUnread.ID {
		t.Fatal("expired identity replay was not renewed")
	}
}

func TestMarkAllNotificationsReadSpansPagesAndPreservesContent(t *testing.T) {
	svc, cleanup := openNotificationService(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for i := range 60 {
		if _, err := svc.Publish(ctx, notifications.Source{Kind: notifications.SourceCore, ID: "many"}, testMessage(fmt.Sprint(i), "body unchanged", now)); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.MarkAllRead(ctx); err != nil {
		t.Fatal(err)
	}
	page, err := svc.List(ctx, notifications.ListOptions{})
	if err != nil || len(page.Notifications) != 50 || page.NextBefore == 0 || page.UnreadCount != 0 {
		t.Fatalf("bulk page=%#v, %v", page, err)
	}
	for _, item := range page.Notifications {
		if item.ReadAt == nil || item.Title != "body unchanged" {
			t.Fatal("content changed or unread")
		}
	}
	page, err = svc.List(ctx, notifications.ListOptions{Before: page.NextBefore})
	if err != nil || len(page.Notifications) != 10 {
		t.Fatal("older page lost", err)
	}
	for _, item := range page.Notifications {
		if item.ReadAt == nil {
			t.Fatal("older page not read")
		}
	}
}

func TestExpiredCapacityIsReclaimedAtomically(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir()+"/capacity.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	svc, err := notifications.NewServiceWithRetention(NewNotificationStore(db), 30, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Publish(ctx, notifications.Source{Kind: notifications.SourceCore, ID: "capacity"}, testMessage("first", "first", now))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<9999)
 INSERT INTO notifications(id,source_kind,source_id,connection_id,event_id,severity,title,body,occurred_at,received_at,destination_json,payload_json)
 SELECT 'fixture-'||x,source_kind,source_id,connection_id,'event-'||x,severity,title,body,occurred_at,received_at,destination_json,payload_json FROM notifications,n WHERE event_id='first'`)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * 24 * time.Hour)
	r, err := svc.Publish(ctx, notifications.Source{Kind: notifications.SourceCore, ID: "capacity"}, testMessage("new", "new", now))
	if err != nil || !r.Accepted {
		t.Fatalf("expired capacity remained full: %#v %v", r, err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM notifications").Scan(&count); err != nil || count != 1 {
		t.Fatalf("physical rows=%d, %v", count, err)
	}
}
