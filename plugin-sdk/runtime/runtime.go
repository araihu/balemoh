// Package runtime implements local go-plugin/gRPC transport for trusted UI code.
package runtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

const maxWireBytes = ui.MaxHTML + (64 << 10)

var handshake = plugin.HandshakeConfig{ProtocolVersion: 1, MagicCookieKey: "BALEMOH_PLUGIN", MagicCookieValue: manifest.Protocol}

type wirePlugin struct {
	plugin.NetRPCUnsupportedPlugin
	server *rpcServer
}

func (p *wirePlugin) GRPCServer(b *plugin.GRPCBroker, s *grpc.Server) error {
	p.server.broker = b
	pb.RegisterControlServer(s, p.server)
	if p.server.backend != nil {
		pb.RegisterBackendServer(s, p.server)
	} else {
		pb.RegisterUIServer(s, p.server)
	}
	return nil
}
func (*wirePlugin) GRPCClient(_ context.Context, b *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &wireClient{control: pb.NewControlClient(c), ui: pb.NewUIClient(c), backend: pb.NewBackendClient(c), broker: b}, nil
}

type wireClient struct {
	control pb.ControlClient
	ui      pb.UIClient
	backend pb.BackendClient
	broker  *plugin.GRPCBroker
}
type rpcServer struct {
	pb.UnimplementedControlServer
	pb.UnimplementedUIServer
	pb.UnimplementedBackendServer
	pkg     *manifest.Package
	handler ui.Handler
	backend backend.Handler
	broker  *plugin.GRPCBroker
}

func (s *rpcServer) Describe(context.Context, *emptypb.Empty) (*pb.Description, error) {
	m := s.pkg.Manifest
	return &pb.Description{PluginId: m.ID, Version: m.Version, ManifestDigest: s.pkg.Digest, Protocol: m.Protocol, UiProfile: m.UIProfile}, nil
}
func (*rpcServer) CheckHealth(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*rpcServer) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (s *rpcServer) Handle(ctx context.Context, r *pb.UIRequest) (*pb.UIResponse, error) {
	if proto.Size(r) > 64<<10 || validateRequest(s.pkg.Manifest, fromWire(r)) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid UI invocation")
	}
	if err := validateContribution(s.pkg.Manifest, r.ContributionType, r.ContributionId, r.Mode); err != nil {
		return nil, status.Error(codes.InvalidArgument, "unknown UI contribution")
	}
	request := fromWire(r)
	var services ui.HostServices = ui.NoDataServices{Current: ui.Placement{ID: r.PlacementId, ContributionID: r.ContributionId, Options: []byte("{}"), Revision: "1"}}
	if r.HostBrokerId != 0 {
		conn, err := s.broker.DialWithOptions(r.HostBrokerId, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxWireBytes), grpc.MaxCallSendMsgSize(maxWireBytes)))
		if err != nil {
			return nil, status.Error(codes.Unavailable, "host callback unavailable")
		}
		defer conn.Close()
		services = &hostClient{client: pb.NewHostClient(conn)}
	}
	response, err := s.handler.Handle(ctx, request, services)
	if err != nil {
		return nil, status.Error(codes.Internal, "plugin could not render content")
	}
	if err := validateUIResponse(s.pkg, response, request); err != nil {
		return nil, status.Error(codes.Internal, "invalid plugin UI response")
	}
	return &pb.UIResponse{Kind: response.Kind, Status: int32(response.Status), Title: response.Title, Description: response.Description, Html: response.HTML, AssetIds: response.AssetIDs, Redirect: redirectToWire(response.Redirect)}, nil
}

// Serve runs a UI-only P1 child. The manifest is read before calling Serve.
func Serve(p *manifest.Package, h ui.Handler) {
	plugin.Serve(&plugin.ServeConfig{HandshakeConfig: handshake, Plugins: plugin.PluginSet{"ui": &wirePlugin{server: &rpcServer{pkg: p, handler: h}}}, GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
		return plugin.DefaultGRPCServer(append(opts, grpc.MaxRecvMsgSize(maxWireBytes), grpc.MaxSendMsgSize(maxWireBytes)))
	}, Logger: hclog.NewNullLogger()})
}

type Client struct {
	client *plugin.Client
	wire   *wireClient
	pkg    *manifest.Package
	once   sync.Once
	slots  chan struct{}
	cancel context.CancelFunc
}

