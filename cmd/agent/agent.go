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

const AgentVersion = "1.0.4"
const DefaultSerialPort = "/dev/virtio-ports/org.guest-agent.0"

type Agent struct {
	legacyMode bool
	serialPort string

	withoutSSH bool
	withoutTCP bool

	crtStore cert.Store
}

func (a Agent) ListenAndServe(ctx context.Context) error {
	// The cancel function is needed to be able
	// to shutdown the agent from the GRPC request
	ctx, cancel := context.WithCancel(ctx)

	group, ctx := errgroup.WithContext(ctx)

	//
	// Init GRPC
	//

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

	// Register main GRPC handler
	pb.RegisterAgentServiceServer(grpcSrv, &AgentServiceServer{stat: poller.Stat, cancel: cancel})

	idleConnsClosed := make(chan struct{})

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
		close(idleConnsClosed)
	}()

	//
	// Stat poller
	//

	group.Go(func() error {
		return poller.Run(ctx, 30*time.Second)
	})

	//
	// Listeners
	//

	listeners := make(chan net.Listener)

	go func() {
		defer close(listeners)

		// A main virtio listener
		vl, err := func() (net.Listener, error) {
			if _, err := os.Stat("/dev/vsock"); os.IsNotExist(err) || a.legacyMode {
				log.Debug("Using legacy mode via virtio serial port")
				return devconn.ListenDevice(a.serialPort)
			}
			log.Debug("Using Linux VM sockets (AF_VSOCK) as a transport")
			return vsock.Listen(8383)
		}()
		if err != nil {
			log.Errorf(err.Error())
			return
		}

		listeners <- vl

		if _, ok := vl.(*vsock.Listener); ok || a.withoutTCP {
			return
		}

		// Additional TCP listeners in case the main transport is virtio sirial port
		processed := make(map[string]struct{})

		tlsConfig, err := cert.NewServerConfig(a.crtStore)
		if err != nil {
			log.Debugf("Non-fatal error: unable to use TCP transport: %s", err)
			return
		}

		var attempt int

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			addrs, err := a.getLinkLocalAddrs()
			if err != nil {
				log.Debugf("Non-fatal error: could not get the list of link-local IP adressess: %s", err)
				break
			}

			for _, addr := range addrs {
				if _, ok := processed[addr]; !ok {
					if tl, err := tls.Listen("tcp", net.JoinHostPort(addr, "8383"), tlsConfig); err == nil {
						listeners <- tl
						processed[addr] = struct{}{}
					} else {
						log.Debugf("Non-fatal error: could not bind to %s: %s", addr, err)
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			// We wait for link-local addresses on network interfaces
			// within about 20*3 seconds
			if attempt == 20 {
				break
			}
			attempt++
		}
	}()

	//
	// Run servers
	//

	var srvnum int

	for l := range listeners {
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

		srvnum++
	}

	if srvnum == 0 {
		log.Debug("No one server has been launched. Exit")
		cancel()
	}

	<-idleConnsClosed

	return group.Wait()
}

func (a *Agent) getLinkLocalAddrs() ([]string, error) {
	var lladdrs []string

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, netif := range ifaces {
		ifaddrs, err := netif.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range ifaddrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() == nil {
				if ipnet.IP.IsLinkLocalUnicast() {
					lladdrs = append(lladdrs, ipnet.IP.String()+"%"+netif.Name)
					break
				}
			}
		}
	}

	return lladdrs, nil
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
