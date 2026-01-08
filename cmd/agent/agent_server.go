package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/0xef53/phoenix-guest-agent/cert"
	"github.com/0xef53/phoenix-guest-agent/core"
	"github.com/0xef53/phoenix-guest-agent/internal/devconn"
	"github.com/0xef53/phoenix-guest-agent/services"
	"github.com/0xef53/phoenix-guest-agent/services/interceptors"

	_ "github.com/0xef53/phoenix-guest-agent/services/agent"
	_ "github.com/0xef53/phoenix-guest-agent/services/filesystem"
	_ "github.com/0xef53/phoenix-guest-agent/services/network"
	_ "github.com/0xef53/phoenix-guest-agent/services/secure_shell"
	_ "github.com/0xef53/phoenix-guest-agent/services/system"

	grpcserver "github.com/0xef53/go-grpc/server"
	grpcserver_interceptors "github.com/0xef53/go-grpc/server/interceptors"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc "google.golang.org/grpc"
	grpc_credentials "google.golang.org/grpc/credentials"

	"github.com/mdlayher/vsock"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Agent struct {
	core.AgentFeatures

	crtStore cert.Store
}

func (a *Agent) ListenAndServe(ctx context.Context) error {
	if len(a.SerialPort) == 0 {
		a.SerialPort = core.DefaultGuestSerialPort
	}

	tlsConfig, err := cert.NewServerConfig(a.crtStore)
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)

	//
	// Init GRPC
	//

	ui := []grpc.UnaryServerInterceptor{
		interceptors.PreHandlerUnaryServerInterceptor(),
		interceptors.MapErrorsUnaryServerInterceptor(),
	}

	si := []grpc.StreamServerInterceptor{
		interceptors.PreHandlerStreamServerInterceptor(),
	}

	grpcSrv := newServer(ui, si, tlsConfig)

	if h, err := core.NewServer(ctx, &a.AgentFeatures); err == nil {
		if _, err := services.NewServiceServer(h); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("pre-start error: %w", err)
	}

	for _, svc := range grpcserver.Services("pga") {
		log.Info("Registering service: ", svc.Name())

		svc.RegisterGRPC(grpcSrv)
	}

	idleConnsClosed := make(chan struct{})

	// Graceful shutdown
	go func() {
		<-ctx.Done()

		grpcSrv.GracefulStop()

		close(idleConnsClosed)
	}()

	//
	// Listeners
	//

	listeners := make(chan net.Listener)

	go func() {
		defer close(listeners)

		// A main virtio listener
		vl, err := func() (net.Listener, error) {
			if _, err := os.Stat("/dev/vsock"); os.IsNotExist(err) || a.LegacyMode {
				log.Info("Using legacy mode via virtio serial port")

				return devconn.ListenDevice(a.SerialPort)
			}
			log.Info("Using Linux VM sockets (AF_VSOCK) as a transport")

			return vsock.Listen(core.GRPCPort, nil)
		}()
		if err != nil {
			log.Errorf("%s", err.Error())

			return
		}

		listeners <- vl

		if _, ok := vl.(*vsock.Listener); ok || a.WithoutTCP {
			return
		}

		// Additional TCP listeners in case the main transport is virtio sirial port
		processed := make(map[string]struct{})

		var attempt int

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			addrs, err := getLinkLocalAddrs()
			if err != nil {
				log.Warnf("Could not get the list of IPv6 link-local adressess: %s", err)

				break
			}

			for _, addr := range addrs {
				if _, ok := processed[addr]; !ok {
					if l, err := net.Listen("tcp", net.JoinHostPort(addr, fmt.Sprintf("%d", core.GRPCPort))); err == nil {
						listeners <- l

						processed[addr] = struct{}{}
					} else {
						log.Errorf("Could not bind to %s: %s", addr, err)
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
	// Run GRPC servers
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
		log.Error("No one server has been launched. Exit")
	}

	<-idleConnsClosed

	return group.Wait()
}

// newServer returns a new grpc.Server instance with a preconfigured list of interceptors.
func newServer(ui []grpc.UnaryServerInterceptor, si []grpc.StreamServerInterceptor, tlsConfig *tls.Config) *grpc.Server {
	_ui := append(grpcserver.DefaultUnaryInterceptors, ui...)

	// Add after the "ui" to allow changes in "grpc_ctxtags"
	_ui = append(_ui, grpcserver_interceptors.LogRequestUnaryServerInterceptor())

	_si := append(grpcserver.DefaultStreamInterceptors, si...)

	// Add after the "si" to allow changes in "grpc_ctxtags"
	_si = append(_si, grpcserver_interceptors.LogRequestStreamServerInterceptor())

	opts := []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(_ui...),
		grpc_middleware.WithStreamServerChain(_si...),
	}

	if tlsConfig != nil {
		opts = append(opts, grpc.Creds(grpc_credentials.NewTLS(tlsConfig)))
	}

	return grpc.NewServer(opts...)
}

func getLinkLocalAddrs() ([]string, error) {
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
