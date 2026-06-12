// Package pgp provides PGP signing and verification utilities.
// This is a pure-stdlib implementation that uses only crypto/... packages
// from the Go standard library to avoid external dependency issues.
//
// Supported operations:
//   - SHA-256 hash-based signature (HMAC-SHA256) for data integrity
//   - Placeholder for full PGP verification (requires ProtonMail/go-crypto)
//
// For full PGP-compatible signature verification (NVD data), integrate
// ProtonMail/go-crypto separately in the service that needs it.
package pgp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// DefaultPublicKey is a placeholder. Supply the actual NVD PGP public key
// from https://nvd.nist.gov/general/nvd-gpg-key in production.
const DefaultPublicKey = ""

// ErrNotImplemented is returned by operations that require a full PGP library.
var ErrNotImplemented = errors.New("pgp: full PGP operations require ProtonMail/go-crypto; use the sha256 HMAC functions for basic integrity")

// Verifier checks PGP signatures against a trusted key ring.
// This implementation uses SHA-256 HMAC for internal use.
// For real NVD PGP signature verification, use VerifyDetached.
type Verifier struct {
	publicKeyPEM string
}

// NewVerifier creates a Verifier with PGP public keys from armored text.
// Note: For workspace environments where ProtonMail/go-crypto is unavailable,
// this returns a verifier that only supports HMAC-SHA256 verification.
func NewVerifier(armoredPublicKeys string) (*Verifier, error) {
	if armoredPublicKeys == "" {
		return nil, fmt.Errorf("pgp: empty public key")
	}
	return &Verifier{publicKeyPEM: armoredPublicKeys}, nil
}

// Verify checks the PGP signature of data.
// NOTE: This is a STUB. Real PGP verification requires ProtonMail/go-crypto.
// Returns ErrNotImplemented for actual PGP signatures.
// For HMAC-SHA256 signatures (created by Signer.Sign), use VerifyHMAC instead.
func (v *Verifier) Verify(data, signature []byte) error {
	return fmt.Errorf("%w: use ProtonMail/go-crypto for PGP verification", ErrNotImplemented)
}

// VerifyReaders checks a PGP signature using io.Reader inputs.
// NOTE: This is a STUB. Returns ErrNotImplemented.
func (v *Verifier) VerifyReaders(data, signature interface{}) error {
	return fmt.Errorf("%w: use ProtonMail/go-crypto for PGP verification", ErrNotImplemented)
}

// Signer creates signatures for data integrity.
// Uses HMAC-SHA256 for internal compatibility.
type Signer struct {
	secret []byte
}

// NewSigner creates a Signer from an armored PGP private key.
// NOTE: This stub uses the key as HMAC secret (sha256 of key bytes).
func NewSigner(armoredPrivateKey string) (*Signer, error) {
	if armoredPrivateKey == "" {
		return nil, fmt.Errorf("pgp: empty private key")
	}
	h := sha256.Sum256([]byte(armoredPrivateKey))
	return &Signer{secret: h[:]}, nil
}

// Sign creates an HMAC-SHA256 signature of data.
// Note: This is NOT a PGP signature. For real PGP signing, use ProtonMail/go-crypto.
func (s *Signer) Sign(data []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(data)
	return mac.Sum(nil), nil
}

// SignArmored creates a hex-encoded HMAC-SHA256 signature (pseudo-armored).
// Format: "HMAC-SHA256:<hex>"
func (s *Signer) SignArmored(data []byte) ([]byte, error) {
	sig, err := s.Sign(data)
	if err != nil {
		return nil, err
	}
	armored := "HMAC-SHA256:" + hex.EncodeToString(sig)
	return []byte(armored), nil
}

// VerifyHMAC verifies an HMAC-SHA256 signature created by Signer.Sign.
func VerifyHMAC(data, signature, secret []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, signature)
}

// LoadKeyRing is a stub that returns an error.
// Implement using ProtonMail/go-crypto if full PGP is needed.
func LoadKeyRing(armoredPublicKeys string) (interface{}, error) {
	return nil, fmt.Errorf("%w: use ProtonMail/go-crypto for LoadKeyRing", ErrNotImplemented)
}

// SHA256Checksum computes the SHA-256 checksum of data.
// Useful for verifying NVD download integrity via .sha256 files.
func SHA256Checksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// VerifySHA256 checks that data matches the expected SHA-256 hex string.
func VerifySHA256(data []byte, expectedHex string) error {
	got := SHA256Checksum(data)
	if got != expectedHex {
		return fmt.Errorf("pgp: sha256 mismatch: got %s, want %s", got, expectedHex)
	}
	return nil
}
