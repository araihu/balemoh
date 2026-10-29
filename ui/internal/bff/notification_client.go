package bff

import (
	"context"
	"net/http"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
)

func (c *APIClient) notificationAuthority() (api.RequestEditorFn, error) {
	if c.notificationToken == "" {
		return nil, &upstreamError{operation: "notifications", status: 401}
	}
	return func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "Bearer "+c.notificationToken)
		return nil
	}, nil
}
func (c *APIClient) Notifications(ctx context.Context, before string, unread bool, limit int) (view.NotificationsPage, error) {
	auth, err := c.notificationAuthority()
	if err != nil {
		return view.NotificationsPage{}, err
	}
	size := int32(limit)
	p := &api.GetNotificationsParams{Unread: &unread, Limit: &size}
	if before != "" {
		p.Before = &before
	}
	response, err := c.client.GetNotificationsWithResponse(ctx, p, auth)
	if err != nil {
		return view.NotificationsPage{}, &upstreamError{operation: "notifications", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return view.NotificationsPage{}, &upstreamError{operation: "notifications", status: response.StatusCode()}
	}
	page := view.NotificationsPage{UnreadCount: int(response.JSON200.UnreadCount), RetentionDays: int(response.JSON200.RetentionDays), NextCursor: notificationString(response.JSON200.NextCursor)}
	for _, item := range response.JSON200.Notifications {
		page.Items = append(page.Items, notificationView(item))
	}
	return page, nil
}
func (c *APIClient) Notification(ctx context.Context, id string) (NotificationRecord, error) {
	auth, err := c.notificationAuthority()
	if err != nil {
		return NotificationRecord{}, err
	}
	response, err := c.client.GetNotificationWithResponse(ctx, id, auth)
	if err != nil {
		return NotificationRecord{}, &upstreamError{operation: "notification", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return NotificationRecord{}, &upstreamError{operation: "notification", status: response.StatusCode()}
	}
	item := *response.JSON200
	record := NotificationRecord{Item: notificationView(item)}
	if d := item.Destination; d != nil {
		record.Target = &NotificationTarget{Kind: string(d.Kind), Route: notificationString(d.Route), URL: notificationString(d.Url), PluginID: notificationString(d.PluginId), ConnectionID: notificationString(d.ConnectionId), Page: notificationString(d.Page)}
		if d.Params != nil {
			record.Target.Params = *d.Params
		}
		if d.Query != nil {
			record.Target.Query = *d.Query
		}
	}
	return record, nil
}
func (c *APIClient) MarkNotificationRead(ctx context.Context, id string) error {
	auth, err := c.notificationAuthority()
	if err != nil {
		return err
	}
	response, err := c.client.MarkNotificationReadWithResponse(ctx, id, auth)
	if err != nil {
		return &upstreamError{operation: "notification read", cause: err}
	}
	if response.StatusCode() != http.StatusNoContent {
		return &upstreamError{operation: "notification read", status: response.StatusCode()}
	}
	return nil
}
func (c *APIClient) ClearNotifications(ctx context.Context) error {
	auth, err := c.notificationAuthority()
	if err != nil {
		return err
	}
	response, err := c.client.ClearNotificationsWithResponse(ctx, auth)
	if err != nil {
		return &upstreamError{operation: "clear notifications", cause: err}
	}
	if response.StatusCode() != http.StatusNoContent {
		return &upstreamError{operation: "clear notifications", status: response.StatusCode()}
	}
	return nil
}

func (c *APIClient) MarkAllNotificationsRead(ctx context.Context) error {
	auth, err := c.notificationAuthority()
	if err != nil {
		return err
	}
	response, err := c.client.MarkAllNotificationsReadWithResponse(ctx, auth)
	if err != nil {
		return &upstreamError{operation: "mark all notifications read", cause: err}
	}
	if response.StatusCode() != http.StatusNoContent {
		return &upstreamError{operation: "mark all notifications read", status: response.StatusCode()}
	}
	return nil
}

func notificationView(item api.Notification) view.Notification {
	source := string(item.Source.Kind) + " · " + item.Source.Id
	if item.Source.ConnectionId != nil {
		source += " · " + *item.Source.ConnectionId
	}
	itemView := view.Notification{ServiceID: notificationString(item.ServiceId), ProducerID: item.Source.Id, ProducerConnectionID: notificationString(item.Source.ConnectionId), SourceKind: string(item.Source.Kind), SourceLabel: item.Source.Id, ID: item.Id.String(), Title: item.Title, Body: item.Body, Severity: string(item.Severity), Source: source, ReceivedAt: item.ReceivedAt, Read: item.ReadAt != nil, HasDestination: item.Destination != nil, External: item.Destination != nil && string(item.Destination.Kind) == "external"}
	if d := item.Destination; d != nil && string(d.Kind) == "plugin" && notificationString(d.PluginId) == item.Source.Id && notificationString(d.ConnectionId) == itemView.ProducerConnectionID {
		itemView.ProducerPage = notificationString(d.Page)
	}
	return itemView
}
func notificationString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ NotificationCatalog = (*APIClient)(nil)
