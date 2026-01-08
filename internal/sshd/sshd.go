package sshd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/crypto/ssh"

	"github.com/mdlayher/vsock"
	log "github.com/sirupsen/logrus"
	"github.com/u-root/u-root/pkg/pty"
)

// The ssh package does not define these things so we will
type ptyReq struct {
	TERM   string // TERM environment variable value (e.g., vt100)
	Col    uint32
	Row    uint32
	Xpixel uint32
	Ypixel uint32
	Modes  string // encoded terminal modes
}

type execReq struct {
	Command string
}

type exitStatusReq struct {
	ExitStatus uint32
}

// start a command
// TODO: use /etc/passwd, but the Go support for that is incomplete
func runCommand(c ssh.Channel, p *pty.Pty, cmd string, args ...string) error {
	defer c.Close()

	var ps *os.ProcessState

	if p != nil {
		log.Printf("Executing PTY command %s %v", cmd, args)

		p.Command(cmd, args...)

		if err := p.C.Start(); err != nil {
			log.Debugf("Failed to execute: %v", err)

			return err
		}
		defer p.C.Wait()

		go io.Copy(p.Ptm, c)
		go io.Copy(c, p.Ptm)

		ps, _ = p.C.Process.Wait()
	} else {
		e := exec.Command(cmd, args...)

		e.Stdin, e.Stdout, e.Stderr = c, c, c

		log.Printf("Executing non-PTY command %s %v", cmd, args)

		// execute command and wait for response
		if err := e.Run(); err != nil {
			log.Debugf("Failed to execute: %v", err)

			return err
		}

		ps = e.ProcessState
	}

	// TODO(bluecmd): If somebody wants we can send exit-signal to return
	// information about signal termination, but leave it until somebody needs
	// it.
	// if ws.Signaled() {
	// }
	if ps.Exited() {
		code := uint32(ps.ExitCode())

		log.Debugf("Exit status %v", code)

		c.SendRequest("exit-status", false, ssh.Marshal(exitStatusReq{code}))
	}

	return nil
}

func newPTY(b []byte) (*pty.Pty, error) {
	ptyReq := ptyReq{}

	if err := ssh.Unmarshal(b, &ptyReq); err != nil {
		return nil, err
	}

	p, err := pty.New()
	if err != nil {
		return nil, err
	}

	ws, err := p.TTY.GetWinSize()
	if err != nil {
		return nil, err
	}

	ws.Row = uint16(ptyReq.Row)
	ws.Ypixel = uint16(ptyReq.Ypixel)
	ws.Col = uint16(ptyReq.Col)
	ws.Xpixel = uint16(ptyReq.Xpixel)

	log.Debugf("newPTY: Set winsizes to %v", ws)

	if err := p.TTY.SetWinSize(ws); err != nil {
		return nil, err
	}

	log.Debugf("newPTY: set TERM to %q", ptyReq.TERM)

	if err := os.Setenv("TERM", ptyReq.TERM); err != nil {
		return nil, err
	}

	return p, nil
}

func session(chans <-chan ssh.NewChannel) {
	var p *pty.Pty

	// Service the incoming Channel channel
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")

			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel: %s", err)

			continue
		}

		// Sessions have out-of-band requests such as "shell", "exec"
		// "pty-req" and "env".
		go func(in <-chan *ssh.Request) {
			for req := range in {
				log.Debugf("Request %v", req.Type)

				switch req.Type {
				case "shell":
					err := runCommand(channel, p, "/bin/bash")

					req.Reply(true, []byte(fmt.Sprintf("%v", err)))
				case "exec":
					e := &execReq{}

					if err := ssh.Unmarshal(req.Payload, e); err != nil {
						log.Printf("sshd: %v", err)

						break
					}

					// Execute command using user's shell. This is what OpenSSH does
					// so it's the least surprising to the user.
					err := runCommand(channel, p, "/bin/bash", "-c", e.Command)

					req.Reply(true, []byte(fmt.Sprintf("%v", err)))
				case "pty-req":
					p, err = newPTY(req.Payload)

					req.Reply(err == nil, nil)
				default:
					log.Printf("Not handling req %v %q", req, string(req.Payload))

					req.Reply(false, nil)
				}
			}
		}(requests)
	}
}

type Server struct {
	config *Config

	privateKey []byte

	userPrivateKey []byte
	userPublicKey  []byte
}

func NewServer(c *Config) (*Server, error) {
	serverKey, err := GeneratePrivateKey(2048)
	if err != nil {
		return nil, err
	}

	userKey, err := GeneratePrivateKey(2048)
	if err != nil {
		return nil, err
	}

	userPubKeyBytes, err := GeneratePublicKey(&userKey.PublicKey)
	if err != nil {
		return nil, err
	}

	return &Server{
		config:         c,
		privateKey:     EncodePrivateKeyToPEM(serverKey),
		userPrivateKey: EncodePrivateKeyToPEM(userKey),
		userPublicKey:  userPubKeyBytes,
	}, nil
}

func (s *Server) UserPrivateKey() []byte {
	return s.userPrivateKey
}

func (s *Server) Run(ctx context.Context) error {
	authorizedKeysMap := map[string]bool{}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(s.userPublicKey)
	if err != nil {
		return err
	}

	authorizedKeysMap[string(pubKey.Marshal())] = true

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		// Remove to disable public key auth.
		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if authorizedKeysMap[string(pubKey.Marshal())] {
				return &ssh.Permissions{
					// Record the public key used for authentication.
					Extensions: map[string]string{
						"pubkey-fp": ssh.FingerprintSHA256(pubKey),
					},
				}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", c.User())
		},
	}

	serverKey, err := ssh.ParsePrivateKey(s.privateKey)
	if err != nil {
		return err
	}

	config.AddHostKey(serverKey)

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := vsock.Listen(4949, nil)
	if err != nil {
		return err
	}

	// Закрываем listener при отмене контекста
	go func() {
		<-ctx.Done()

		listener.Close()
	}()

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept incoming connection: %s", err)

			select {
			case <-ctx.Done():
				fmt.Printf("OLOLO DEBUG: closed via context")
				return nil
			default:
				continue
			}
		}

		// Before use, a handshake must be performed on the incoming
		// net.Conn.
		conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
		if err != nil {
			log.Printf("failed to handshake: %v", err)

			continue
		}

		log.Printf("%v logged in with key %s", conn.RemoteAddr(), conn.Permissions.Extensions["pubkey-fp"])

		// The incoming Request channel must be serviced.
		go ssh.DiscardRequests(reqs)

		go session(chans)
	}
}
