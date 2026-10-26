package plugins

import (
	"context"
	"errors"
	"sync"

	core "github.com/araihu/balemoh/internal/application/notifications"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	sdk "github.com/araihu/balemoh/plugin-sdk/notifications"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	pluginui "github.com/araihu/balemoh/plugin-sdk/ui"
)

const notificationPublishCapability = "notifications.publish/v1"

type NotificationPublisher interface {
	Publish(context.Context, core.Source, core.Message) (core.Receipt, error)
}

// The authority lock covers the commit acknowledgement. Revocation waits for
// earlier publications, then prevents every later call before closing the broker.
type connectionPublisher struct {
	mu        sync.RWMutex
	active    bool
	ctx       context.Context
	publisher NotificationPublisher
	source    core.Source
	pkg       *manifest.Package
}

func newConnectionPublisher(ctx context.Context, publisher NotificationPublisher, pkg *manifest.Package, connection string) *connectionPublisher {
	return &connectionPublisher{active: true, ctx: ctx, publisher: publisher, pkg: pkg, source: core.Source{Kind: core.SourcePlugin, ID: pkg.Manifest.ID, ConnectionID: connection}}
}
func (p *connectionPublisher) revoke() { p.mu.Lock(); p.active = false; p.mu.Unlock() }
func (p *connectionPublisher) Publish(ctx context.Context, message sdk.Message) (sdk.Receipt, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.active || p.ctx.Err() != nil || p.publisher == nil {
		return sdk.Receipt{}, sdk.Fail("permission_denied")
	}
	if err := message.Validate(); err != nil {
		return sdk.Receipt{}, err
	}
	m := core.Message{EventID: message.EventID, Severity: core.Severity(message.Severity), Title: message.Title, Body: message.Body, OccurredAt: message.OccurredAt}
	if d := message.Destination; d != nil {
		m.Destination = core.Destination{Kind: d.Kind, Route: d.Route, PluginID: d.PluginID, ConnectionID: d.ConnectionID, Page: d.Page, Params: d.Params, Query: d.Query, URL: d.URL}
		if d.Kind == core.SourcePlugin {
			if d.PluginID != p.source.ID || d.ConnectionID != p.source.ConnectionID {
				return sdk.Receipt{}, sdk.Fail("permission_denied")
			}
			if !pluginruntime.Available(p.pkg.Manifest, "page", d.Page) {
				return sdk.Receipt{}, sdk.Fail("invalid_argument")
			}
			if _, err := (pluginui.Request{ConnectionID: p.source.ConnectionID}).PageURL(p.pkg.Manifest, d.Page, d.Params); err != nil {
				return sdk.Receipt{}, sdk.Fail("invalid_argument")
			}
		}
	}
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(p.ctx, cancel)
	defer stop()
	receipt, err := p.publisher.Publish(callCtx, p.source, m)
	if err != nil {
		return sdk.Receipt{}, notificationFailure(err)
	}
	return sdk.Receipt{ID: receipt.ID, Created: receipt.Accepted}, nil
}
func notificationFailure(err error) error {
	switch {
	case errors.Is(err, core.ErrInvalid), errors.Is(err, core.ErrTooLarge):
		return sdk.Fail("invalid_argument")
	case errors.Is(err, core.ErrConflict):
		return sdk.Fail("conflict")
	case errors.Is(err, core.ErrCapacity):
		return sdk.Fail("capacity_exceeded")
	case errors.Is(err, context.DeadlineExceeded):
		return sdk.Fail("deadline_exceeded")
	default:
		return sdk.Fail("unavailable")
	}
}
func (c preparedConnection) canPublishNotifications() bool {
	for _, capability := range c.capabilityGrants {
		if capability == notificationPublishCapability {
			return true
		}
	}
	return false
}
