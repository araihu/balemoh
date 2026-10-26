package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/notifications"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
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

func publisherForSession(session *notificationSession) notifications.Publisher {
	if session == nil {
		return nil
	}
	return session.publisher
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
	if message.Destination != nil {
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
		result.Destination = &notifications.Destination{Kind: d.Kind, Route: d.Route, PluginID: d.PluginId, ConnectionID: d.ConnectionId, Page: d.Page, URL: d.Url}
		if len(d.Params) > 0 {
			result.Destination.Params = make(map[string]string, len(d.Params))
			for key, value := range d.Params {
				result.Destination.Params[key] = value
			}
		}
		if len(d.Query) > 0 {
			result.Destination.Query = make(map[string][]string, len(d.Query))
			for key, values := range d.Query {
				if values == nil {
					result.Destination.Query[key] = nil
					continue
				}
				result.Destination.Query[key] = append([]string(nil), values.Values...)
			}
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
	switch status.Code(err) {
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
		"deadline_exceeded":  codes.DeadlineExceeded,
	}[code]
	if grpcCode == codes.OK {
		grpcCode = codes.Unavailable
	}
	return status.Error(grpcCode, notifications.Fail(code).Message)
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

func (s *rpcServer) replaceNotificationSession(connectionID string, session *notificationSession) {
	s.notificationMu.Lock()
	if s.notificationSessions == nil {
		s.notificationSessions = map[string]*notificationSession{}
	}
	old := s.notificationSessions[connectionID]
	if session == nil {
		delete(s.notificationSessions, connectionID)
	} else {
		s.notificationSessions[connectionID] = session
	}
	s.notificationMu.Unlock()
	if old != nil {
		old.Close()
	}
}

func (s *rpcServer) closeNotificationSession(connectionID string) {
	s.notificationMu.Lock()
	old := s.notificationSessions[connectionID]
	delete(s.notificationSessions, connectionID)
	s.notificationMu.Unlock()
	if old != nil {
		old.Close()
	}
}

func (s *rpcServer) closeAllNotificationSessions() {
	s.notificationMu.Lock()
	old := s.notificationSessions
	s.notificationSessions = nil
	s.notificationMu.Unlock()
	for _, session := range old {
		session.Close()
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
