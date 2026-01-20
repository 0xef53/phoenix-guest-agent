package grpcclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/0xef53/phoenix-guest-agent/core"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	grpcclient_interceptors "github.com/0xef53/go-grpc/client/interceptors"

	grpc "google.golang.org/grpc"
	grpc_credentials "google.golang.org/grpc/credentials"
	grpc_credentials_insecure "google.golang.org/grpc/credentials/insecure"

	"github.com/mdlayher/vsock"
)

func ExecuteGRPC(ctx context.Context, endpoint string, tlsConfig *tls.Config, fn func(*grpc_interfaces.Agent) error) error {
	conn, err := newConnection(endpoint, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	return fn(grpc_interfaces.NewAgentInterface(conn))
}

func newConnection(endpoint string, tlsConfig *tls.Config) (*grpc.ClientConn, error) {
	if tlsConfig == nil {
		return nil, fmt.Errorf("TLS config is not set")
	}

	var withoutTLS bool

	var dialfn func(string, time.Duration) (net.Conn, error)

	switch {
	case strings.HasPrefix(endpoint, "cid:"):
		// It is a VM sockets context ID (mainstream scheme via Linux VM sockets)
		if len(endpoint) <= 4 {
			return nil, fmt.Errorf("no context ID defined")
		}

		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			cid, err := strconv.ParseUint(addr[4:], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid socket context ID: %s", err)
			}

			return vsock.Dial(uint32(cid), core.GRPCPort, nil)
		}
	case strings.HasPrefix(endpoint, "tcp:"):
		// It is an IP address (an alternative scheme when vsock is unavailable)
		if len(endpoint) <= 4 {
			return nil, fmt.Errorf("no IP address defined")
		}

		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			return net.Dial("tcp", net.JoinHostPort(addr[4:], fmt.Sprintf("%d", core.GRPCPort)))
		}
	case strings.Contains(endpoint, "/"):
		// It is a path to the socket (old scheme via virtserialport)
		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			return net.Dial("unix", addr)
		}

		withoutTLS = true
	default:
		return nil, fmt.Errorf("unknown endpoint")
	}

	dialOpts := []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(
			grpcclient_interceptors.WithRequestIdentifier(),
		),
		grpc.WithChainStreamInterceptor(
			grpcclient_interceptors.WithStreamRequestIdentifier(),
		),
		grpc.WithDialer(dialfn),
	}

	if withoutTLS {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(grpc_credentials_insecure.NewCredentials()))
	} else {
		// We use only secure connection with VSOCK and TCP
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(grpc_credentials.NewTLS(tlsConfig)))
	}

	return grpc.Dial(endpoint, dialOpts...)
}
