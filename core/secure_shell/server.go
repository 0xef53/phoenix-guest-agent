package secure_shell

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	secure_shell "github.com/0xef53/phoenix-guest-agent/internal/secure_shell"

	"github.com/creack/pty"
	"github.com/mdlayher/vsock"
	log "github.com/sirupsen/logrus"
)

const (
	Version         = "1.0.0"
	ExtendedVersion = "PGA-built-in-SecureShell-" + Version
)

var ErrServerClosed = secure_shell.ErrServerClosed

type Config struct {
	Port int
}

type Server struct {
	config *Config

	userPrivateKey []byte
	userPublicKey  []byte
}

func NewServer(c *Config) (*Server, error) {
	userKey, err := secure_shell.GeneratePrivateKey(2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new user key: %w", err)
	}

	userKeyBytes := secure_shell.EncodePrivateKeyToPEM(userKey)

	userPubKeyBytes, err := secure_shell.GeneratePublicKey(&userKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create new user pubkey: %w", err)
	}

	return &Server{
		config:         c,
		userPrivateKey: userKeyBytes,
		userPublicKey:  userPubKeyBytes,
	}, nil
}

func (s *Server) UserPrivateKey() []byte {
	return s.userPrivateKey
}

func (s *Server) ListenAndServer(ctx context.Context) error {
	log.WithField("port", s.config.Port).Info("Starting Remote Control Protocol")

	listener, err := vsock.Listen(uint32(s.config.Port), nil)
	if err != nil {
		return err
	}

	srv := &secure_shell.Server{
		Handler: s.handler,
		Version: ExtendedVersion,
		Banner:  "Welcome to PGA's built-in Secure Shell v" + Version + "\n",

		// Timeouts
		IdleTimeout: 600 * time.Second,
	}

	// Public key auth
	err = srv.SetOption(secure_shell.PublicKeyAuth(
		func(ctx secure_shell.Context, key secure_shell.PublicKey) bool {
			allowed, _, _, _, _ := secure_shell.ParseAuthorizedKey(s.userPublicKey)

			return secure_shell.KeysEqual(key, allowed)
		},
	))
	if err != nil {
		return err
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()

		log.WithField("port", s.config.Port).Info("Remote Control Protocol stopped")

		// For now, it seems that there is no need to wait for all active sessions
		// to be released, as this is too long and does not fit into the current concept
		// of terminating the guest agent.
		srv.Shutdown(context.Background())
	}()

	return srv.Serve(listener)
}

func (s *Server) handler(sess secure_shell.Session) {
	defer sess.Close()

	u, err := user.Lookup(sess.User())
	if err != nil {
		io.WriteString(sess, fmt.Sprintf("User lookup failed: %s\n", err))
		sess.Exit(1)
		return
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		io.WriteString(sess, fmt.Sprintf("Invalid UID of user '%s': %s\n", sess.User(), err))
		sess.Exit(1)
		return
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		io.WriteString(sess, fmt.Sprintf("Invalid GID of user '%s': %s\n", sess.User(), err))
		sess.Exit(1)
		return
	}

	shell := getUserShell(u)

	extraEnvs := make([]string, 0, len(sess.Environ()))

	for _, envline := range sess.Environ() {
		switch {
		case strings.HasPrefix(envline, "SHELL="):
			// Overriding the user's shell
			parts := strings.SplitN(envline, "=", 2)

			if len(parts) == 2 {
				shell = parts[1]
			}

			extraEnvs = append(extraEnvs, envline)
		case strings.HasPrefix(envline, "LANG="):
			extraEnvs = append(extraEnvs, envline)
		}
	}

	ptyReq, winCh, isPty := sess.Pty()

	if isPty {
		cmd := exec.Command(shell)

		cmd.Dir = u.HomeDir

		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TERM=%s", ptyReq.Term),
			fmt.Sprintf("HOME=%s", u.HomeDir),
			fmt.Sprintf("USER=%s", u.Username),
			"HISTFILE=/dev/null",
		)

		// Also append extra envs from SSH client
		cmd.Env = append(cmd.Env, extraEnvs...)

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
			Setctty: true,
			Setsid:  true,
		}

		fd, err := pty.Start(cmd)
		if err != nil {
			io.WriteString(sess, fmt.Sprintf("Cannot assign a pseudo-terminal tty: %s\n", err))
			sess.Exit(1)
			return
		}
		// Do not close manually.
		// The descriptor will be closed automatically at the appropriate time.
		//defer fd.Close()

		go func() {
			for win := range winCh {
				setWinsize(fd, win.Width, win.Height)
			}
		}()

		go func() {
			io.Copy(fd, sess) // stdin
		}()

		io.Copy(sess, fd) // stdout

		cmd.Wait()
	} else {
		if len(sess.Command()) == 0 {
			io.WriteString(sess, "No command given: exit status 1\n")
			sess.Exit(1)
			return
		}

		cmd := exec.Command(shell, "-c", strings.Join(sess.Command(), " "))

		cmd.Dir = u.HomeDir

		cmd.Env = append(cmd.Env,
			fmt.Sprintf("HOME=%s", u.HomeDir),
			fmt.Sprintf("USER=%s", u.Username),
			"HISTFILE=/dev/null",
		)

		// Also append extra envs from SSH client
		cmd.Env = append(cmd.Env, extraEnvs...)

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}

		cmd.Stdout = sess
		cmd.Stderr = sess.Stderr()

		stdin, err := cmd.StdinPipe()
		if err != nil {
			io.WriteString(sess, fmt.Sprintf("Failed to create stdin pipe: %s\n", err))
			sess.Exit(1)
			return
		}
		go func() {
			defer stdin.Close()

			io.Copy(stdin, sess)
		}()

		if err := cmd.Run(); err != nil {
			io.WriteString(sess.Stderr(), fmt.Sprintf("Command exited with error: %s\n", err))
		}

		sess.CloseWrite()
		sess.Exit(commandExitCode(err))
	}
}
