package network

import (
	"context"
	"fmt"

	"github.com/0xef53/phoenix-guest-agent/core"
	"github.com/0xef53/phoenix-guest-agent/services"

	pb "github.com/0xef53/phoenix-guest-agent/api/services/agent/v2"

	grpcserver "github.com/0xef53/go-grpc/server"

	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	grpc "google.golang.org/grpc"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/vishvananda/netlink"
)

var _ = pb.AgentNetworkServiceServer(new(service))

func init() {
	grpcserver.Register(new(service), grpcserver.WithServiceBucket("pga"))
}

type service struct {
	*services.ServiceServer
}

func (s *service) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *service) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *service) RegisterGRPC(server *grpc.Server) {
	pb.RegisterAgentNetworkServiceServer(server, s)
}

func (s *service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}

func (s *service) GetRouteList(ctx context.Context, req *pb.GetRouteListRequest) (*pb.GetRouteListResponse, error) {
	routes, err := s.ServiceServer.GetRouteList(ctx, core.InetFamily(req.Family))
	if err != nil {
		return nil, err
	}

	return &pb.GetRouteListResponse{Routes: routesToProto(routes)}, nil
}

func (s *service) AddRoute(ctx context.Context, req *pb.RouteRequest) (*pb.AddRouteResponse, error) {
	attrs := core.RouteAttrs{
		LinkName: req.LinkName,
		Scope:    netlink.Scope(req.Scope),
		Dst:      req.Dst,
		Src:      req.Src,
		Gw:       req.Gw,
		Table:    int(req.Table),
	}

	r, err := s.ServiceServer.AddRoute(ctx, &attrs)
	if err != nil {
		return nil, err
	}

	return &pb.AddRouteResponse{Route: routeToProto(r)}, nil
}

func (s *service) DelRoute(ctx context.Context, req *pb.RouteRequest) (*pb.DelRouteResponse, error) {
	attrs := core.RouteAttrs{
		LinkName: req.LinkName,
		Scope:    netlink.Scope(req.Scope),
		Dst:      req.Dst,
		Src:      req.Src,
		Gw:       req.Gw,
		Table:    int(req.Table),
	}

	r, err := s.ServiceServer.DelRoute(ctx, &attrs)
	if err != nil {
		return nil, err
	}

	return &pb.DelRouteResponse{Route: routeToProto(r)}, nil
}

func (s *service) GetInterfaces(ctx context.Context, _ *empty.Empty) (*pb.GetInterfacesResponse, error) {
	ifaces, err := s.ServiceServer.GetInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GetInterfacesResponse{Interfaces: ifaceListToProto(ifaces)}, nil
}

func (s *service) SetInterfaceLinkUp(ctx context.Context, req *pb.SetInterfaceLinkStateRequest) (*empty.Empty, error) {
	err := s.ServiceServer.SetInterfaceLinkUp(ctx, req.LinkName)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) SetInterfaceLinkDown(ctx context.Context, req *pb.SetInterfaceLinkStateRequest) (*empty.Empty, error) {
	err := s.ServiceServer.SetInterfaceLinkDown(ctx, req.LinkName)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) AddIPAddr(ctx context.Context, req *pb.IPAddrRequest) (*empty.Empty, error) {
	err := s.ServiceServer.AddIPAddr(ctx, req.LinkName, req.Addr)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DelIPAddr(ctx context.Context, req *pb.IPAddrRequest) (*empty.Empty, error) {
	err := s.ServiceServer.DelIPAddr(ctx, req.LinkName, req.Addr)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
