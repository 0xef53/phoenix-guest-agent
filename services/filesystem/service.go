package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/0xef53/phoenix-guest-agent/services"

	pb "github.com/0xef53/phoenix-guest-agent/api/services/agent/v2"

	grpcserver "github.com/0xef53/go-grpc/server"

	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	empty "github.com/golang/protobuf/ptypes/empty"
)

var _ = pb.AgentFileSystemServiceServer(new(Service))

func init() {
	grpcserver.Register(new(Service), grpcserver.WithServiceBucket("pga"))
}

type Service struct {
	*services.ServiceServer
}

func (s *Service) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *Service) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *Service) RegisterGRPC(server *grpc.Server) {
	pb.RegisterAgentFileSystemServiceServer(server, s)
}

func (s *Service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}

func (s *Service) Sync(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	err := s.ServiceServer.SyncFileSystems(ctx)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) Freeze(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	err := s.ServiceServer.FreezeFileSystems(ctx)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) Unfreeze(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	err := s.ServiceServer.UnfreezeFileSystems(ctx)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) GetFileMD5Hash(ctx context.Context, req *pb.GetFileMD5HashRequest) (*pb.GetFileMD5HashResponse, error) {
	hash, err := s.ServiceServer.GetFileMD5Hash(ctx, req.Path)
	if err != nil {
		return nil, err
	}

	return &pb.GetFileMD5HashResponse{Hash: hash}, nil
}

func (s *Service) GetFileStat(ctx context.Context, req *pb.GetFileStatRequest) (*pb.GetFileStatResponse, error) {
	fstats, err := s.ServiceServer.GetFileStat(ctx, req.Path, req.WithDirContent)
	if err != nil {
		return nil, err
	}

	return &pb.GetFileStatResponse{Files: fileStatsToProto(fstats)}, nil
}

func (s *Service) SetFileOwner(ctx context.Context, req *pb.SetFileOwnerRequest) (*empty.Empty, error) {
	err := s.ServiceServer.SetFileOwner(ctx, req.Path, req.Owner, req.Group)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) SetFileMode(ctx context.Context, req *pb.SetFileModeRequest) (*empty.Empty, error) {
	err := s.ServiceServer.SetFileMode(ctx, req.Path, os.FileMode(req.Mode))
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) CreateDir(ctx context.Context, req *pb.CreateDirRequest) (*empty.Empty, error) {
	err := s.ServiceServer.CreateDir(ctx, req.Path, os.FileMode(req.Mode))
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *Service) UploadFile(stream pb.AgentFileSystemService_UploadFileServer) error {
	req, err := stream.Recv()
	if err != nil {
		return grpc_status.Errorf(grpc_codes.Internal, "cannot create a new stream: %s", err)
	}

	var fullname string

	if v := req.GetInfo(); v != nil {
		fullname = v.Path
	}

	if len(fullname) == 0 {
		return grpc_status.Errorf(grpc_codes.InvalidArgument, "file name is undefined")
	}

	tmpfile, err := os.CreateTemp(filepath.Dir(fullname), "."+filepath.Base(fullname)+".*")
	if err != nil {
		return grpc_status.Errorf(grpc_codes.Internal, "%s", err.Error())
	}
	defer tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	var fileSize uint64

	for {
		switch stream.Context().Err() {
		case context.Canceled:
			return grpc_status.Error(grpc_codes.Canceled, "request is canceled")
		case context.DeadlineExceeded:
			return grpc_status.Error(grpc_codes.DeadlineExceeded, "deadline is exceeded")
		}

		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// no more data
				break
			}

			return grpc_status.Errorf(grpc_codes.Internal, "chunk recv failed: %s", err)
		}

		chunk := req.GetChunkData()
		if chunk == nil {
			return grpc_status.Errorf(grpc_codes.Internal, "unexpected: chunk is nil")
		}

		fileSize += uint64(len(chunk))

		if fileSize > 1<<31 { // 2Gb
			return grpc_status.Errorf(grpc_codes.InvalidArgument, "file is too large: %d > %d", fileSize, 1<<31)
		}

		if _, err := tmpfile.Write(chunk); err != nil {
			return grpc_status.Errorf(grpc_codes.Internal, "chunk write failed: %s", err)
		}
	}

	if err := tmpfile.Sync(); err != nil {
		return grpc_status.Errorf(grpc_codes.Internal, "file sync failed: %s", err)
	}

	if err := os.Rename(tmpfile.Name(), fullname); err != nil {
		return grpc_status.Errorf(grpc_codes.Internal, "rename temp file failed: %s", err)
	}

	return stream.SendAndClose(new(empty.Empty))
}

func (s *Service) DownloadFile(req *pb.DownloadFileRequest, stream pb.AgentFileSystemService_DownloadFileServer) error {
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
			return grpc_status.Errorf(grpc_codes.Internal, "chunk send failed: %s", err)
		}
	}

	return nil
}
