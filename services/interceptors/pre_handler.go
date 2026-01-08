package interceptors

import (
	"context"
	"fmt"

	"github.com/0xef53/phoenix-guest-agent/services/filesystem"
	"github.com/0xef53/phoenix-guest-agent/services/system"

	grpc "google.golang.org/grpc"
)

func PreHandlerUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var locked bool

		switch s := info.Server.(type) {
		case *system.Service:
			locked = s.IsLocked()
		case *filesystem.Service:
			locked = s.IsLocked()
		}

		if locked {
			switch info.FullMethod {
			case "/pga.api.services.system.v2.AgentSystemService/GetInfo":
			case "/pga.api.services.agent.v2.AgentService/GetInfo":
			case "/pga.api.services.agent.v2.AgentFileSystemService/Unfreeze":
			default:
				return nil, fmt.Errorf("All filesystems are frozen. Unable to execute: %s", info.FullMethod)
			}
		}

		return handler(ctx, req)
	}
}

func PreHandlerStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		/*
			switch s := srv.(type) {
			case *AgentServiceServer:
				if s.IsLocked() {
					switch info.FullMethod {
					case "/agent.AgentService/DownloadFile":
					default:
						return fmt.Errorf("All filesystems are frozen. Unable to execute: %s", info.FullMethod)
					}
				}
			}
		*/

		fmt.Printf("DEBUG: PreHandlerStreamServerInterceptor(): type of srv = %T\n", srv)

		return handler(srv, stream)
	}
}
