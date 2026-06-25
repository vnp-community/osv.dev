package crypto

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "errors"
    "fmt"
    "strings"

    "golang.org/x/crypto/argon2"
)

// Argon2id parameters — OWASP 2024 recommendations
// Timing: ~200ms on standard hardware (intentional slowness)
const (
    argon2Memory      = 64 * 1024 // 64 MB
    argon2Iterations  = 3
    argon2Parallelism = 2
    argon2SaltLength  = 16
    argon2KeyLength   = 32
)

// PasswordHasher provides Argon2id password hashing
type PasswordHasher struct{}

// NewPasswordHasher creates a PasswordHasher
func NewPasswordHasher() *PasswordHasher {
    return &PasswordHasher{}
}

// Hash hashes a plaintext password using Argon2id.
// Returns a PHC format string:
//   $argon2id$v=19$m=65536,t=3,p=2$<base64_salt>$<base64_hash>
func (h *PasswordHasher) Hash(password string) (string, error) {
    if len(password) < 12 {
        return "", ErrPasswordTooShort
    }

    salt := make([]byte, argon2SaltLength)
    if _, err := rand.Read(salt); err != nil {
        return "", fmt.Errorf("generate salt: %w", err)
    }

    hash := argon2.IDKey(
        []byte(password),
        salt,
        argon2Iterations,
        argon2Memory,
        argon2Parallelism,
        argon2KeyLength,
    )

    b64Salt := base64.RawStdEncoding.EncodeToString(salt)
    b64Hash := base64.RawStdEncoding.EncodeToString(hash)

    return fmt.Sprintf(
        "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version,
        argon2Memory,
        argon2Iterations,
        argon2Parallelism,
        b64Salt,
        b64Hash,
    ), nil
}

// Verify checks a plaintext password against a PHC-format hash.
// Uses constant-time comparison to prevent timing attacks.
func (h *PasswordHasher) Verify(password, encoded string) (bool, error) {
    params, salt, storedHash, err := decodeArgon2Hash(encoded)
    if err != nil {
        return false, fmt.Errorf("decode hash: %w", err)
    }

    computed := argon2.IDKey(
        []byte(password),
        salt,
        params.iterations,
        params.memory,
        params.parallelism,
        params.keyLength,
    )

    // Constant-time comparison prevents timing side-channel attacks
    return subtle.ConstantTimeCompare(storedHash, computed) == 1, nil
}

type argon2Params struct {
    memory      uint32
    iterations  uint32
    parallelism uint8
    keyLength   uint32
}

func decodeArgon2Hash(encoded string) (*argon2Params, []byte, []byte, error) {
    parts := strings.Split(encoded, "$")
    if len(parts) != 6 {
        return nil, nil, nil, errors.New("invalid hash format: expected 6 parts")
    }

    if parts[1] != "argon2id" {
        return nil, nil, nil, errors.New("invalid algorithm: expected argon2id")
    }

    var version int
    if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
        return nil, nil, nil, fmt.Errorf("parse version: %w", err)
    }

    var p argon2Params
    if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d",
        &p.memory, &p.iterations, &p.parallelism); err != nil {
        return nil, nil, nil, fmt.Errorf("parse params: %w", err)
    }

    salt, err := base64.RawStdEncoding.DecodeString(parts[4])
    if err != nil {
        return nil, nil, nil, fmt.Errorf("decode salt: %w", err)
    }

    hash, err := base64.RawStdEncoding.DecodeString(parts[5])
    if err != nil {
        return nil, nil, nil, fmt.Errorf("decode hash: %w", err)
    }

    p.keyLength = uint32(len(hash))
    return &p, salt, hash, nil
}

var ErrPasswordTooShort = errors.New("password must be at least 12 characters")
