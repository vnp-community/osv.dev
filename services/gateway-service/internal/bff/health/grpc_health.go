// Package health provides gRPC upstream health check utilities for gateway-service.
// Uses connection-level probing (no grpc_health_v1 protocol required from upstreams).
package health

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCCheckResult is the health result for a single gRPC upstream.
type GRPCCheckResult struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	Status    string `json:"status"`    // "healthy" | "unhealthy" | "unreachable"
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// PingGRPC dials a gRPC target and checks its connectivity state.
// Returns a result within timeout (3 seconds).
// [FIX TASK-HC-004] Real gRPC health check — no longer stubbed.
func PingGRPC(ctx context.Context, name, target string) GRPCCheckResult {
	start := time.Now()
	result := GRPCCheckResult{Name: name, Target: target}

	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext( //nolint:staticcheck
		dialCtx,
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true), //nolint:staticcheck
	)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "unreachable"
		result.Error = fmt.Sprintf("dial: %v", err)
		return result
	}
	defer conn.Close() //nolint:errcheck

	state := conn.GetState()
	if state == connectivity.Ready || state == connectivity.Idle {
		result.Status = "healthy"
	} else {
		result.Status = "unhealthy"
		result.Error = state.String()
	}
	return result
}

// PingAll pings multiple gRPC upstreams concurrently and returns their statuses.
func PingAll(ctx context.Context, upstreams map[string]string) []GRPCCheckResult {
	if len(upstreams) == 0 {
		return nil
	}
	ch := make(chan GRPCCheckResult, len(upstreams))
	for name, addr := range upstreams {
		go func(n, a string) {
			ch <- PingGRPC(ctx, n, a)
		}(name, addr)
	}
	results := make([]GRPCCheckResult, 0, len(upstreams))
	for range upstreams {
		results = append(results, <-ch)
	}
	return results
}

// IsAllHealthy returns true when all gRPC checks are healthy.
func IsAllHealthy(results []GRPCCheckResult) bool {
	for _, r := range results {
		if r.Status != "healthy" {
			return false
		}
	}
	return true
}
