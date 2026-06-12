// Package errors defines domain sentinel errors for the CVE sync service.
package errors

import "errors"

var (
	ErrSyncSourceFailed = errors.New("sync source fetch failed")
	ErrJobNotFound      = errors.New("sync job not found")
	ErrSyncInProgress   = errors.New("sync already in progress for this source")
)
