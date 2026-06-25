// Package grpc provides shared gRPC client utilities for OSV platform services.
// All internal service-to-service gRPC communication should use these helpers
// for consistent connection, timeout, and keepalive behavior.
//
// This package is ADDITIVE — it does not modify any existing code in shared/pkg.
package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const defaultDialTimeout = 5 * time.Second

// DefaultDialOptions returns standard gRPC dial options for internal (in-cluster) communication.
// Uses insecure credentials — TLS is handled at the network layer (service mesh / VPN).
func DefaultDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}
}

// DialTimeout creates a gRPC client connection with a 5-second dial timeout.
// Extra options are appended after the defaults.
func DialTimeout(addr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()
	opts := append(DefaultDialOptions(), extraOpts...)
	conn, err := grpc.DialContext(ctx, addr, opts...) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return conn, nil
}
