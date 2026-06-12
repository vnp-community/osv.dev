// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package logging provides zerolog-based gRPC interceptors.
package logging

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that logs
// each RPC call with method name, status code, and duration.
func UnaryServerInterceptor(logger *zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		l := logger
		if l == nil {
			global := log.Logger
			l = &global
		}

		code := status.Code(err)
		event := l.Info()
		if err != nil {
			event = l.Error().Err(err)
		}
		event.
			Str("method", info.FullMethod).
			Str("status", code.String()).
			Dur("duration_ms", duration).
			Msg("grpc request")

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor for logging.
func StreamServerInterceptor(logger *zerolog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		l := logger
		if l == nil {
			global := log.Logger
			l = &global
		}

		code := status.Code(err)
		event := l.Info()
		if err != nil {
			event = l.Error().Err(err)
		}
		event.
			Str("method", info.FullMethod).
			Str("status", code.String()).
			Dur("duration_ms", duration).
			Bool("is_client_stream", info.IsClientStream).
			Bool("is_server_stream", info.IsServerStream).
			Msg("grpc stream")

		return err
	}
}
