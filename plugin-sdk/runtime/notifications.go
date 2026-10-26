package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// notificationSession is owned by a backend child. Closing its connection
// revokes the child-held Publisher without depending on the Configure RPC
// context.
type notificationSession struct {
	conn      *grpc.ClientConn
	publisher notifications.Publisher
	once      sync.Once
}

// rebindableNotificationPublisher is the author-visible publisher for one
// accepted connection configuration. A repeated successful configuration with
// the same revision rebases this object onto the replacement host
// callback, so a handler that retained the original interface survives a lost
// Configure response. The read lock spans the callback RPC; revocation and
// rebinding therefore cannot close the old session underneath a publication.
type rebindableNotificationPublisher struct {
	mu        sync.RWMutex
	publisher notifications.Publisher
	revoked   bool
}

func (p *rebindableNotificationPublisher) Publish(ctx context.Context, message notifications.Message) (notifications.Receipt, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.revoked || p.publisher == nil {
		return notifications.Receipt{}, notifications.Fail("permission_denied")
	}
	return p.publisher.Publish(ctx, message)
}

func (p *rebindableNotificationPublisher) rebind(publisher notifications.Publisher) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.revoked || publisher == nil {
		return false
	}
	p.publisher = publisher
	return true
}

func (p *rebindableNotificationPublisher) revoke() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.revoked = true
	p.publisher = nil
}

type notificationBinding struct {
	revision  string
	session   *notificationSession
	publisher *rebindableNotificationPublisher
}

func newNotificationBinding(revision string, session *notificationSession) *notificationBinding {
	binding := &notificationBinding{revision: revision, session: session}
	if session != nil {
		binding.publisher = &rebindableNotificationPublisher{publisher: session.publisher}
	}
	return binding
}

func (b *notificationBinding) sameConfiguration(revision string, hasPublisher bool) bool {
	return b != nil && b.revision == revision && (b.session != nil) == hasPublisher
}

func (b *notificationBinding) authorPublisher() notifications.Publisher {
	if b == nil || b.publisher == nil {
		return nil
	}
	return b.publisher
}

func (b *notificationBinding) revokeAndClose() {
	if b == nil {
		return
	}
	if b.publisher != nil {
		b.publisher.revoke()
	}
	if b.session != nil {
		b.session.Close()
	}
}

func (s *notificationSession) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		_ = s.conn.Close()
	})
}

type notificationPublisher struct {
	client pb.BackendHostClient
}

