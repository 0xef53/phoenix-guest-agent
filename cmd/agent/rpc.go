package main

import (
	"context"
	"sync"
	"time"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	empty "github.com/golang/protobuf/ptypes/empty"
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
	return &pb.AgentInfo{Version: VERSION, IsLocked: s.locked}, nil
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
