package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	empty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *AgentServiceServer) GetFileMD5Hash(ctx context.Context, req *pb.FileRequest) (*pb.FileMD5Hash, error) {
	f, err := os.Open(req.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hash := md5.New()

	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}

	return &pb.FileMD5Hash{Hash: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (s *AgentServiceServer) GetFileStat(ctx context.Context, req *pb.FileStatRequest) (*pb.FileStatList, error) {
	info, err := os.Lstat(req.Path)
	if err != nil {
		return nil, err
	}

	_, uids, err := getOSUsers()
	if err != nil {
		return nil, err
	}

	_, gids, err := getOSGroups()
	if err != nil {
		return nil, err
	}

	getStat := func(fi os.FileInfo) *pb.FileStat {
		st := pb.FileStat{
			Name:      fi.Name(),
			Mode:      uint32(fi.Mode()),
			SizeBytes: fi.Size(),
			IsDir:     fi.IsDir(),
		}
		if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
			st.Owner = &pb.FileStat_Owner{Uid: sys.Uid}
			if v, ok := uids[sys.Uid]; ok {
				st.Owner.Name = v
			}
			st.Group = &pb.FileStat_Group{Gid: sys.Gid}
			if v, ok := gids[sys.Gid]; ok {
				st.Group.Name = v
			}
		}
		return &st
	}

	files := make([]*pb.FileStat, 0)

	if info.IsDir() && req.WithDirContent {
		dir, err := os.Open(req.Path)
		if err != nil {
			return nil, err
		}
		ffi, err := dir.Readdir(-1)
		if err != nil {
			return nil, err
		}
		for _, fi := range ffi {
			files = append(files, getStat(fi))
		}
	} else {
		files = append(files, getStat(info))
	}

	return &pb.FileStatList{Files: files}, nil
}

func getOSUsers() (map[string]uint32, map[uint32]string, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	names := make(map[string]uint32)
	uids := make(map[uint32]string)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// alice:x:1005:1006::/home/alice:/usr/bin/bash
		parts := strings.SplitN(scanner.Text(), ":", 7)
		if len(parts) < 6 || parts[0] == "" || parts[0][0] == '+' || parts[0][0] == '-' {
			continue
		}
		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, nil, err
		}
		names[parts[0]] = uint32(uid)
		uids[uint32(uid)] = parts[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return names, uids, nil
}

func getOSGroups() (map[string]uint32, map[uint32]string, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	names := make(map[string]uint32)
	gids := make(map[uint32]string)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// wheel:*:0:root
		parts := strings.SplitN(scanner.Text(), ":", 4)
		if len(parts) < 4 || parts[0] == "" || parts[0][0] == '+' || parts[0][0] == '-' {
			// If the file contains +foo and you search for "foo", glibc
			// returns an "invalid argument" error. Similarly, if you search
			// for a gid for a row where the group name starts with "+" or "-",
			// glibc fails to find the record.
			continue
		}

		gid, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, nil, err
		}
		names[parts[0]] = uint32(gid)
		gids[uint32(gid)] = parts[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return names, gids, nil
}

func (s *AgentServiceServer) SetFileOwner(ctx context.Context, req *pb.FileRequest) (*empty.Empty, error) {
	users, _, err := getOSUsers()
	if err != nil {
		return nil, err
	}

	groups, _, err := getOSGroups()
	if err != nil {
		return nil, err
	}

	var uid, gid int

	if v, ok := users[req.Owner]; ok {
		uid = int(v)
	} else {
		v, err := strconv.Atoi(req.Owner)
		if err != nil {
			return nil, fmt.Errorf("invalid user name/uid: %s", req.Owner)
		}
		uid = v
	}

	if v, ok := groups[req.Group]; ok {
		gid = int(v)
	} else {
		v, err := strconv.Atoi(req.Group)
		if err != nil {
			return nil, fmt.Errorf("invalid group name/gid: %s", req.Group)
		}
		gid = v
	}

	if err := os.Chown(req.Path, uid, gid); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *AgentServiceServer) SetFileMode(ctx context.Context, req *pb.FileRequest) (*empty.Empty, error) {
	oldmask := syscall.Umask(0000)
	defer syscall.Umask(oldmask)

	if err := os.Chmod(req.Path, os.FileMode(req.Mode)); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *AgentServiceServer) CreateDir(ctx context.Context, req *pb.FileRequest) (*empty.Empty, error) {
	oldmask := syscall.Umask(0000)
	defer syscall.Umask(oldmask)

	if err := os.MkdirAll(req.Path, os.FileMode(req.Mode)); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *AgentServiceServer) UploadFile(stream pb.AgentService_UploadFileServer) error {
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "cannot create a new stream: %s", err)
	}

	var fullname string

	if i := req.GetInfo(); i != nil {
		fullname = i.Path
	}

	if len(fullname) == 0 {
		return status.Errorf(codes.InvalidArgument, "file name is undefined")
	}

	tmpfile, err := ioutil.TempFile(filepath.Dir(fullname), "."+filepath.Base(fullname)+".*")
	if err != nil {
		return status.Errorf(codes.Internal, err.Error())
	}
	defer tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	var fileSize uint64

	for {
		switch stream.Context().Err() {
		case context.Canceled:
			return status.Error(codes.Canceled, "request is canceled")
		case context.DeadlineExceeded:
			return status.Error(codes.DeadlineExceeded, "deadline is exceeded")
		}

		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// no more data
				break
			}
			return status.Errorf(codes.Internal, "chunk recv failed: %s", err)
		}

		chunk := req.GetChunkData()
		if chunk == nil {
			return status.Errorf(codes.Internal, "unexpected: chunk is nil")
		}

		fileSize += uint64(len(chunk))

		if fileSize > 1<<31 { // 2Gb
			return status.Errorf(codes.InvalidArgument, "file is too large: %d > %d", fileSize, 1<<31)
		}

		if _, err := tmpfile.Write(chunk); err != nil {
			return status.Errorf(codes.Internal, "chunk write failed: %s", err)
		}
	}

	if err := tmpfile.Sync(); err != nil {
		return status.Errorf(codes.Internal, "file sync failed: %s", err)
	}
	if err := os.Rename(tmpfile.Name(), fullname); err != nil {
		return status.Errorf(codes.Internal, "rename temp file failed: %s", err)
	}

	return stream.SendAndClose(new(empty.Empty))
}

func (s *AgentServiceServer) DownloadFile(req *pb.FileRequest, stream pb.AgentService_DownloadFileServer) error {
	file, err := os.Open(req.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 2*1024*1024)

	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		resp := &pb.FileContent{
			ChunkData: buffer[:n],
		}
		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "chunk send failed: %s", err)
		}
	}

	return nil
}
