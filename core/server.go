package core

import (
	"context"
	"sync"
	"syscall"
	"time"

	secure_shell "github.com/0xef53/phoenix-guest-agent/core/secure_shell"
	"github.com/0xef53/phoenix-guest-agent/internal/version"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	SessionID string

	mu     sync.Mutex
	locked bool

	stat       func() *GuestInfo
	sshUserKey func() []byte

	features *AgentFeatures
}

func NewServer(ctx context.Context, features *AgentFeatures) (*Server, error) {
	srv := Server{
		SessionID: uuid.New().String(),
		features:  features,
	}

	// Start stat poller
	poller := StatPoller{}

	srv.stat = poller.Stat

	go poller.Run(ctx, 30*time.Second)

	// Start Secure Shell server
	if !features.WithoutSSH && !features.LegacyMode {
		go func() {
			sshSrv, err := secure_shell.NewServer(&secure_shell.Config{Port: RCPPort})
			if err != nil {
				log.WithField("port", RCPPort).Errorf("Cannot init Remote Control Protocol: %s", err)

				return
			}

			srv.sshUserKey = sshSrv.UserPrivateKey

			if err := sshSrv.ListenAndServer(ctx); err != nil {
				if err != secure_shell.ErrServerClosed {
					log.WithField("port", RCPPort).Errorf("Cannot start Remote Control Protocol: %s", err)
				}

				return
			}
		}()
	}

	return &srv, nil
}

func (s *Server) GracefulShutdown(_ context.Context) error {
	log.Info("A graceful shutdown requested. SIGTEM will be sent to the agent process")

	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	time.Sleep(3 * time.Second)

	return nil
}

func (s *Server) lock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.locked = true
}

func (s *Server) unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.locked = false
}

func (s *Server) IsLocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.locked
}

type AgentInfo struct {
	SessionID string
	Version   *version.Version
	IsLocked  bool
	Features  *AgentFeatures
}

type AgentFeatures struct {
	LegacyMode bool
	SerialPort string
	WithoutSSH bool
	WithoutTCP bool
}

func (s *Server) GetAgentInfo(ctx context.Context) *AgentInfo {
	return &AgentInfo{
		SessionID: s.SessionID,
		Version:   &AgentVersion,
		IsLocked:  s.locked,
		Features:  s.features,
	}
}
