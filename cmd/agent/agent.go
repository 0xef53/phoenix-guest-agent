package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xef53/phoenix-guest-agent/pkg/devconn"
	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	"github.com/mdlayher/vsock"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type Agent struct {
	WithoutSSH bool
	LegacyMode bool
	SerialPort string
}

func (a Agent) Serve() error {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)
		s := <-sigc

		log.WithFields(log.Fields{"signal": s}).Info("Graceful shutdown initiated ...")

		cancel()
	}()

	// Which listener should we use ?
	listener, err := func() (net.Listener, error) {
		if _, err := os.Stat("/dev/vsock"); os.IsNotExist(err) || a.LegacyMode {
			// legacy mode via virtio serial port
			log.Debug("Using legacy mode via virtio serial port")
			return devconn.ListenDevice(a.SerialPort)
		}
		log.Debug("Using Linux VM sockets (AF_VSOCK) as a transport")
		return vsock.Listen(8383)
	}()
	if err != nil {
		return err
	}

	// Run stat poller and insecure GRPC server
	group, ctx := errgroup.WithContext(ctx)

	poller := StatPoller{}
	group.Go(func() error {
		return poller.Run(ctx, 30*time.Second)
	})

	group.Go(func() error {
		return runGRPCServer(ctx, listener, func(srv *grpc.Server) {
			pb.RegisterAgentServiceServer(srv, &AgentServiceServer{stat: poller.Stat, cancel: cancel})
		})
	})

	return group.Wait()
}
