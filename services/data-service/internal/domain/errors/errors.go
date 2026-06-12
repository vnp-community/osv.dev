// Package errors defines domain-level sentinel errors for the KEV service.
package errors

import "errors"

// Sentinel errors for the KEV domain.
var (
	// ErrKEVNotFound is returned when a KEV entry cannot be found by CVE ID.
	ErrKEVNotFound = errors.New("kev entry not found")

	// ErrInvalidCVEID is returned when a supplied CVE ID has invalid format.
	ErrInvalidCVEID = errors.New("invalid CVE ID format")

	// ErrSyncFailed is returned when the CISA catalog fetch fails.
	ErrSyncFailed = errors.New("KEV sync failed")

	// ErrEmptyCVEIDs is returned when a bulk check is called with no IDs.
	ErrEmptyCVEIDs = errors.New("no CVE IDs provided")

	// ErrTooManyCVEIDs is returned when bulk check exceeds the 500 ID limit.
	ErrTooManyCVEIDs = errors.New("too many CVE IDs (max 500)")
)
