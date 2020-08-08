package main

import (
	"context"
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc "google.golang.org/grpc"
)

type RegisterGRPCFunc func(*grpc.Server)

func unaryLogRequestInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	tags := grpc_ctxtags.Extract(ctx)
	log.WithFields(tags.Values()).Debug("GRPC Request: ", info.FullMethod)

	return handler(ctx, req)
}

func unaryPreHandlerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	switch s := info.Server.(type) {
	case *AgentServiceServer:
		if s.IsLocked() {
			switch info.FullMethod {
			case "/agent.AgentService/UnfreezeFileSystems", "/agent.AgentService/GetAgentInfo", "/agent.AgentService/GetGuestInfo":
			default:
				return nil, fmt.Errorf("All filesystems are frozen. Unable to execute: %s", info.FullMethod)
			}
		}
	}

	return handler(ctx, req)
}

func streamPreHandlerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
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

	return handler(srv, stream)
}

func newGRPCServer() *grpc.Server {
	return grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.TagBasedRequestFieldExtractor("log"))),
			grpc.UnaryServerInterceptor(unaryLogRequestInterceptor),
			grpc.UnaryServerInterceptor(unaryPreHandlerInterceptor),
		),
		grpc_middleware.WithStreamServerChain(
			grpc.StreamServerInterceptor(streamPreHandlerInterceptor),
		),
	)
}

func runGRPCServer(ctx context.Context, listener net.Listener, registerGRPC RegisterGRPCFunc) error {
	defer listener.Close()

	srv := newGRPCServer()

	// Register GRPS handlers
	registerGRPC(srv)

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	return srv.Serve(listener)
}
