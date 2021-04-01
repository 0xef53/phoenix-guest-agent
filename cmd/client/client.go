package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/0xef53/phoenix-guest-agent/cert"
	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	"github.com/mdlayher/vsock"
	grpc "google.golang.org/grpc"
)

func newClient(target string) (pb.AgentServiceClient, error) {
	var dialfn func(string, time.Duration) (net.Conn, error)

	switch {
	case strings.HasPrefix(target, "cid:"):
		// It is a VM sockets context ID (new scheme via Linux VM sockets)
		if len(target) <= 4 {
			return nil, fmt.Errorf("no context ID defined")
		}
		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			cid, err := strconv.ParseUint(addr[4:], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid socket context ID: %s", err)
			}
			return vsock.Dial(uint32(cid), 8383)
		}
	case strings.HasPrefix(target, "tcp:"):
		// It is an IP address (an alternative scheme when vsock is unavailable)
		if len(target) <= 4 {
			return nil, fmt.Errorf("no IP address defined")
		}
		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			var store cert.Store
			if v := os.Getenv("CERTDIR"); len(v) != 0 {
				store = cert.Dir(v)
			} else {
				store = cert.EmbedStore
			}

			tlsConfig, err := cert.NewClientConfig(store)
			if err != nil {
				return nil, err
			}
			tlsConfig.InsecureSkipVerify = true
			return tls.Dial("tcp", addr[4:]+":8383", tlsConfig)
		}
	case strings.Contains(target, "/"):
		// It is a path to the socket (old scheme via virtserialport)
		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			return net.Dial("unix", addr)
		}
	default:
		return nil, fmt.Errorf("unknown type of a given target string")
	}

	conn, err := grpc.Dial(target, []grpc.DialOption{grpc.WithInsecure(), grpc.WithDialer(dialfn)}...)

	if err != nil {
		return nil, fmt.Errorf("grpc dial error: %s", err)
	}

	return pb.NewAgentServiceClient(conn), nil
}