func Open(ctx context.Context, p *manifest.Package) (*Client, error) {
	return open(ctx, p, "ui")
}
func open(ctx context.Context, p *manifest.Package, role string) (*Client, error) {
	if err := Supported(p.Manifest); err != nil {
		return nil, err
	}
	executable, err := p.ExecutableFor(role)
	if err != nil {
		return nil, err
	}
	childCtx, stopChild := context.WithCancel(ctx)
	startupDeadline := time.Now().Add(10 * time.Second)
	timer := time.AfterFunc(10*time.Second, stopChild)
	defer timer.Stop()
	command := exec.CommandContext(childCtx, executable)
	command.Dir = p.Root
	command.Env = []string{}
	process := plugin.NewClient(&plugin.ClientConfig{HandshakeConfig: handshake, Plugins: plugin.PluginSet{role: &wirePlugin{}}, Cmd: command, AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC}, StartTimeout: 10 * time.Second, SkipHostEnv: true, Logger: hclog.NewNullLogger(), Stderr: io.Discard, SyncStderr: io.Discard, SyncStdout: io.Discard, AutoMTLS: true, GRPCDialOptions: []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxWireBytes), grpc.MaxCallSendMsgSize(maxWireBytes))}})
	failed := true
	defer func() {
		if failed {
			stopChild()
			process.Kill()
		}
	}()
	protocol, err := process.Client()
	if err != nil {
		return nil, errors.New("plugin startup failed")
	}
	dispensed, err := protocol.Dispense(role)
	if err != nil {
		return nil, errors.New("plugin UI protocol unavailable")
	}
	wire := dispensed.(*wireClient)
	checkCtx, cancel := context.WithDeadline(childCtx, startupDeadline)
	defer cancel()
	d, err := wire.control.Describe(checkCtx, &emptypb.Empty{})
	if err != nil {
		return nil, errors.New("plugin identity unavailable")
	}
	m := p.Manifest
	if d.PluginId != m.ID || d.Version != m.Version || d.ManifestDigest != p.Digest || d.Protocol != m.Protocol || d.UiProfile != m.UIProfile {
		return nil, errors.New("plugin identity does not match installation")
	}
	if !timer.Stop() || childCtx.Err() != nil {
		return nil, errors.New("plugin startup deadline exceeded")
	}
	failed = false
	return &Client{client: process, wire: wire, pkg: p, slots: make(chan struct{}, 8), cancel: stopChild}, nil
}

