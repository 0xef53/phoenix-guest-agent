package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	"github.com/mdlayher/vsock"
	grpc "google.golang.org/grpc"
)

func newClient(target string) (pb.AgentServiceClient, error) {
	var dialfn func(string, time.Duration) (net.Conn, error)

	switch {
	case strings.Contains(target, "/"):
		// It is a path to the socket (old scheme via virtserialport)
		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			return net.Dial("unix", addr)
		}
	case strings.HasPrefix(target, "cid:"):
		// It is a VM sockets context ID (new scheme via Linux VM sockets)
		var cid uint32
		if len(target) <= 4 {
			return nil, fmt.Errorf("invalid sockets context ID: %s", target)
		}
		if v, err := strconv.ParseUint(target[4:], 10, 32); err == nil {
			cid = uint32(v)
		} else {
			return nil, fmt.Errorf("invalid sockets context ID: %s", err)
		}
		dialfn = func(addr string, t time.Duration) (net.Conn, error) {
			return vsock.Dial(cid, 8383)
		}
		target += ":8383"
	default:
		return nil, fmt.Errorf("unknown type of a given target string")
	}

	conn, err := grpc.Dial(target, []grpc.DialOption{grpc.WithInsecure(), grpc.WithDialer(dialfn)}...)

	if err != nil {
		return nil, fmt.Errorf("grpc dial error: %s", err)
	}

	return pb.NewAgentServiceClient(conn), nil
}
