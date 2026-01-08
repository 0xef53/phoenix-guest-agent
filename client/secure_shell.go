package client

import (
	"context"
	"os"
	"syscall"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *client) ExecSecureShellClient(ctx context.Context) error {
	var privateKey []byte

	err := c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
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

	tmpfile, err := os.CreateTemp("", "private.*.key")
	if err != nil {
		return err
	}
	defer tmpfile.Close()
	//defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(privateKey); err != nil {
		os.Remove(tmpfile.Name())

		return err
	}

	if err := os.Chmod(tmpfile.Name(), 0400); err != nil {
		return err
	}

	sshArgs := []string{
		"/usr/bin/ssh",
		"-i", tmpfile.Name(),
		"root@127.0.0.22",
		"-p", "4949",
	}

	return syscall.Exec("/usr/bin/ssh", sshArgs, os.Environ())
}
