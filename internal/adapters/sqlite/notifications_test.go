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
}