func (p *notificationPublisher) Publish(ctx context.Context, message notifications.Message) (notifications.Receipt, error) {
	request, err := notificationRequest(message)
	if err != nil {
		return notifications.Receipt{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, notifications.PublishTimeout)
	defer cancel()
	response, err := p.client.Publish(ctx, request)
	if err != nil {
		return notifications.Receipt{}, fromNotificationRPC(err)
	}
	return notificationReceipt(response)
}

func notificationRequest(message notifications.Message) (*pb.NotificationPublishRequest, error) {
	if err := message.Validate(); err != nil {
		return nil, err
	}
	wire := &pb.NotificationMessage{
		EventId:    message.EventID,
		Severity:   message.Severity,
		Title:      message.Title,
		Body:       message.Body,
		OccurredAt: message.OccurredAt.UTC().Format(time.RFC3339Nano),
	}
	if message.Destination != nil && !message.Destination.IsZero() {
		d := message.Destination
		wire.Destination = &pb.NotificationDestination{Kind: d.Kind, Route: d.Route, PluginId: d.PluginID, ConnectionId: d.ConnectionID, Page: d.Page, Url: d.URL}
		if len(d.Params) > 0 {
			wire.Destination.Params = make(map[string]string, len(d.Params))
			for key, value := range d.Params {
				wire.Destination.Params[key] = value
			}
		}
		if len(d.Query) > 0 {
			wire.Destination.Query = make(map[string]*pb.Values, len(d.Query))
			for key, values := range d.Query {
				wire.Destination.Query[key] = &pb.Values{Values: append([]string(nil), values...)}
			}
		}
	}
	request := &pb.NotificationPublishRequest{Message: wire}
	if proto.Size(request) > notifications.MaxEnvelope {
		return nil, notifications.Fail("invalid_argument")
	}
	return request, nil
}

func notificationMessage(message *pb.NotificationMessage) (notifications.Message, error) {
	if message == nil {
		return notifications.Message{}, notifications.Fail("invalid_argument")
	}
	occurred, err := time.Parse(time.RFC3339Nano, message.OccurredAt)
	if err != nil {
		return notifications.Message{}, notifications.Fail("invalid_argument")
	}
	result := notifications.Message{EventID: message.EventId, Severity: message.Severity, Title: message.Title, Body: message.Body, OccurredAt: occurred}
	if d := message.Destination; d != nil {
		destination := notifications.Destination{Kind: d.Kind, Route: d.Route, PluginID: d.PluginId, ConnectionID: d.ConnectionId, Page: d.Page, URL: d.Url}
		if len(d.Params) > 0 {
			destination.Params = make(map[string]string, len(d.Params))
			for key, value := range d.Params {
				destination.Params[key] = value
			}
		}
		if len(d.Query) > 0 {
			destination.Query = make(map[string][]string, len(d.Query))
			for key, values := range d.Query {
				if values == nil {
					destination.Query[key] = nil
					continue
				}
				destination.Query[key] = append([]string(nil), values.Values...)
			}
		}
		if !destination.IsZero() {
			result.Destination = &destination
		}
	}
	if err := result.Validate(); err != nil {
		return notifications.Message{}, err
	}
	return result, nil
}

func notificationReceipt(receipt *pb.NotificationReceipt) (notifications.Receipt, error) {
	if receipt == nil || receipt.Id == "" {
		return notifications.Receipt{}, notifications.Fail("unavailable")
	}
	return notifications.Receipt{ID: receipt.Id, Created: receipt.Created}, nil
}

func fromNotificationRPC(err error) error {
	code := "unavailable"
	statusCode := status.Code(err)
	switch statusCode {
	case codes.InvalidArgument:
		code = "invalid_argument"
	case codes.PermissionDenied, codes.Unauthenticated:
		code = "permission_denied"
	case codes.Aborted:
		code = "conflict"
	case codes.ResourceExhausted:
		code = "resource_exhausted"
	case codes.DeadlineExceeded:
		code = "deadline_exceeded"
	case codes.Canceled:
		code = "unavailable"
	}
	if statusCode == codes.ResourceExhausted && notificationErrorInfoReason(err) == notifications.ErrorInfoReasonCapacity {
		code = "capacity_exceeded"
	}
	return notifications.Fail(code)
}

func toNotificationRPC(err error) error {
	code := notifications.Code(err)
	// Core adapters may use the SDK's backend-safe error type. Preserve only
	// its known code; all other errors remain sanitized unavailable failures.
	if code == "unavailable" {
		if backendCode := backend.Code(err); backendCode != "unavailable" {
			code = backendCode
		}
	}
	grpcCode := map[string]codes.Code{
		"invalid_argument":   codes.InvalidArgument,
		"permission_denied":  codes.PermissionDenied,
		"conflict":           codes.Aborted,
		"resource_exhausted": codes.ResourceExhausted,
		"capacity_exceeded":  codes.ResourceExhausted,
		"deadline_exceeded":  codes.DeadlineExceeded,
	}[code]
	if grpcCode == codes.OK {
		grpcCode = codes.Unavailable
	}
	result := status.New(grpcCode, notifications.Fail(code).Message)
	result, detailsErr := result.WithDetails(&errdetails.ErrorInfo{Domain: notifications.ErrorInfoDomain, Reason: notificationErrorInfoReasonForCode(code)})
	if detailsErr != nil {
		return status.Error(grpcCode, notifications.Fail(code).Message)
	}
	return result.Err()
}

func notificationErrorInfoReason(err error) string {
	result, ok := status.FromError(err)
	if !ok {
		return ""
	}
	for _, detail := range result.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if ok && info.Domain == notifications.ErrorInfoDomain {
			return info.Reason
		}
	}
	return ""
}

func notificationErrorInfoReasonForCode(code string) string {
	switch code {
	case "resource_exhausted":
		return notifications.ErrorInfoReasonBusy
	case "capacity_exceeded":
		return notifications.ErrorInfoReasonCapacity
	default:
		return notifications.ErrorInfoReasonUnavailable
	}
}

// backendHostServer is registered only by the host's backend connection
// callback. It never receives source identity from the child.
type backendHostServer struct {
	pb.UnimplementedBackendHostServer
	ctx       context.Context
	publisher notifications.Publisher
	slots     chan struct{}
}

