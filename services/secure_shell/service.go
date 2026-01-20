package secure_shell

import (
	"context"
	"fmt"

	"github.com/0xef53/phoenix-guest-agent/services"

	pb "github.com/0xef53/phoenix-guest-agent/api/services/secure_shell/v2"

	grpcserver "github.com/0xef53/go-grpc/server"

	empty "github.com/golang/protobuf/ptypes/empty"
	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	grpc "google.golang.org/grpc"
)

var _ = pb.AgentSecureShellServiceServer(new(Service))

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
	pb.RegisterAgentSecureShellServiceServer(server, s)
}

func (s *Service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}

func (s *Service) GetUserKey(ctx context.Context, _ *empty.Empty) (*pb.GetUserKeyResponse, error) {
	return &pb.GetUserKeyResponse{
		Key: s.ServiceServer.GetUserPrivateKey(ctx),
	}, nil
}
