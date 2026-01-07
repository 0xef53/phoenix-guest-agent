package client

import (
	"context"
	"crypto/tls"

	"github.com/0xef53/phoenix-guest-agent/cert"
	"github.com/0xef53/phoenix-guest-agent/client/grpcclient"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"
)

type client struct {
	endpoint  string
	tlsConfig *tls.Config

	crtStore cert.Store
}

func NewClient(endpoint string, crtStore cert.Store) (*client, error) {
	tlsConfig, err := cert.NewServerConfig(crtStore)
	if err != nil {
		return nil, err
	}

	return &client{
		endpoint:  endpoint,
		tlsConfig: tlsConfig,
		crtStore:  crtStore,
	}, nil
}

func (c *client) executeGRPC(ctx context.Context, fn func(*grpc_interfaces.Agent) error) error {
	return grpcclient.ExecuteGRPC(ctx, c.endpoint, c.tlsConfig, fn)
}