func (h *backendHostServer) Publish(ctx context.Context, request *pb.NotificationPublishRequest) (*pb.NotificationReceipt, error) {
	if h.publisher == nil {
		return nil, toNotificationRPC(notifications.Fail("permission_denied"))
	}
	if h.ctx.Err() != nil {
		return nil, toNotificationRPC(notifications.Fail("unavailable"))
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return nil, toNotificationRPC(notifications.Fail("resource_exhausted"))
	}
	if request == nil || proto.Size(request) > notifications.MaxEnvelope {
		return nil, toNotificationRPC(notifications.Fail("invalid_argument"))
	}
	message, err := notificationMessage(request.Message)
	if err != nil {
		return nil, toNotificationRPC(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, notifications.PublishTimeout)
	defer cancel()
	stop := context.AfterFunc(h.ctx, cancel)
	defer stop()
	receipt, err := h.publisher.Publish(callCtx, message)
	if err != nil {
		return nil, toNotificationRPC(err)
	}
	if receipt.ID == "" {
		return nil, toNotificationRPC(notifications.Fail("unavailable"))
	}
	return &pb.NotificationReceipt{Id: receipt.ID, Created: receipt.Created}, nil
}

func (s *rpcServer) openNotificationSession(brokerID uint32) (*notificationSession, error) {
	if s.broker == nil || brokerID == 0 {
		return nil, notifications.Fail("unavailable")
	}
	conn, err := s.broker.DialWithOptions(brokerID, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxWireBytes), grpc.MaxCallSendMsgSize(maxWireBytes)))
	if err != nil {
		return nil, notifications.Fail("unavailable")
	}
	return &notificationSession{conn: conn, publisher: &notificationPublisher{client: pb.NewBackendHostClient(conn)}}, nil
}

func (s *rpcServer) replaceNotificationBinding(connectionID string, binding *notificationBinding) *notificationBinding {
	s.notificationMu.Lock()
	if s.notificationBindings == nil {
		s.notificationBindings = map[string]*notificationBinding{}
	}
	old := s.notificationBindings[connectionID]
	if binding == nil {
		delete(s.notificationBindings, connectionID)
	} else {
		s.notificationBindings[connectionID] = binding
	}
	s.notificationMu.Unlock()
	return old
}

func (s *rpcServer) closeNotificationBinding(connectionID string) {
	s.notificationMu.Lock()
	old := s.notificationBindings[connectionID]
	delete(s.notificationBindings, connectionID)
	s.notificationMu.Unlock()
	old.revokeAndClose()
}

func (s *rpcServer) closeAllNotificationSessions() {
	s.notificationMu.Lock()
	old := s.notificationBindings
	s.notificationBindings = nil
	s.notificationMu.Unlock()
	for _, binding := range old {
		binding.revokeAndClose()
	}
}

type hostNotificationCallback struct {
	id     uint32
	server *grpc.Server
	done   chan struct{}
	cancel context.CancelFunc
	once   sync.Once
}

func (c *hostNotificationCallback) Stop() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		c.cancel()
		if c.server != nil {
			c.server.Stop()
		}
		<-c.done
	})
}

func (c *BackendClient) startNotificationCallback(publisher notifications.Publisher) (*hostNotificationCallback, error) {
	if publisher == nil || !c.process.supportsNotifications {
		return nil, notifications.Fail("permission_denied")
	}
	if c.lifecycleCtx.Err() != nil || c.process.Exited() {
		return nil, notifications.Fail("unavailable")
	}
	ctx, cancel := context.WithCancel(c.lifecycleCtx)
	id := c.process.wire.broker.NextId()
	ready := make(chan *grpc.Server, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.process.wire.broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
			server := grpc.NewServer(append(opts, grpc.MaxRecvMsgSize(maxWireBytes), grpc.MaxSendMsgSize(maxWireBytes))...)
			pb.RegisterBackendHostServer(server, &backendHostServer{ctx: ctx, publisher: publisher, slots: make(chan struct{}, notifications.MaxConcurrent)})
			ready <- server
			return server
		})
	}()
	select {
	case server := <-ready:
		return &hostNotificationCallback{id: id, server: server, done: done, cancel: cancel}, nil
	case <-done:
		cancel()
		return nil, notifications.Fail("unavailable")
	case <-c.lifecycleCtx.Done():
		cancel()
		go func() {
			select {
			case server := <-ready:
				server.Stop()
			case <-done:
			}
		}()
		return nil, notifications.Fail("unavailable")
	}
}

func (c *BackendClient) replaceNotificationCallback(connectionID string, callback *hostNotificationCallback) {
	c.notificationMu.Lock()
	old := c.notificationCallbacks[connectionID]
	if callback == nil {
		delete(c.notificationCallbacks, connectionID)
	} else {
		c.notificationCallbacks[connectionID] = callback
	}
	c.notificationMu.Unlock()
	if old != nil {
		old.Stop()
	}
}

func (c *BackendClient) revokeNotificationCallback(connectionID string) {
	c.replaceNotificationCallback(connectionID, nil)
}

func (c *BackendClient) revokeAllNotificationCallbacks() {
	c.notificationMu.Lock()
	old := c.notificationCallbacks
	c.notificationCallbacks = map[string]*hostNotificationCallback{}
	c.notificationMu.Unlock()
	for _, callback := range old {
		callback.Stop()
	}
}

func (c *BackendClient) monitorBackendExit() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.monitorDone:
			return
		case <-ticker.C:
			if c.process.Exited() {
				c.revokeAllNotificationCallbacks()
				return
			}
		}
	}
}
