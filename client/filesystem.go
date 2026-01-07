package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	pb_agent "github.com/0xef53/phoenix-guest-agent/api/services/agent/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *client) ShowFileStat(ctx context.Context, fname string, useLongFormat, withoutContent bool) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		req := pb_agent.GetFileStatRequest{
			Path:           fname,
			WithDirContent: !withoutContent,
		}

		resp, err := grpcClient.FileSystem().GetFileStat(ctx, &req)
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) ShowFileMD5Hash(ctx context.Context, fname string) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		req := pb_agent.GetFileMD5HashRequest{
			Path: fname,
		}

		resp, err := grpcClient.FileSystem().GetFileMD5Hash(ctx, &req)
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) SetFileOwner(ctx context.Context, fname, owner string) error {
	parts := strings.SplitN(owner, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/group value: %s", owner)
	}

	if len(parts[1]) == 0 {
		parts[1] = parts[0]
	}

	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		req := pb_agent.SetFileOwnerRequest{
			Path:  fname,
			Owner: parts[0],
			Group: parts[1],
		}

		resp, err := grpcClient.FileSystem().SetFileOwner(ctx, &req)
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) SetFileMode(ctx context.Context, fname, perm string) error {
	m, err := strconv.ParseUint(perm, 8, 32)
	if err != nil {
		return err
	}

	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		req := pb_agent.SetFileModeRequest{
			Path: fname,
			Mode: uint32(m),
		}

		resp, err := grpcClient.FileSystem().SetFileMode(ctx, &req)
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) CreateDir(ctx context.Context, dirname, perm string) error {
	var mode uint32

	if len(perm) == 0 {
		mode = 0755
	} else {
		if m, err := strconv.ParseUint(perm, 8, 32); err == nil {
			mode = uint32(m)
		} else {
			return err
		}
	}

	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		req := pb_agent.CreateDirRequest{
			Path: dirname,
			Mode: mode,
		}

		_, err := grpcClient.FileSystem().CreateDir(ctx, &req)

		return err
	})
}

func (c *client) CopyFile(ctx context.Context, srcname, dstname string) error {
	copyTo := func(src, dst string) error {
		var r io.Reader

		if src == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(src)
			if err != nil {
				return err
			}
			defer f.Close()

			r = f
		}

		return c.streamTo(ctx, r, dst)
	}

	copyFrom := func(src, dst string) error {
		switch st, err := os.Stat(dst); {
		case err == nil:
			if st.IsDir() {
				dst = filepath.Join(dst, filepath.Base(src))
			}
		case os.IsNotExist(err):
			if strings.HasSuffix(dst, "/") {
				return err
			}
		default:
			return err
		}

		tmpfile, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(src)+".*")
		if err != nil {
			return err
		}
		defer tmpfile.Close()
		defer os.Remove(tmpfile.Name())

		if err := c.streamFrom(ctx, src, tmpfile); err != nil {
			return err
		}

		return os.Rename(tmpfile.Name(), dst)
	}

	prefix := "guest:"

	switch {
	case strings.HasPrefix(srcname, prefix):
		return copyFrom(strings.TrimPrefix(srcname, prefix), dstname)
	case strings.HasPrefix(dstname, prefix):
		return copyTo(srcname, strings.TrimPrefix(dstname, prefix))
	}

	return fmt.Errorf("must specify at least one 'guest:' source")
}

func (c *client) ShowFileContent(ctx context.Context, fname string) error {
	return c.streamFrom(ctx, fname, os.Stdout)
}

func (c *client) streamTo(ctx context.Context, src io.Reader, dst string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		stream, err := grpcClient.FileSystem().UploadFile(ctx)
		if err != nil {
			return fmt.Errorf("cannot create new upload stream: %s", err)
		}

		req := pb_agent.UploadFileRequest{
			Data: &pb_agent.UploadFileRequest_Info{
				Info: &pb_agent.UploadFileRequest_FileInfo{
					Path: dst,
				},
			},
		}

		if err := stream.Send(&req); err != nil {
			return fmt.Errorf("initial request failed: %s, %s", err, stream.RecvMsg(nil))
		}

		buffer := make([]byte, 2*1024*1024)

		for {
			n, err := src.Read(buffer)
			if err != nil {
				if err == io.EOF {
					break
				}

				return fmt.Errorf("chunk read failed: %s", err)
			}

			req := pb_agent.UploadFileRequest{
				Data: &pb_agent.UploadFileRequest_ChunkData{
					ChunkData: buffer[:n],
				},
			}

			if err := stream.Send(&req); err != nil {
				return fmt.Errorf("chunk send failed: %s, %s", err, stream.RecvMsg(nil))
			}
		}

		if _, err := stream.CloseAndRecv(); err != nil {
			return fmt.Errorf("final request failed: %s", err)
		}

		return nil
	})
}

func (c *client) streamFrom(ctx context.Context, src string, dst io.Writer) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		req := pb_agent.DownloadFileRequest{
			Path: src,
		}

		stream, err := grpcClient.FileSystem().DownloadFile(ctx, &req)
		if err != nil {
			return fmt.Errorf("cannot create new download stream: %s", err)
		}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}

				return fmt.Errorf("chunk recv failed: %s", err)
			}

			if _, err := dst.Write(resp.ChunkData); err != nil {
				return fmt.Errorf("chunk write failed: %s", err)
			}
		}

		return nil
	})
}

func (c *client) FreezeAll(ctx context.Context) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		_, err := grpcClient.FileSystem().Freeze(ctx, new(empty.Empty))

		return err
	})
}

func (c *client) UnfreezeAll(ctx context.Context) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		_, err := grpcClient.FileSystem().Unfreeze(ctx, new(empty.Empty))

		return err
	})
}
