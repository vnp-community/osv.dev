// Package grpcutil — helper to export time.Second for external packages using BaseClient.
package grpcutil

import "time"

// Second returns 1 * time.Second for use in config multipliers.
func Second() time.Duration { return time.Second }
