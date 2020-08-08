package main

import (
	"context"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *AgentServiceServer) GetGuestInfo(ctx context.Context, req *empty.Empty) (*pb.GuestInfo, error) {
	return s.stat(), nil
}
