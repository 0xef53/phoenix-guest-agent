package interceptors

import (
	"context"
	"fmt"

	grpc "google.golang.org/grpc"
)

func PreHandlerUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		/*		switch s := info.Server.(type) {
				case *AgentFileSystemService:
					if s.IsLocked() {
						switch info.FullMethod {
						case "/agent.AgentService/UnfreezeFileSystems":
						case "/agent.AgentService/GetAgentInfo":
						case "/agent.AgentService/GetGuestInfo":
						default:
							return nil, fmt.Errorf("All filesystems are frozen. Unable to execute: %s", info.FullMethod)
						}
					}
				}
		*/

		fmt.Printf("DEBUG: PreHandlerUnaryServerInterceptor(): type of info.Server = %T\n", info.Server)

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
