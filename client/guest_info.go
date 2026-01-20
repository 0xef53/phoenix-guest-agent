package client

import (
	"context"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *client) ShowGuestInfo(ctx context.Context) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		resp, err := grpcClient.Agent().GetInfo(ctx, new(empty.Empty))
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}
