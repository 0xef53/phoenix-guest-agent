package interceptors

import (
	"context"
	"errors"
	"io/fs"

	"github.com/0xef53/phoenix-guest-agent/core"

	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func MapErrorsUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		switch grpc_status.Code(err) {
		case grpc_codes.NotFound:
			return nil, err
		}

		var code grpc_codes.Code

		switch {
		case errors.Is(err, fs.ErrNotExist):
			code = grpc_codes.NotFound
		case errors.Is(err, core.ErrNotReadyNow):
			code = grpc_codes.NotFound
		default:
			code = grpc_codes.Internal
		}

		return nil, grpc_status.Error(code, err.Error())
	}
}
