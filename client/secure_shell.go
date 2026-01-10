package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/0xef53/phoenix-guest-agent/core"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *client) ExecSecureShellClient(ctx context.Context, endpoint, user, shell, socatBinary string, command ...string) error {
	endpoint = strings.TrimSpace(endpoint)

	var vmHost string

	if strings.HasPrefix(endpoint, "cid:") {
		vmHost = fmt.Sprintf("vm-cid-%s", endpoint[4:])
	} else {
		return fmt.Errorf("secure shell is only available through Linux VM sockets (AF_VSOCK)")
	}

	user = strings.TrimSpace(user)

	if len(user) == 0 {
		user = "root"
	}

	socatBinary = strings.TrimSpace(socatBinary)

	if len(socatBinary) == 0 {
		// Try to find default system socat
		if p, err := exec.LookPath("socat"); err == nil {
			socatBinary = p
		} else {
			return err
		}
	}

	sshBinary, err := exec.LookPath("ssh")
	if err != nil {
		return err
	}

	var privateKey []byte

	err = c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		resp, err := grpcClient.SecureShell().GetUserKey(ctx, new(empty.Empty))
		if err != nil {
			return err
		}

		privateKey = resp.Key

		return nil
	})
	if err != nil {
		return err
	}

	tmpfile, err := os.CreateTemp("", "pga.*.key")
	if err != nil {
		return err
	}
	defer tmpfile.Close()
	defer func() {
		time.Sleep(1 * time.Second)

		os.Remove(tmpfile.Name())
	}()

	if _, err := tmpfile.Write(privateKey); err != nil {
		os.Remove(tmpfile.Name())

		return err
	}

	tmpfile.Close()

	if err := os.Chmod(tmpfile.Name(), 0400); err != nil {
		return err
	}

	sshArgs := []string{
		sshBinary,
		"-o", "StrictHostKeyChecking=no",
		"-o", fmt.Sprintf("ProxyCommand=%s - VSOCK-CONNECT:%s:%d", socatBinary, endpoint[4:], core.RCPPort),
	}

	shell = strings.TrimSpace(shell)

	if len(shell) > 0 {
		sshArgs = append(sshArgs, "-o", "SetEnv SHELL="+shell)
	}

	sshArgs = append(sshArgs,
		"-i", tmpfile.Name(),
		"-l", user,
		"-p", fmt.Sprintf("%d", core.RCPPort),
		vmHost,
	)

	sshArgs = append(sshArgs, command...)

	fmt.Printf("Command: %s\n", strings.Join(sshArgs, " "))

	return syscall.Exec(sshBinary, sshArgs, os.Environ())
}
