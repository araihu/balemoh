package runtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func ServeBackend(p *manifest.Package, h backend.Handler) {
	plugin.Serve(&plugin.ServeConfig{HandshakeConfig: handshake, Plugins: plugin.PluginSet{"backend": &wirePlugin{server: &rpcServer{pkg: p, backend: h}}}, GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
		return plugin.DefaultGRPCServer(append(opts, grpc.MaxRecvMsgSize(maxWireBytes), grpc.MaxSendMsgSize(maxWireBytes)))
	}, Logger: hclog.NewNullLogger()})
}

type BackendClient struct {
	process               *Client
	slots                 chan struct{}
	lifecycleCtx          context.Context
	lifecycleCancel       context.CancelFunc
	closeOnce             sync.Once
	notificationMu        sync.Mutex
	notificationCallbacks map[string]*hostNotificationCallback
	monitorDone           chan struct{}
}

func OpenBackend(ctx context.Context, p *manifest.Package) (*BackendClient, error) {
	lifecycleCtx, lifecycleCancel := context.WithCancel(ctx)
	c, err := open(lifecycleCtx, p, "backend")
	if err != nil {
		lifecycleCancel()
		return nil, err
	}
	client := &BackendClient{process: c, slots: make(chan struct{}, 4), lifecycleCtx: lifecycleCtx, lifecycleCancel: lifecycleCancel, notificationCallbacks: map[string]*hostNotificationCallback{}, monitorDone: make(chan struct{})}
	go client.monitorBackendExit()
	return client, nil
}
func (c *BackendClient) Close() {
	c.closeOnce.Do(func() {
		c.lifecycleCancel()
		close(c.monitorDone)
		c.revokeAllNotificationCallbacks()
		c.process.Close()
	})
}
func (c *BackendClient) Exited() bool {
	exited := c.process.Exited()
	if exited {
		c.revokeAllNotificationCallbacks()
	}
	return exited
}
func (c *BackendClient) PID() int { return c.process.PID() }
func (c *BackendClient) Configure(ctx context.Context, config backend.Config) (backend.Status, error) {
	return c.configure(ctx, config, config.Notifications)
}

