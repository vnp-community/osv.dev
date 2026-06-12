// Package grpcutil provides production-grade gRPC interceptor chains for all services.
package grpcutil

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ── Unary Interceptors ────────────────────────────────────────────────────────

// RecoveryInterceptor catches panics and returns InternalError.
func RecoveryInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("method", info.FullMethod).
					Str("stack", string(debug.Stack())).
					Msg("recovered from panic")
				err = fmt.Errorf("internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// LoggingInterceptor logs every gRPC call with duration and status code.
func LoggingInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		event := log.Info()
		if err != nil {
			event = log.Warn()
		}

		code := "OK"
		if s, ok := status.FromError(err); ok {
			code = s.Code().String()
		}

		event.
			Str("method", info.FullMethod).
			Str("code", code).
			Dur("duration_ms", duration).
			Err(err).
			Msg("gRPC call")

		return resp, err
	}
}

// TracingInterceptor creates OTel spans for each gRPC call.
func TracingInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(serviceName)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithAttributes(attribute.String("rpc.method", info.FullMethod)),
		)
		defer span.End()

		// Propagate request ID from metadata
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if ids := md.Get("x-request-id"); len(ids) > 0 {
				span.SetAttributes(attribute.String("request_id", ids[0]))
			}
		}

		resp, err := handler(ctx, req)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
		return resp, err
	}
}

// TimeoutInterceptor enforces a per-call deadline.
func TimeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Only apply if context has no deadline yet
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return handler(ctx, req)
	}
}

// RequestIDInterceptor injects a request ID into the context if not present.
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract from incoming metadata or generate
		if md, ok := metadata.FromIncomingContext(ctx); !ok || len(md.Get("x-request-id")) == 0 {
			requestID := generateRequestID()
			md = metadata.New(map[string]string{"x-request-id": requestID})
			ctx = metadata.NewIncomingContext(ctx, md)
		}
		return handler(ctx, req)
	}
}

// ── Server Options ────────────────────────────────────────────────────────────

// ServerOptions returns a standard gRPC server option set for production.
func ServerOptions(serviceName string, log zerolog.Logger, defaultTimeout time.Duration) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			RecoveryInterceptor(log),
			RequestIDInterceptor(),
			TracingInterceptor(serviceName),
			LoggingInterceptor(log),
			TimeoutInterceptor(defaultTimeout),
		),
		grpc.MaxRecvMsgSize(16 * 1024 * 1024), // 16MB max message
		grpc.MaxSendMsgSize(16 * 1024 * 1024),
	}
}

// ── Client Options ────────────────────────────────────────────────────────────

// ClientOptions returns standard gRPC client dial options for service-to-service calls.
func ClientOptions(serviceName string) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(
			clientTracingInterceptor(serviceName),
			clientLoggingInterceptor(),
		),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16 * 1024 * 1024),
		),
	}
}

func clientTracingInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	tracer := otel.Tracer(serviceName + "-client")
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, span := tracer.Start(ctx, "grpc.client."+method,
			trace.WithAttributes(attribute.String("rpc.target", cc.Target())),
		)
		defer span.End()
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
}

func clientLoggingInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		_ = start // caller logs if needed
		return err
	}
}

// generateRequestID creates a simple time-based request ID.
func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}
