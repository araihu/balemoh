package runtime

import (
	"context"
	"encoding/json"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	pb "github.com/araihu/balemoh/plugin-sdk/protocol/v1alpha1"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type hostClient struct{ client pb.HostClient }

func (h *hostClient) ResolveServices(ctx context.Context, ids []string) ([]ui.ServiceIdentity, error) {
	if ui.ValidateServiceIDs(ids) != nil {
		return nil, backend.Fail("invalid_argument")
	}
	r, err := h.client.ResolveServices(ctx, &pb.ServiceIDs{Ids: ids})
	if err != nil {
		return nil, fromRPC(err)
	}
	services := make([]ui.ServiceIdentity, 0, len(r.Services))
	for _, s := range r.Services {
		if s == nil {
			return nil, backend.Fail("invalid_response")
		}
		services = append(services, ui.ServiceIdentity{ID: s.Id, Name: s.Name, IconURL: s.IconUrl, IconSpriteURL: s.IconSpriteUrl, IconSymbol: s.IconSymbol})
	}
	if ui.ValidateServiceIdentities(ids, services) != nil {
		return nil, backend.Fail("invalid_response")
	}
	return services, nil
}

func (h *hostClient) Query(ctx context.Context, operation string, input json.RawMessage) (ui.QueryResult, error) {
	if !backend.Object(input, backend.MaxInput) {
		return ui.QueryResult{}, backend.Fail("invalid_argument")
	}
	r, err := h.client.Query(ctx, &pb.QueryRequest{OperationId: operation, InputJson: input})
	if err != nil {
		return ui.QueryResult{}, fromRPC(err)
	}
	return resultFromWire(r)
}
func (h *hostClient) Placement(ctx context.Context) (ui.Placement, error) {
	r, err := h.client.Placement(ctx, &emptypb.Empty{})
	if err != nil {
		return ui.Placement{}, fromRPC(err)
	}
	if !backend.Object(r.OptionsJson, backend.MaxInput) {
		return ui.Placement{}, backend.Fail("invalid_response")
	}
	return ui.Placement{ID: r.PlacementId, ContributionID: r.ContributionId, Options: r.OptionsJson, Revision: r.Revision}, nil
}

type hostServer struct {
	pb.UnimplementedHostServer
	ctx      context.Context
	services ui.HostServices
	slots    chan struct{}
}

func (h *hostServer) ResolveServices(ctx context.Context, r *pb.ServiceIDs) (*pb.ServiceIdentities, error) {
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return nil, toRPC(backend.Fail("resource_exhausted"))
	}
	if h.ctx.Err() != nil {
		return nil, toRPC(h.ctx.Err())
	}
	if ui.ValidateServiceIDs(r.Ids) != nil {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	catalog, ok := h.services.(ui.ServiceCatalog)
	if !ok {
		return nil, toRPC(backend.Fail("permission_denied"))
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(h.ctx, cancel)
	defer stop()
	services, err := catalog.ResolveServices(ctx, r.Ids)
	if err != nil {
		return nil, toRPC(err)
	}
	if ui.ValidateServiceIdentities(r.Ids, services) != nil {
		return nil, toRPC(backend.Fail("invalid_response"))
	}
	result := &pb.ServiceIdentities{}
	for _, s := range services {
		result.Services = append(result.Services, &pb.ServiceIdentity{Id: s.ID, Name: s.Name, IconUrl: s.IconURL, IconSpriteUrl: s.IconSpriteURL, IconSymbol: s.IconSymbol})
	}
	return result, nil
}

func (h *hostServer) Query(ctx context.Context, r *pb.QueryRequest) (*pb.QueryResult, error) {
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return nil, toRPC(backend.Fail("resource_exhausted"))
	}
	if h.ctx.Err() != nil {
		return nil, toRPC(h.ctx.Err())
	}
	if !backend.Object(r.InputJson, backend.MaxInput) {
		return nil, toRPC(backend.Fail("invalid_argument"))
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(h.ctx, cancel)
	defer stop()
	result, err := h.services.Query(ctx, r.OperationId, r.InputJson)
	if err != nil {
		return nil, toRPC(err)
	}
	if result.Validate() != nil {
		return nil, toRPC(backend.Fail("invalid_response"))
	}
	return resultToWire(result), nil
}
func (h *hostServer) Placement(ctx context.Context, _ *emptypb.Empty) (*pb.PlacementResult, error) {
	if h.ctx.Err() != nil {
		return nil, toRPC(h.ctx.Err())
	}
	r, err := h.services.Placement(ctx)
	if err != nil {
		return nil, toRPC(err)
	}
	if !backend.Object(r.Options, backend.MaxInput) {
		return nil, toRPC(backend.Fail("invalid_response"))
	}
	return &pb.PlacementResult{PlacementId: r.ID, ContributionId: r.ContributionID, OptionsJson: r.Options, Revision: r.Revision}, nil
}
func (c *Client) callback(ctx context.Context, services ui.HostServices) (uint32, func(), error) {
	id := c.wire.broker.NextId()
	ready := make(chan *grpc.Server, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.wire.broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
			server := grpc.NewServer(append(opts, grpc.MaxRecvMsgSize(maxWireBytes), grpc.MaxSendMsgSize(maxWireBytes))...)
			pb.RegisterHostServer(server, &hostServer{ctx: ctx, services: services, slots: make(chan struct{}, 4)})
			ready <- server
			return server
		})
	}()
	select {
	case server := <-ready:
		return id, func() { server.Stop(); <-done }, nil
	case <-done:
		return 0, nil, backend.Fail("unavailable")
	case <-ctx.Done():
		go func() {
			select {
			case s := <-ready:
				s.Stop()
			case <-done:
			}
		}()
		return 0, nil, backend.Fail("deadline_exceeded")
	}
}
