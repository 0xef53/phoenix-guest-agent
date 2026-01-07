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

var _ = pb.AgentSystemServiceServer(new(service))

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
	pb.RegisterAgentSystemServiceServer(server, s)
}

func (s *service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}

func (s *service) GetInfo(ctx context.Context, _ *empty.Empty) (*pb.GetInfoResponse, error) {
	return &pb.GetInfoResponse{
		Info: agentInfoToProto(s.ServiceServer.GetAgentInfo(ctx)),
	}, nil
}

func (s *service) ShutdownAgent(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	return new(empty.Empty), nil
}
