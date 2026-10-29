package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/internal/application/notifications"
)

func TestNotificationServiceReferencePreservesRetainedEvent(t *testing.T) {
	service, closeDB := openNotificationService(t)
	defer closeDB()
	ctx := context.Background()
	source := notifications.Source{Kind: notifications.SourcePlugin, ID: "example.alerts", ConnectionID: "00000000-0000-4000-8000-000000000001"}
	at := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	message := testMessage("existing", "Original event", at)
	receipt, err := service.Publish(ctx, source, message)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.MarkRead(ctx, receipt.ID); err != nil {
		t.Fatal(err)
	}
	before, err := service.Get(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	bound, err := service.BindService(ctx, source, message.EventID, id)
	if err != nil || bound.ID != receipt.ID || bound.Accepted {
		t.Fatal(bound, err)
	}
	after, err := service.Get(ctx, receipt.ID)
	if err != nil || after.ServiceID != id {
		t.Fatal(after, err)
	}
	after.ServiceID = ""
	if !reflect.DeepEqual(before, after) {
		t.Fatal("binding changed immutable content, receipt, destination or read state")
	}
	if _, err = service.BindService(ctx, source, message.EventID, id); err != nil {
		t.Fatal("same binding is not idempotent", err)
	}
	if _, err = service.BindService(ctx, source, message.EventID, strings.Repeat("b", 64)); !errors.Is(err, notifications.ErrConflict) {
		t.Fatal("replaced first binding", err)
	}
	other := source
	other.ConnectionID = "00000000-0000-4000-8000-000000000002"
	if _, err = service.BindService(ctx, other, message.EventID, id); !errors.Is(err, notifications.ErrNotFound) {
		t.Fatal("crossed source scope", err)
	}
	if _, err = service.BindService(ctx, source, "absent", id); !errors.Is(err, notifications.ErrNotFound) {
		t.Fatal("binding created missing event", err)
	}
	message.ServiceID = id
	if replay, err := service.Publish(ctx, source, message); err != nil || replay.Accepted || replay.ID != receipt.ID {
		t.Fatal("presentation changed event identity", replay, err)
	}
	message.Title = "Changed event"
	if _, err = service.Publish(ctx, source, message); !errors.Is(err, notifications.ErrConflict) {
		t.Fatal("binding bypassed content conflict", err)
	}
	page, err := service.List(ctx, notifications.ListOptions{})
	if err != nil || len(page.Notifications) != 1 || page.UnreadCount != 0 {
		t.Fatal(page, err)
	}
}
