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

var _ = pb.AgentNetworkServiceServer(new(Service))

func init() {
	grpcserver.Register(new(Service), grpcserver.WithServiceBucket("pga"))
}

type Service struct {
	*services.ServiceServer
}

func (s *Service) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *Service) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *Service) RegisterGRPC(server *grpc.Server) {
	pb.RegisterAgentNetworkServiceServer(server, s)
}

func (s *Service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}

func (s *Service) GetRouteList(ctx context.Context, req *pb.GetRouteListRequest) (*pb.GetRouteListResponse, error) {
	routes, err := s.ServiceServer.GetRouteList(ctx, core.InetFamily(req.Family))
	if err != nil {
		return nil, err
	}

	return &pb.GetRouteListResponse{Routes: routesToProto(routes)}, nil
}

func (s *Service) AddRoute(ctx context.Context, req *pb.RouteRequest) (*pb.AddRouteResponse, error) {
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

func (s *Service) DelRoute(ctx context.Context, req *pb.RouteRequest) (*pb.DelRouteResponse, error) {
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

func (s *Service) GetInterfaces(ctx context.Context, _ *empty.Empty) (*pb.GetInterfacesResponse, error) {
	ifaces, err := s.ServiceServer.GetInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GetInterfacesResponse{Interfaces: ifaceListToProto(ifaces)}, nil
}

func (s *Service) SetInterfaceLinkUp(ctx context.Context, req *pb.SetInterfaceLinkStateRequest) (*empty.Empty, error) {
	err := s.ServiceServer.SetInterfaceLinkUp(ctx, req.LinkName)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) SetInterfaceLinkDown(ctx context.Context, req *pb.SetInterfaceLinkStateRequest) (*empty.Empty, error) {
	err := s.ServiceServer.SetInterfaceLinkDown(ctx, req.LinkName)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) AddIPAddr(ctx context.Context, req *pb.IPAddrRequest) (*empty.Empty, error) {
	err := s.ServiceServer.AddIPAddr(ctx, req.LinkName, req.Addr)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) DelIPAddr(ctx context.Context, req *pb.IPAddrRequest) (*empty.Empty, error) {
	err := s.ServiceServer.DelIPAddr(ctx, req.LinkName, req.Addr)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
