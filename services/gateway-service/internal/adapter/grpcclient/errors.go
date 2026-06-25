// Package grpcclient — errors.go
// Shared error helpers for gRPC client adapters.
package grpcclient

import (
	"errors"
	"fmt"
)

// ErrUnauthorized is returned when a token/key is invalid.
var ErrUnauthorized = errors.New("unauthorized")

// errUnauthorized wraps the server-provided error message.
func errUnauthorized(msg string) error {
	if msg == "" {
		return ErrUnauthorized
	}
	return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
}
