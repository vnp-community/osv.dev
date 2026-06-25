// Package crypto provides HMAC-SHA256 signing for audit event integrity.
package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// HMACSvc provides HMAC-SHA256 signing and verification.
// The key must be at least 32 bytes (from env OSV_AUDIT_HMAC_KEY).
type HMACSvc struct {
	key []byte
}

// NewHMACSvc creates a new HMACSvc with the given key.
func NewHMACSvc(key []byte) *HMACSvc {
	return &HMACSvc{key: key}
}

// Sign computes HMAC-SHA256 of data and returns the hex-encoded digest.
func (s *HMACSvc) Sign(data string) string {
	mac := hmac.New(sha256.New, s.key)
	io.WriteString(mac, data) //nolint:errcheck
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify returns true if signature matches the HMAC of data.
// Uses constant-time comparison to prevent timing attacks.
func (s *HMACSvc) Verify(data, signature string) bool {
	expected := s.Sign(data)
	return hmac.Equal([]byte(expected), []byte(signature))
}
