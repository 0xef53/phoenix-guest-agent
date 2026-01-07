package services

import (
	"fmt"

	"github.com/0xef53/phoenix-guest-agent/core"

	grpcserver "github.com/0xef53/go-grpc/server"
)

type ServiceServer struct {
	*core.Server
}

func NewServiceServer(base *core.Server) (*ServiceServer, error) {
	h := &ServiceServer{
		Server: base,
	}

	for _, s := range grpcserver.Services("pga") {
		if x, ok := s.(interface{ Init(*ServiceServer) }); ok {
			x.Init(h)
		} else {
			return nil, fmt.Errorf("invalid 'PGA' interface: %T", s)
		}
	}

	return h, nil
}
