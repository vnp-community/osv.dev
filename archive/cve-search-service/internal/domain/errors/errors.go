// Package errors defines domain sentinel errors for the CVE search service.
package errors

import "errors"

var (
	ErrCVENotFound  = errors.New("CVE not found")
	ErrInvalidCVEID = errors.New("invalid CVE ID format")
	ErrSearchFailed = errors.New("search failed")
	ErrCacheMiss    = errors.New("cache miss")
)