// ConfigureWithNotifications configures a connection and grants its backend a
// connection-scoped Publisher. The callback outlives the Configure RPC and is
// revoked on replacement, CloseConnection, child exit or Close.
func (c *BackendClient) ConfigureWithNotifications(ctx context.Context, config backend.Config, publisher notifications.Publisher) (backend.Status, error) {
	config.Notifications = publisher
	return c.configure(ctx, config, publisher)
}
func (c *BackendClient) configure(ctx context.Context, config backend.Config, publisher notifications.Publisher) (backend.Status, error) {
	var callback *hostNotificationCallback
	if publisher != nil {
		var err error
		callback, err = c.startNotificationCallback(publisher)
		if err != nil {
			return backend.Status{}, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	brokerID := uint32(0)
	if callback != nil {
		brokerID = callback.id
	}
	r, err := c.process.wire.backend.Configure(ctx, &pb.ConnectionConfig{ConnectionId: config.ConnectionID, Revision: config.Revision, ConfigJson: config.Config, Secrets: config.Secrets, NotificationsBrokerId: brokerID})
	if err != nil {
		if callback != nil {
			callback.Stop()
		}
		return backend.Status{}, fromRPC(err)
	}
	statusValue, err := statusFromWire(r)
	if err != nil {
		if callback != nil {
			callback.Stop()
		}
		return backend.Status{}, err
	}
	c.replaceNotificationCallback(config.ConnectionID, callback)
	return statusValue, nil
}
func (c *BackendClient) Status(ctx context.Context, id string) (backend.Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	r, err := c.process.wire.backend.Status(ctx, &pb.ConnectionID{ConnectionId: id})
	if err != nil {
		return backend.Status{}, fromRPC(err)
	}
	return statusFromWire(r)
}
func (c *BackendClient) CloseConnection(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := c.process.wire.backend.CloseConnection(ctx, &pb.ConnectionID{ConnectionId: id})
	c.revokeNotificationCallback(id)
	if err != nil {
		return fromRPC(err)
	}
	return nil
}
func (c *BackendClient) Query(ctx context.Context, r backend.Request) (backend.Result, error) {
	started := time.Now()
	defer func() {
		slog.Debug("plugin query completed", "plugin", c.process.pkg.Manifest.ID, "operation", r.OperationID, "requestId", r.RequestID, "duration", time.Since(started))
	}()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	default:
		return backend.Result{}, backend.Fail("resource_exhausted")
	}
	if !backend.Object(r.Input, backend.MaxInput) {
		return backend.Result{}, backend.Fail("invalid_argument")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	value, err := c.process.wire.backend.Query(ctx, &pb.BackendQueryRequest{ConnectionId: r.ConnectionID, OperationId: r.OperationID, InputJson: r.Input, RequestId: r.RequestID})
	if err != nil {
		return backend.Result{}, fromRPC(err)
	}
	return resultFromWire(value)
}
func (s *rpcServer) Configure(ctx context.Context, r *pb.ConnectionConfig) (*pb.ConnectionStatus, error) {
	if proto.Size(r) > backend.MaxInput || r.ConnectionId == "" || !backend.Revision(r.Revision) || !backend.Object(r.ConfigJson, backend.MaxInput) || s.pkg.ValidateJSON(s.pkg.Manifest.ConnectionSchema, r.ConfigJson) != nil {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	declared := map[string]bool{}
	for _, secret := range s.pkg.Manifest.Secrets {
		declared[secret.ID] = true
		if secret.Required && r.Secrets[secret.ID] == "" {
			return nil, toRPC(backend.Fail("invalid_argument"))
		}
	}
	for id, value := range r.Secrets {
		if !declared[id] || len(value) > 4096 {
			return nil, toRPC(backend.Fail("invalid_argument"))
		}
	}
	if r.NotificationsBrokerId != 0 && !s.pkg.Manifest.Declares("notifications.publish/v1") {
		return nil, toRPC(backend.Fail("permission_denied"))
	}
	var session *notificationSession
	if r.NotificationsBrokerId != 0 {
		var err error
		session, err = s.openNotificationSession(r.NotificationsBrokerId)
		if err != nil {
			return nil, toRPC(err)
		}
	}
	result, err := s.backend.Configure(ctx, backend.Config{ConnectionID: r.ConnectionId, Revision: r.Revision, Config: r.ConfigJson, Secrets: r.Secrets, Notifications: publisherForSession(session)})
	if err != nil {
		if session != nil {
			session.Close()
		}
		return nil, toRPC(err)
	}
	s.replaceNotificationSession(r.ConnectionId, session)
	return statusToWire(result), nil
}
func (s *rpcServer) Status(ctx context.Context, r *pb.ConnectionID) (*pb.ConnectionStatus, error) {
	result, err := s.backend.Status(ctx, r.ConnectionId)
	if err != nil {
		return nil, toRPC(err)
	}
	return statusToWire(result), nil
}
func (s *rpcServer) CloseConnection(ctx context.Context, r *pb.ConnectionID) (*emptypb.Empty, error) {
	err := s.backend.CloseConnection(ctx, r.ConnectionId)
	s.closeNotificationSession(r.ConnectionId)
	if err != nil {
		return nil, toRPC(err)
	}
	return &emptypb.Empty{}, nil
}
func (s *rpcServer) Query(ctx context.Context, r *pb.BackendQueryRequest) (*pb.QueryResult, error) {
	q, ok := s.pkg.Manifest.Query(r.OperationId)
	if !ok || !backend.Object(r.InputJson, backend.MaxInput) || s.pkg.ValidateJSON(q.InputSchema, r.InputJson) != nil {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	result, err := s.backend.Query(ctx, backend.Request{ConnectionID: r.ConnectionId, OperationID: r.OperationId, Input: r.InputJson, RequestID: r.RequestId})
	if err != nil {
		return nil, toRPC(err)
	}
	if result.Validate() != nil || s.pkg.ValidateJSON(q.OutputSchema, result.Data) != nil {
		return nil, toRPC(backend.Fail("invalid_response"))
	}
	return resultToWire(result), nil
}
func toRPC(err error) error {
	code := backend.Code(err)
	m := map[string]codes.Code{"invalid_argument": codes.InvalidArgument, "not_found": codes.NotFound, "permission_denied": codes.PermissionDenied, "conflict": codes.Aborted, "resource_exhausted": codes.ResourceExhausted, "invalid_response": codes.DataLoss, "deadline_exceeded": codes.DeadlineExceeded}
	c, ok := m[code]
	if !ok {
		c = codes.Unavailable
	}
	return status.Error(c, backend.Fail(code).Message)
}
func fromRPC(err error) error {
	code := "unavailable"
	switch status.Code(err) {
	case codes.InvalidArgument:
		code = "invalid_argument"
	case codes.NotFound:
		code = "not_found"
	case codes.PermissionDenied:
		code = "permission_denied"
	case codes.Aborted:
		code = "conflict"
	case codes.ResourceExhausted:
		code = "resource_exhausted"
	case codes.DataLoss:
		code = "invalid_response"
	case codes.DeadlineExceeded:
		code = "deadline_exceeded"
	}
	return backend.Fail(code)
}
func timeWire(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil, backend.Fail("invalid_response")
	}
	return &t, nil
}
func errorWire(e *backend.Error) *pb.Failure {
	if e == nil {
		return nil
	}
	safe := backend.Fail(e.Code)
	return &pb.Failure{Code: safe.Code, Message: safe.Message, Retryable: safe.Retryable}
}
func errorFromWire(e *pb.Failure) *backend.Error {
	if e == nil {
		return nil
	}
	return backend.Fail(e.Code)
}
func resultToWire(r backend.Result) *pb.QueryResult {
	return &pb.QueryResult{DataJson: r.Data, State: r.State, ObservationRevision: r.ObservationRevision, ObservedAt: timeWire(r.ObservedAt), LastSuccessAt: timeWire(r.LastSuccessAt), SourceError: errorWire(r.SourceError)}
}
func resultFromWire(r *pb.QueryResult) (backend.Result, error) {
	at, err := parseTime(r.ObservedAt)
	if err != nil {
		return backend.Result{}, err
	}
	success, err := parseTime(r.LastSuccessAt)
	if err != nil {
		return backend.Result{}, err
	}
	result := backend.Result{Data: r.DataJson, State: r.State, ObservationRevision: r.ObservationRevision, ObservedAt: at, LastSuccessAt: success, SourceError: errorFromWire(r.SourceError)}
	return result, result.Validate()
}
func statusToWire(s backend.Status) *pb.ConnectionStatus {
	return &pb.ConnectionStatus{ConnectionId: s.ConnectionID, AppliedRevision: s.AppliedRevision, State: s.State, ObservationRevision: s.ObservationRevision, ObservedAt: timeWire(s.ObservedAt), LastAttemptAt: timeWire(s.LastAttemptAt), LastSuccessAt: timeWire(s.LastSuccessAt), SourceError: errorWire(s.SourceError)}
}
func statusFromWire(s *pb.ConnectionStatus) (backend.Status, error) {
	at, err := parseTime(s.ObservedAt)
	if err != nil {
		return backend.Status{}, err
	}
	attempt, err := parseTime(s.LastAttemptAt)
	if err != nil {
		return backend.Status{}, err
	}
	success, err := parseTime(s.LastSuccessAt)
	if err != nil {
		return backend.Status{}, err
	}
	if s.ConnectionId == "" || !backend.Revision(s.AppliedRevision) || !backend.Revision(s.ObservationRevision) || !backend.State(s.State) {
		return backend.Status{}, backend.Fail("invalid_response")
	}
	return backend.Status{ConnectionID: s.ConnectionId, AppliedRevision: s.AppliedRevision, State: s.State, ObservationRevision: s.ObservationRevision, ObservedAt: at, LastAttemptAt: attempt, LastSuccessAt: success, SourceError: errorFromWire(s.SourceError)}, nil
}

func (c *BackendClient) CheckHealth(ctx context.Context) error { return c.process.CheckHealth(ctx) }
