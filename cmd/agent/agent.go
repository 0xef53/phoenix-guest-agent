package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/0xef53/phoenix-guest-agent/cert"
	"github.com/0xef53/phoenix-guest-agent/pkg/devconn"
	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc "google.golang.org/grpc"

	"github.com/mdlayher/vsock"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const AgentVersion = "1.0.3"
const DefaultSerialPort = "/dev/virtio-ports/org.guest-agent.0"

type Agent struct {
	legacyMode bool
	serialPort string

	withoutSSH bool
	withoutTCP bool

	crtStore cert.Store
}

func (a Agent) ListenAndServe(ctx context.Context) error {
	grpcSrv := grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.TagBasedRequestFieldExtractor("log"))),
			grpc.UnaryServerInterceptor(unaryLogRequestInterceptor),
			grpc.UnaryServerInterceptor(unaryPreHandlerInterceptor),
		),
		grpc_middleware.WithStreamServerChain(
			grpc.StreamServerInterceptor(streamPreHandlerInterceptor),
		),
	)

	poller := StatPoller{}

	// The cancel function is needed here to be able
	// to shutdown the agent from the GRPC request
	ctx, cancel := context.WithCancel(ctx)

	// Register main GRPC handler
	pb.RegisterAgentServiceServer(grpcSrv, &AgentServiceServer{stat: poller.Stat, cancel: cancel})

	//
	// Listeners
	//

	var listeners []net.Listener

	if _, err := os.Stat("/dev/vsock"); os.IsNotExist(err) || a.legacyMode {
		log.Debug("Using legacy mode via virtio serial port")

		if l, err := devconn.ListenDevice(a.serialPort); err == nil {
			listeners = append(listeners, l)
		} else {
			return err
		}

		if !a.withoutTCP {
			if ll, err := a.linkLocalListeners(); err == nil {
				listeners = append(listeners, ll...)
			} else {
				log.Debugf("Non-fatal error: unable to use IPv6 link-local addresses: %s", err)
			}
		}
	} else {
		log.Debug("Using Linux VM sockets (AF_VSOCK) as a transport")

		if l, err := vsock.Listen(8383); err == nil {
			listeners = append(listeners, l)
		} else {
			return err
		}
	}

	//
	// Run servers
	//

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return poller.Run(ctx, 30*time.Second)
	})

	idleConnsClosed := make(chan struct{})

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
		close(idleConnsClosed)
	}()

	for _, l := range listeners {
		listener := l
		laddr := listener.Addr().String()

		group.Go(func() error {
			log.WithField("addr", laddr).Info("Starting GRPC server")

			if err := grpcSrv.Serve(listener); err != nil {
				// Error starting or closing listener
				return err
			}

			log.WithField("addr", laddr).Info("GRPC server stopped")

			return nil
		})
	}

	<-idleConnsClosed

	return group.Wait()
}

func (a *Agent) linkLocalListeners() ([]net.Listener, error) {
	tlsConfig, err := cert.NewServerConfig(a.crtStore)
	if err != nil {
		return nil, err
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var listeners []net.Listener

	for _, netif := range ifaces {
		addrs, err := netif.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() == nil {
				if ipnet.IP.IsLinkLocalUnicast() {
					l, err := tls.Listen("tcp", "["+ipnet.IP.String()+"%"+netif.Name+"]:8383", tlsConfig)
					if err != nil {
						return nil, err
					}
					listeners = append(listeners, l)
					break
				}
			}
		}
	}

	return listeners, nil
}

func unaryLogRequestInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	tags := grpc_ctxtags.Extract(ctx)

	log.WithFields(tags.Values()).Debug("GRPC Request: ", info.FullMethod)

	return handler(ctx, req)
}

func unaryPreHandlerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	switch s := info.Server.(type) {
	case *AgentServiceServer:
		if s.IsLocked() {
			switch info.FullMethod {
			case "/agent.AgentService/UnfreezeFileSystems", "/agent.AgentService/GetAgentInfo", "/agent.AgentService/GetGuestInfo":
			default:
				return nil, fmt.Errorf("All filesystems are frozen. Unable to execute: %s", info.FullMethod)
			}
		}
	}

	return handler(ctx, req)
}

func streamPreHandlerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	switch s := srv.(type) {
	case *AgentServiceServer:
		if s.IsLocked() {
			switch info.FullMethod {
			case "/agent.AgentService/DownloadFile":
			default:
				return fmt.Errorf("All filesystems are frozen. Unable to execute: %s", info.FullMethod)
			}
		}
	}

	return handler(srv, stream)
}
