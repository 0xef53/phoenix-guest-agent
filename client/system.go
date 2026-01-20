package client

import (
	"context"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *client) ShowAgentInfo(ctx context.Context) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		resp, err := grpcClient.System().GetInfo(ctx, new(empty.Empty))
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) ShutdownAgent(ctx context.Context) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		_, err := grpcClient.System().ShutdownAgent(ctx, new(empty.Empty))

		return err
	})
}
