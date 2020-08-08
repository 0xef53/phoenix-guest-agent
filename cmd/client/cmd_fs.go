package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"
)

func (c *Command) ShowFileStat(fname string, useLongFormat, withoutContent bool) error {
	req := pb.FileStatRequest{
		Path:           fname,
		WithDirContent: !withoutContent,
	}

	resp, err := c.client.GetFileStat(c.ctx, &req)
	if err != nil {
		return err
	}

	return printJSON(&resp)
}

func (c *Command) ShowFileMD5Hash(fname string) error {
	req := pb.FileRequest{
		Path: fname,
	}

	resp, err := c.client.GetFileMD5Hash(c.ctx, &req)
	if err != nil {
		return err
	}

	return printJSON(&resp)
}

func (c *Command) SetFileOwner(fname, owner string) error {
	parts := strings.SplitN(owner, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/group value: %s", owner)
	}

	if len(parts[1]) == 0 {
		parts[1] = parts[0]
	}

	req := pb.FileRequest{
		Path:  fname,
		Owner: parts[0],
		Group: parts[1],
	}

	if _, err := c.client.SetFileOwner(c.ctx, &req); err != nil {
		return err
	}

	return nil
}

func (c *Command) SetFileMode(fname, perm string) error {
	m, err := strconv.ParseUint(perm, 8, 32)
	if err != nil {
		return err
	}

	req := pb.FileRequest{
		Path: fname,
		Mode: uint32(m),
	}

	if _, err := c.client.SetFileMode(c.ctx, &req); err != nil {
		return err
	}

	return nil
}

func (c *Command) CreateDir(dirname, perm string) error {
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

	req := pb.FileRequest{
		Path: dirname,
		Mode: mode,
	}

	if _, err := c.client.CreateDir(c.ctx, &req); err != nil {
		return err
	}

	return nil
}

func (c *Command) CopyFile(srcname, dstname string) error {
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
		return c.streamTo(r, dst)
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

		tmpfile, err := ioutil.TempFile(filepath.Dir(dst), "."+filepath.Base(src)+".*")
		if err != nil {
			return err
		}
		defer tmpfile.Close()
		defer os.Remove(tmpfile.Name())
		if err := c.streamFrom(src, tmpfile); err != nil {
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

func (c *Command) ShowFileContent(fname string) error {
	return c.streamFrom(fname, os.Stdout)
}

func (c *Command) streamTo(src io.Reader, dst string) error {
	ctx, cancel := context.WithTimeout(c.ctx, time.Hour)
	defer cancel()

	stream, err := c.client.UploadFile(ctx)
	if err != nil {
		return fmt.Errorf("cannot create new stream: %s", err)
	}

	req := &pb.UploadFileRequest{
		Data: &pb.UploadFileRequest_Info{
			Info: &pb.UploadFileRequest_FileInfo{
				Path: dst,
			},
		},
	}
	if err := stream.Send(req); err != nil {
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

		req := &pb.UploadFileRequest{
			Data: &pb.UploadFileRequest_ChunkData{
				ChunkData: buffer[:n],
			},
		}

		if err := stream.Send(req); err != nil {
			return fmt.Errorf("chunk send failed: %s, %s", err, stream.RecvMsg(nil))
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		fmt.Errorf("final request failed: %s", err)
	}

	return nil
}

func (c *Command) streamFrom(src string, dst io.Writer) error {
	req := pb.FileRequest{
		Path: src,
	}

	stream, err := c.client.DownloadFile(c.ctx, &req)
	if err != nil {
		return fmt.Errorf("cannot create a new stream: %s", err)
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
}
