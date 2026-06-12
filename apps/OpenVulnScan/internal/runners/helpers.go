// Package runners — helpers.go
// Shared gRPC interceptors và utilities cho tất cả service goroutines.
package runners

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcRecoveryInterceptor recover từ panic trong gRPC handler.
func grpcRecoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("method", info.FullMethod).
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("gRPC panic recovered")
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}

// grpcLoggingInterceptor log mỗi gRPC call.
func grpcLoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		log.Error().Str("method", info.FullMethod).Err(err).Msg("gRPC error")
	} else {
		log.Debug().Str("method", info.FullMethod).Msg("gRPC ok")
	}
	return resp, err
}

// wrapRunnerError wraps error với service name context.
func wrapRunnerError(svc string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", svc, err)
}
