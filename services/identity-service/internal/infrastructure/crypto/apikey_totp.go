package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new TOTP secret for the given account identifier.
// Returns:
//   - secret: base32-encoded secret (store encrypted)
//   - qrURL: otpauth:// URI for QR code display
func GenerateTOTPSecret(issuer, accountName string) (secret, qrURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		SecretSize:  20,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate TOTP key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks if the provided TOTP code is valid for the given secret.
// Accepts codes from the current, previous, and next 30-second windows.
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

const (
	apiKeyPrefix   = "ovs_"
	apiKeyBodyLen  = 32 // random bytes encoded as base58
	apiKeyPrefixLen = 12 // "ovs_" + 8 chars
)

// base58Chars is the character set for API key generation (URL-safe, no ambiguous chars).
var base58Chars = []byte("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")

// GenerateAPIKey creates a new API key.
// Returns:
//   - fullKey: the complete key (only returned once, never store)
//   - prefix: first 12 chars for display/lookup (e.g. "ovs_Ab3xYz9q")
//   - hash: hex(sha256(fullKey)) for DB storage
func GenerateAPIKey() (fullKey, prefix, hash string, err error) {
	body := make([]byte, apiKeyBodyLen)
	encoded := make([]byte, apiKeyBodyLen)

	for i := range body {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(base58Chars))))
		if err != nil {
			return "", "", "", fmt.Errorf("generate key byte: %w", err)
		}
		encoded[i] = base58Chars[n.Int64()]
	}

	fullKey = apiKeyPrefix + string(encoded)
	prefix = fullKey[:apiKeyPrefixLen]

	h := sha256.Sum256([]byte(fullKey))
	hash = hex.EncodeToString(h[:])

	return fullKey, prefix, hash, nil
}

// HashAPIKey computes the SHA-256 hash of an API key for lookup.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ValidateAPIKey verifies that a stored hash matches a provided key.
// Uses constant-time comparison.
func ValidateAPIKey(key, storedHash string) bool {
	computed := HashAPIKey(key)
	if len(computed) != len(storedHash) {
		return false
	}
	// constant-time string comparison
	diff := 0
	for i := range computed {
		if computed[i] != storedHash[i] {
			diff++
		}
	}
	return diff == 0
}