func (c *Client) Close() {
	c.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = c.wire.control.Shutdown(ctx, &emptypb.Empty{})
		if c.cancel != nil {
			c.cancel()
		}
		c.client.Kill()
	})
}
func (c *Client) Exited() bool { return c.client.Exited() }
func (c *Client) Handle(ctx context.Context, r ui.Request) (ui.Response, error) {
	return c.HandleWithServices(ctx, r, nil)
}
func (c *Client) HandleWithServices(ctx context.Context, r ui.Request, services ui.HostServices) (ui.Response, error) {
	started := time.Now()
	defer func() {
		slog.Debug("plugin render completed", "plugin", c.pkg.Manifest.ID, "contribution", r.ContributionID, "requestId", r.RequestID, "duration", time.Since(started))
	}()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	default:
		return ui.Response{}, errors.New("plugin UI capacity exhausted")
	}
	if err := validateRequest(c.pkg.Manifest, r); err != nil {
		return ui.Response{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	w := toWire(r)
	if services != nil {
		id, stop, err := c.callback(ctx, services)
		if err != nil {
			return ui.Response{}, err
		}
		defer stop()
		w.HostBrokerId = id
	}
	if proto.Size(w) > 64<<10 {
		return ui.Response{}, errors.New("UI request exceeds limit")
	}
	response, err := c.wire.ui.Handle(ctx, w)
	if err != nil {
		return ui.Response{}, errors.New("plugin UI is unavailable")
	}
	v := ui.Response{Kind: response.Kind, Status: int(response.Status), Title: response.Title, Description: response.Description, HTML: response.Html, AssetIDs: response.AssetIds, Redirect: redirectFromWire(response.Redirect)}
	if err := validateUIResponse(c.pkg, v, r); err != nil {
		return ui.Response{}, err
	}
	return v, nil
}

func Supported(m manifest.Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	for _, cap := range m.Capabilities.Required {
		if !supportedCapability(cap) {
			return errors.New("plugin requires an unavailable capability")
		}
	}

	return nil
}
func validateContribution(m manifest.Manifest, kind, id, mode string) error {
	var caps []string
	var extension string
	switch kind {
	case "page":
		extension = "page.content/v1"
		p, ok := m.Page(id)
		if !ok || (mode != "page" && mode != "fragment") {
			return errors.New("unknown page")
		}
		caps = p.Capabilities
	case "widget":
		extension = "home.widget/v1"
		w, ok := m.Widget(id)
		if !ok || mode != "widget" {
			return errors.New("unknown widget")
		}
		caps = w.Capabilities
	default:
		return errors.New("unknown contribution type")
	}
	if !m.Declares(extension) {
		return errors.New("undeclared extension point")
	}
	for _, c := range caps {
		if !m.Declares(c) || !supportedCapability(c) {
			return errors.New("contribution requires an unavailable capability")
		}
	}
	return nil
}
func supportedCapability(c string) bool {
	return c == "page.content/v1" || c == "sidebar.navigation/v1" || c == "home.widget/v1" || c == "data.query/v1"
}
func toWire(r ui.Request) *pb.UIRequest {
	q := map[string]*pb.Values{}
	for k, v := range r.Query {
		q[k] = &pb.Values{Values: v}
	}
	form := map[string]*pb.Values{}
	for k, v := range r.Form {
		form[k] = &pb.Values{Values: v}
	}
	return &pb.UIRequest{Form: form, Body: r.Body, Region: r.Region, PackageDigest: r.PackageDigest, RequestId: r.RequestID, ConnectionId: r.ConnectionID, PlacementId: r.PlacementID, ContributionType: r.ContributionType, ContributionId: r.ContributionID, Mode: r.Mode, Method: r.Method, PathParams: r.PathParams, Query: q, Locale: r.Locale, Timezone: r.Timezone, Theme: r.Theme, ColorScheme: r.ColorScheme, BasePath: r.BasePath, DomNamespace: r.DOMNamespace}
}
func fromWire(r *pb.UIRequest) ui.Request {
	q := map[string][]string{}
	for k, v := range r.Query {
		q[k] = v.Values
	}
	form := map[string][]string{}
	for k, v := range r.Form {
		form[k] = v.Values
	}
	return ui.Request{Form: form, Body: r.Body, Region: r.Region, PackageDigest: r.PackageDigest, RequestID: r.RequestId, ConnectionID: r.ConnectionId, PlacementID: r.PlacementId, ContributionType: r.ContributionType, ContributionID: r.ContributionId, Mode: r.Mode, Method: r.Method, PathParams: r.PathParams, Query: q, Locale: r.Locale, Timezone: r.Timezone, Theme: r.Theme, ColorScheme: r.ColorScheme, BasePath: r.BasePath, DOMNamespace: r.DomNamespace}
}

// PID identifies this runtime-owned child for lifecycle diagnostics.
func (c *Client) PID() int { return c.client.ReattachConfig().Pid }

// Available reports whether this P1 host implements a contribution capability.
func Available(m manifest.Manifest, kind, id string) bool {
	return Supported(m) == nil && validateContribution(m, kind, id, kind) == nil
}

func redirectToWire(v *ui.Redirect) *pb.Redirect {
	if v == nil {
		return nil
	}
	q := map[string]*pb.Values{}
	for k, x := range v.Query {
		q[k] = &pb.Values{Values: x}
	}
	return &pb.Redirect{Page: v.Page, Params: v.Params, Query: q}
}
func redirectFromWire(v *pb.Redirect) *ui.Redirect {
	if v == nil {
		return nil
	}
	q := map[string][]string{}
	for k, x := range v.Query {
		q[k] = x.Values
	}
	return &ui.Redirect{Page: v.Page, Params: v.Params, Query: q}
}
func validateUIResponse(p *manifest.Package, v ui.Response, r ui.Request) error {
	if e := ui.ValidateResponse(v, r.Mode); e != nil {
		return e
	}
	if v.Redirect != nil {
		if _, e := r.PageURL(p.Manifest, v.Redirect.Page, v.Redirect.Params); e != nil {
			return e
		}
	}
	seen := map[string]bool{}
	for _, id := range v.AssetIDs {
		found := false
		for _, a := range p.Manifest.Assets {
			if a.ID == id {
				found = true
			}
		}
		if !found || seen[id] {
			return errors.New("invalid response assets")
		}
		seen[id] = true
	}
	return nil
}

func (c *Client) CheckHealth(ctx context.Context) error {
	_, err := c.wire.control.CheckHealth(ctx, &emptypb.Empty{})
	return err
}

func validateRequest(m manifest.Manifest, r ui.Request) error {
	if err := validateContribution(m, r.ContributionType, r.ContributionID, r.Mode); err != nil {
		return err
	}
	if r.ContributionType == "widget" {
		if r.Method != "GET" {
			return errors.New("invalid widget method")
		}
	} else {
		p, _ := m.Page(r.ContributionID)
		allowed := false
		for _, v := range p.Methods {
			if v == r.Method {
				allowed = true
			}
		}
		if !allowed {
			return errors.New("undeclared method")
		}
	}
	if r.Mode == "fragment" && (r.Region == "" || len(r.Region) > 64 || strings.ContainsAny(r.Region, "/\\%.")) {
		return errors.New("invalid fragment region")
	}
	return nil
}
