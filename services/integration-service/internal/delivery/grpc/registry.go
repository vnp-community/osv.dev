// Package grpc provides the integration registry pattern for future integrations.
// Add new integrations by implementing the Integration interface and registering here.
package grpc

import (
	"context"
)

// Event represents a domain event dispatched to integrations.
type Event struct {
	Type    string
	Payload map[string]interface{}
}

// Integration is the interface all external integrations must implement.
type Integration interface {
	Name() string
	HandleEvent(ctx context.Context, event Event) error
}

// Registry holds all registered integrations.
type Registry struct {
	integrations map[string]Integration
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{integrations: make(map[string]Integration)}
}

// Register adds an integration to the registry.
func (r *Registry) Register(i Integration) {
	r.integrations[i.Name()] = i
}

// Dispatch routes an event to all registered integrations.
func (r *Registry) Dispatch(ctx context.Context, event Event) []error {
	var errs []error
	for _, i := range r.integrations {
		if err := i.HandleEvent(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
