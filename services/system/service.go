package system

import (
	"context"
	"fmt"

	"github.com/0xef53/phoenix-guest-agent/services"

	pb "github.com/0xef53/phoenix-guest-agent/api/services/system/v2"

	grpcserver "github.com/0xef53/go-grpc/server"

	empty "github.com/golang/protobuf/ptypes/empty"
	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	grpc "google.golang.org/grpc"
)

var _ = pb.AgentSystemServiceServer(new(Service))

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
	pb.RegisterAgentSystemServiceServer(server, s)
}

func (s *Service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}

func (s *Service) GetInfo(ctx context.Context, _ *empty.Empty) (*pb.GetInfoResponse, error) {
	return &pb.GetInfoResponse{
		Info: agentInfoToProto(s.ServiceServer.GetAgentInfo(ctx)),
	}, nil
}

func (s *Service) ShutdownAgent(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	return new(empty.Empty), nil
}
