package main

import (
	"context"
	"sync"
	"time"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	empty "github.com/golang/protobuf/ptypes/empty"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

type AgentServiceServer struct {
	mu     sync.Mutex
	locked bool

	stat func() *pb.GuestInfo

	cancel context.CancelFunc
}

func (s *AgentServiceServer) lock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.locked = true
}

func (s *AgentServiceServer) unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.locked = false
}

func (s *AgentServiceServer) IsLocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.locked
}

func (s *AgentServiceServer) GetAgentInfo(ctx context.Context, req *empty.Empty) (*pb.AgentInfo, error) {
	return &pb.AgentInfo{Version: AgentVersion, IsLocked: s.locked}, nil
}

func (s *AgentServiceServer) GetGuestInfo(ctx context.Context, req *empty.Empty) (*pb.GuestInfo, error) {
	st := s.stat()

	if st == nil || st.Uptime == 0 {
		return nil, grpc_status.Errorf(grpc_codes.NotFound, "not ready yet")
	}

	return st, nil
}

func (s *AgentServiceServer) ShutdownAgent(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	go func() {
		time.Sleep(3 * time.Second)
		s.cancel()
	}()

	return new(empty.Empty), nil
}
