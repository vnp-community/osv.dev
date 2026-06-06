// Package health provides health check implementations for the DefectDojo API Gateway.
package health

import (
	"context"
	"errors"

	nats "github.com/nats-io/nats.go"
)

// NATSCheck verifies NATS connectivity for the gateway health endpoint.
type NATSCheck struct {
	nc *nats.Conn
}

// NewNATSCheck creates a new NATS health checker.
func NewNATSCheck(nc *nats.Conn) *NATSCheck {
	return &NATSCheck{nc: nc}
}

// Check returns nil if NATS is connected, a descriptive error otherwise.
func (c *NATSCheck) Check(_ context.Context) error {
	if c.nc == nil {
		return errors.New("nats: connection is nil")
	}
	if !c.nc.IsConnected() {
		return errors.New("nats: not connected")
	}
	return nil
}

// Name returns the check identifier used in the health response JSON.
func (c *NATSCheck) Name() string { return "nats" }
