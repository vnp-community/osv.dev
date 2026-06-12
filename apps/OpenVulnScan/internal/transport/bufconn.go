// Package transport provides in-process gRPC transport via bufconn.
// Cho phép các service goroutines giao tiếp với nhau không qua network.
package transport

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const DefaultBufSize = 1 << 20 // 1MB

// NewBufConnListener tạo in-process gRPC listener.
// Mỗi service goroutine tạo 1 listener riêng.
func NewBufConnListener() *bufconn.Listener {
	return bufconn.Listen(DefaultBufSize)
}

// DialBufConn tạo gRPC ClientConn đến một bufconn listener.
// Dùng để connect từ API gateway hoặc service này đến service goroutine khác.
func DialBufConn(ctx context.Context, lis *bufconn.Listener) (*grpc.ClientConn, error) {
	//nolint:staticcheck // grpc.DialContext is deprecated but grpc.NewClient doesn't support WithBlock
	return grpc.DialContext(ctx, //nolint:depguard
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second), //nolint:staticcheck
	)
}

// MustDialBufConn panic nếu không kết nối được (dùng trong startup).
func MustDialBufConn(ctx context.Context, lis *bufconn.Listener) *grpc.ClientConn {
	conn, err := DialBufConn(ctx, lis)
	if err != nil {
		panic("transport.MustDialBufConn: " + err.Error())
	}
	return conn
}
