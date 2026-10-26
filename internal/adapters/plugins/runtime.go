package plugins

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	app "github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
)

type runtimeConnection struct {
	config preparedConnection
	record app.Connection
	status app.Status
	ready  bool
}
type runtimePlugin struct {
	pkg         *manifest.Package
	client      *pluginruntime.BackendClient
	connections map[string]*runtimeConnection
}
type Runtime struct {
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	plugins       map[string]*runtimePlugin
	wg            sync.WaitGroup
	once          sync.Once
	problems      []app.InstallationProblem
	notifications NotificationPublisher
}

func NewRuntime(ctx context.Context, d *Deployment, records []app.Connection, notificationPublishers ...NotificationPublisher) *Runtime {
	ctx, cancel := context.WithCancel(ctx)
	r := &Runtime{ctx: ctx, cancel: cancel, plugins: map[string]*runtimePlugin{}, problems: d.Problems()}
	if len(notificationPublishers) > 0 {
		r.notifications = notificationPublishers[0]
	}
	if d == nil {
		return r
	}
	for _, p := range d.installations {
		item := &runtimePlugin{pkg: p.pkg, connections: map[string]*runtimeConnection{}}
		for _, c := range p.connections {
			for _, record := range records {
				if record.Enabled && record.PluginID == p.pkg.Manifest.ID && record.Key == c.key {
					item.connections[record.ID] = &runtimeConnection{config: c, record: record, status: app.Status{State: "unavailable"}}
				}
			}
		}
		r.plugins[p.pkg.Manifest.ID] = item
		if (len(p.pkg.Manifest.Queries) > 0 || p.pkg.Manifest.Declares(notificationPublishCapability)) && len(item.connections) > 0 {
			r.wg.Add(1)
			go r.run(item)
		}
	}
	return r
}
func (r *Runtime) run(p *runtimePlugin) {
	defer r.wg.Done()
	var stop context.CancelFunc
	var done chan struct{}
	pluginruntime.Supervise(r.ctx, func(ctx context.Context) (*pluginruntime.BackendClient, error) {
		return pluginruntime.OpenBackend(ctx, p.pkg)
	}, func(child *pluginruntime.BackendClient) {
		ctx, cancel := context.WithCancel(r.ctx)
		stop = cancel
		done = make(chan struct{})
		go func() { defer close(done); r.session(ctx, p, child) }()
	}, func(child *pluginruntime.BackendClient) {
		if stop != nil {
			stop()
			<-done
		}
	}, pluginruntime.StateLogger(p.pkg.Manifest.ID, "backend"))
}
func (r *Runtime) session(ctx context.Context, p *runtimePlugin, child *pluginruntime.BackendClient) {
	r.mu.Lock()
	p.client = child
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		p.client = nil
		for _, c := range p.connections {
			c.ready = false
			c.status.State = "unavailable"
			c.status.Ready = false
		}
		r.mu.Unlock()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for id := range p.connections {
			_ = child.CloseConnection(ctx, id)
		}
	}()
	publishers := map[string]*connectionPublisher{}
	defer func() {
		for _, publisher := range publishers {
			publisher.revoke()
		}
	}()
	for id, c := range p.connections {
		if c.config.canPublishNotifications() && r.notifications != nil {
			publishers[id] = newConnectionPublisher(ctx, r.notifications, p.pkg, id)
		}
		attempted := time.Now().UTC()
		r.mu.Lock()
		c.status.LastConfigureAttemptAt = &attempted
		r.mu.Unlock()
		var publisherInterface notifications.Publisher
		if publishers[id] != nil {
			publisherInterface = publishers[id]
		}
		state, err := child.Configure(ctx, backend.Config{ConnectionID: id, Revision: c.record.Revision, Config: c.config.config, Secrets: c.config.secrets, Notifications: publisherInterface})
		if err == nil && (state.ConnectionID != id || state.AppliedRevision != c.record.Revision) {
			err = backend.Fail("invalid_response")
		}
		if err != nil {
			if publishers[id] != nil {
				publishers[id].revoke()
			}
			// A rejected replacement may still have an older valid observation.
			previous, previousErr := child.Status(ctx, id)
			r.mu.Lock()
			if previousErr == nil && previous.ConnectionID == id {
				c.status = coreStatus(previous)
				c.status.Ready = false
			}
			c.status.ConfigurationError = app.Fail(backend.Code(err))
			c.status.LastConfigureAttemptAt = &attempted
			r.mu.Unlock()
			continue
		}
		r.mu.Lock()
		c.ready = true
		c.status = coreStatus(state)
		c.status.LastConfigureAttemptAt = &attempted
		r.mu.Unlock()
	}

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		for id, c := range p.connections {
			state, err := child.Status(ctx, id)
			r.mu.Lock()
			if err == nil && state.ConnectionID == id {
				attempted, configError := c.status.LastConfigureAttemptAt, c.status.ConfigurationError
				c.status = coreStatus(state)
				c.status.Ready = c.ready && state.AppliedRevision == c.record.Revision
				c.status.LastConfigureAttemptAt, c.status.ConfigurationError = attempted, configError
			} else {
				c.status.Ready = false
				if c.ready {
					c.status.State = "unavailable"
				}
			}
			r.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (r *Runtime) Close() { r.once.Do(func() { r.cancel(); r.wg.Wait() }) }
func coreStatus(s backend.Status) app.Status {
	var failure *app.Failure
	if s.SourceError != nil {
		failure = app.Fail(s.SourceError.Code)
	}
	return app.Status{Ready: true, State: s.State, AppliedRevision: &s.AppliedRevision, ObservationRevision: &s.ObservationRevision, ObservedAt: s.ObservedAt, LastAttemptAt: s.LastAttemptAt, LastSuccessAt: s.LastSuccessAt, SourceError: failure}
}
func (r *Runtime) CachedStatus(plugin, id string) (app.Status, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p := r.plugins[plugin]
	if p == nil {
		return app.Status{State: "unavailable"}, []string{}
	}
	c := p.connections[id]
	if c == nil {
		return app.Status{State: "unavailable"}, []string{}
	}
	state := c.status
	if len(p.pkg.Manifest.Queries) == 0 {
		return app.Status{Ready: true, State: "unavailable", AppliedRevision: &c.record.Revision}, []string{}
	}
	if p.client == nil || p.client.Exited() {
		state.State = "unavailable"
		state.Ready = false
	}
	if !state.Ready {
		return state, []string{}
	}
	return state, append([]string{}, c.config.grants...)
}
func (r *Runtime) Query(ctx context.Context, plugin, id, operation string, input json.RawMessage) (app.Result, error) {
	r.mu.RLock()
	p := r.plugins[plugin]
	if p == nil {
		r.mu.RUnlock()
		return app.Result{}, app.Fail("not_found")
	}
	c := p.connections[id]
	if c == nil {
		r.mu.RUnlock()
		return app.Result{}, app.Fail("not_found")
	}
	q, declared := p.pkg.Manifest.Query(operation)
	granted := false
	for _, g := range c.config.grants {
		if g == operation {
			granted = true
		}
	}
	child, ready := p.client, c.ready
	r.mu.RUnlock()
	if !declared {
		return app.Result{}, app.Fail("not_found")
	}
	if !granted {
		return app.Result{}, app.Fail("permission_denied")
	}
	if !backend.Object(input, backend.MaxInput) || p.pkg.ValidateJSON(q.InputSchema, input) != nil {
		return app.Result{}, app.Fail("invalid_argument")
	}
	if child == nil || !ready || child.Exited() {
		return app.Result{}, app.Fail("unavailable")
	}
	result, err := child.Query(ctx, backend.Request{ConnectionID: id, OperationID: operation, Input: input, RequestID: app.RequestID(ctx)})
	if err != nil {
		return app.Result{}, app.Fail(backend.Code(err))
	}
	if result.Validate() != nil || p.pkg.ValidateJSON(q.OutputSchema, result.Data) != nil {
		return app.Result{}, app.Fail("invalid_response")
	}
	var sourceError *app.Failure
	if result.SourceError != nil {
		sourceError = app.Fail(result.SourceError.Code)
	}
	return app.Result{Data: result.Data, State: result.State, ObservationRevision: result.ObservationRevision, ObservedAt: result.ObservedAt, LastSuccessAt: result.LastSuccessAt, SourceError: sourceError}, nil
}

func (r *Runtime) InstallationProblems() []app.InstallationProblem {
	return append([]app.InstallationProblem{}, r.problems...)
}
