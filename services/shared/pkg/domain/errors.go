// Package domain — shared sentinel errors for cve-search services.
// Import as: "github.com/osv/shared/pkg/domain"
// Usage: fmt.Errorf("cve %s: %w", id, domain.ErrNotFound)
package domain

import "errors"

// Sentinel errors for cve-search services.
// Wrap with fmt.Errorf to add context while preserving errors.Is() detection.
var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrInvalidInput is returned when the request parameters are invalid.
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized is returned when the caller is not authenticated.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when the caller lacks permission.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict is returned when a resource already exists.
	ErrConflict = errors.New("conflict")

	// ErrInternal is returned for unexpected internal failures.
	ErrInternal = errors.New("internal error")
)
