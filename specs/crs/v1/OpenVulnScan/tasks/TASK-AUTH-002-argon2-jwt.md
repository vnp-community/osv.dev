# TASK-AUTH-002 — Argon2id Password Hashing + RS256 JWT Manager

| Field | Value |
|-------|-------|
| **Task ID** | T-AUTH-002 |
| **Service** | `identity-service` |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-003 §5 Argon2id, §3 JWT Design |
| **Priority** | 🔴 Critical |
| **Depends On** | T-AUTH-001 |
| **Estimated** | 3h |

---

## Context

Task này implement 2 cryptographic primitives cốt lõi:

1. **Argon2id hasher** — hash passwords với memory-hard algorithm (OWASP recommended)
2. **JWT RS256 manager** — sign/parse JWT tokens bằng RSA private/public key pair

Đây là foundation cho login, registration, và token validation.

---

## Goal

- Argon2id: Hash password + verify với PHC format string
- JWT: Sign claims → JWT string; Parse JWT string → claims struct
- JWKS endpoint support (public key exposure cho services khác)

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/identity-service/internal/crypto/argon2.go` |
| CREATE | `services/identity-service/internal/crypto/argon2_test.go` |
| CREATE | `services/identity-service/internal/crypto/jwt.go` |
| CREATE | `services/identity-service/internal/crypto/jwt_test.go` |
| CREATE | `services/identity-service/internal/crypto/keygen/generate_keys.go` |

---

## Implementation

### File 1: `services/identity-service/internal/crypto/argon2.go`

```go
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
```

### File 2: `services/identity-service/internal/crypto/argon2_test.go`

```go
package crypto

import (
    "strings"
    "testing"
)

func TestPasswordHasher_Hash(t *testing.T) {
    h := NewPasswordHasher()

    hash, err := h.Hash("securePassword123!")
    if err != nil {
        t.Fatalf("Hash() error: %v", err)
    }
    if !strings.HasPrefix(hash, "$argon2id$") {
        t.Errorf("Hash() should start with $argon2id$, got: %s", hash)
    }
}

func TestPasswordHasher_TooShort(t *testing.T) {
    h := NewPasswordHasher()
    _, err := h.Hash("short")
    if err != ErrPasswordTooShort {
        t.Errorf("expected ErrPasswordTooShort, got: %v", err)
    }
}

func TestPasswordHasher_Verify_Correct(t *testing.T) {
    h := NewPasswordHasher()
    password := "securePassword123!"
    hash, _ := h.Hash(password)

    ok, err := h.Verify(password, hash)
    if err != nil {
        t.Fatalf("Verify() error: %v", err)
    }
    if !ok {
        t.Error("Verify() should return true for correct password")
    }
}

func TestPasswordHasher_Verify_Wrong(t *testing.T) {
    h := NewPasswordHasher()
    hash, _ := h.Hash("securePassword123!")

    ok, err := h.Verify("wrongPassword123!", hash)
    if err != nil {
        t.Fatalf("Verify() error: %v", err)
    }
    if ok {
        t.Error("Verify() should return false for wrong password")
    }
}

func TestPasswordHasher_UniqueHashes(t *testing.T) {
    h := NewPasswordHasher()
    password := "securePassword123!"
    hash1, _ := h.Hash(password)
    hash2, _ := h.Hash(password)

    if hash1 == hash2 {
        t.Error("Hash() should produce different hashes (unique salt) each time")
    }
}
```

### File 3: `services/identity-service/internal/crypto/jwt.go`

```go
package crypto

import (
    "crypto/rsa"
    "errors"
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

// JWTClaims represents the payload of an access token
type JWTClaims struct {
    jwt.RegisteredClaims
    Role        string   `json:"role"`
    Permissions []string `json:"permissions"`
}

// JWTManager handles RS256 JWT signing and verification
type JWTManager struct {
    privateKey *rsa.PrivateKey
    publicKey  *rsa.PublicKey
    keyID      string        // Key ID for JWKS rotation (e.g., "key-2026-06")
    issuer     string
    audience   string
    accessTTL  time.Duration
}

// NewJWTManager loads RSA keys from PEM files and creates a manager
func NewJWTManager(privateKeyPath, publicKeyPath, keyID, issuer, audience string, accessTTL time.Duration) (*JWTManager, error) {
    // Load private key (sign only — stays in auth-service)
    privPEM, err := os.ReadFile(privateKeyPath)
    if err != nil {
        return nil, fmt.Errorf("read private key: %w", err)
    }
    privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privPEM)
    if err != nil {
        return nil, fmt.Errorf("parse private key: %w", err)
    }

    // Load public key (for local verification)
    pubPEM, err := os.ReadFile(publicKeyPath)
    if err != nil {
        return nil, fmt.Errorf("read public key: %w", err)
    }
    publicKey, err := jwt.ParseRSAPublicKeyFromPEM(pubPEM)
    if err != nil {
        return nil, fmt.Errorf("parse public key: %w", err)
    }

    return &JWTManager{
        privateKey: privateKey,
        publicKey:  publicKey,
        keyID:      keyID,
        issuer:     issuer,
        audience:   audience,
        accessTTL:  accessTTL,
    }, nil
}

// Sign creates a signed JWT access token for the given user
func (m *JWTManager) Sign(userID, role string, permissions []string) (tokenString, jti string, expiresAt time.Time, err error) {
    jtiUUID := uuid.New()
    jti = jtiUUID.String()
    now := time.Now().UTC()
    expiresAt = now.Add(m.accessTTL)

    claims := JWTClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            Issuer:    m.issuer,
            Audience:  jwt.ClaimStrings{m.audience},
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(expiresAt),
            ID:        jti,
        },
        Role:        role,
        Permissions: permissions,
    }

    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    token.Header["kid"] = m.keyID

    tokenString, err = token.SignedString(m.privateKey)
    if err != nil {
        return "", "", time.Time{}, fmt.Errorf("sign token: %w", err)
    }

    return tokenString, jti, expiresAt, nil
}

// Parse validates and parses a JWT access token
// Returns the claims if valid, or an error if invalid/expired
func (m *JWTManager) Parse(tokenString string) (*JWTClaims, error) {
    token, err := jwt.ParseWithClaims(
        tokenString,
        &JWTClaims{},
        func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return m.publicKey, nil
        },
        jwt.WithIssuer(m.issuer),
        jwt.WithAudience(m.audience),
        jwt.WithExpirationRequired(),
    )
    if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
            return nil, ErrTokenExpired
        }
        return nil, ErrTokenInvalid
    }

    claims, ok := token.Claims.(*JWTClaims)
    if !ok || !token.Valid {
        return nil, ErrTokenInvalid
    }

    return claims, nil
}

// PublicKey returns the RSA public key (for JWKS endpoint)
func (m *JWTManager) PublicKey() *rsa.PublicKey {
    return m.publicKey
}

// KeyID returns the current key ID
func (m *JWTManager) KeyID() string {
    return m.keyID
}

var (
    ErrTokenExpired = errors.New("token has expired")
    ErrTokenInvalid = errors.New("token is invalid")
)
```

### File 4: `services/identity-service/internal/crypto/jwt_test.go`

```go
package crypto

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func setupJWTManager(t *testing.T) *JWTManager {
    t.Helper()

    // Generate test RSA key pair
    privKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        t.Fatalf("generate RSA key: %v", err)
    }

    dir := t.TempDir()
    privPath := filepath.Join(dir, "private.pem")
    pubPath := filepath.Join(dir, "public.pem")

    // Write private key
    privPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(privKey),
    })
    os.WriteFile(privPath, privPEM, 0600)

    // Write public key
    pubBytes, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
    pubPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: pubBytes,
    })
    os.WriteFile(pubPath, pubPEM, 0644)

    mgr, err := NewJWTManager(privPath, pubPath, "test-key", "auth-service", "api", 15*time.Minute)
    if err != nil {
        t.Fatalf("NewJWTManager: %v", err)
    }
    return mgr
}

func TestJWT_SignAndParse(t *testing.T) {
    mgr := setupJWTManager(t)

    tokenStr, jti, exp, err := mgr.Sign("user-123", "user", []string{"scan:read"})
    if err != nil {
        t.Fatalf("Sign(): %v", err)
    }
    if tokenStr == "" || jti == "" {
        t.Error("Sign() returned empty values")
    }
    if exp.Before(time.Now()) {
        t.Error("ExpiresAt should be in the future")
    }

    claims, err := mgr.Parse(tokenStr)
    if err != nil {
        t.Fatalf("Parse(): %v", err)
    }
    if claims.Subject != "user-123" {
        t.Errorf("Subject = %s, want user-123", claims.Subject)
    }
    if claims.Role != "user" {
        t.Errorf("Role = %s, want user", claims.Role)
    }
    if claims.ID != jti {
        t.Errorf("JTI mismatch: %s != %s", claims.ID, jti)
    }
}

func TestJWT_InvalidToken(t *testing.T) {
    mgr := setupJWTManager(t)
    _, err := mgr.Parse("not.a.valid.token")
    if err != ErrTokenInvalid {
        t.Errorf("Expected ErrTokenInvalid, got: %v", err)
    }
}
```

### File 5: `services/identity-service/internal/crypto/keygen/generate_keys.go`

```go
// Package keygen provides a utility to generate RSA key pairs for JWT signing.
// Usage: go run ./internal/crypto/keygen/generate_keys.go
// Output: jwt_private.pem, jwt_public.pem in current directory
package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "os"
)

func main() {
    fmt.Println("Generating 4096-bit RSA key pair for JWT RS256...")

    privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
        os.Exit(1)
    }

    // Save private key
    privFile, _ := os.OpenFile("jwt_private.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    pem.Encode(privFile, &pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
    })
    privFile.Close()
    fmt.Println("✓ Saved jwt_private.pem (keep secret, only in auth-service)")

    // Save public key
    pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
    pubFile, _ := os.OpenFile("jwt_public.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
    pem.Encode(pubFile, &pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: pubBytes,
    })
    pubFile.Close()
    fmt.Println("✓ Saved jwt_public.pem (share with all services)")
    fmt.Println("\nMount these into containers via Docker secrets or Kubernetes secrets.")
}
```

---

## Dependencies to Add

Kiểm tra `services/identity-service/go.mod`. Nếu thiếu, thêm:

```bash
cd services/identity-service
go get golang.org/x/crypto@latest
go get github.com/golang-jwt/jwt/v5@latest
go get github.com/google/uuid@latest
```

---

## Verification

```bash
cd services/identity-service
go test ./internal/crypto/... -v
```

**Expected output**:
```
--- PASS: TestPasswordHasher_Hash
--- PASS: TestPasswordHasher_TooShort
--- PASS: TestPasswordHasher_Verify_Correct
--- PASS: TestPasswordHasher_Verify_Wrong
--- PASS: TestPasswordHasher_UniqueHashes
--- PASS: TestJWT_SignAndParse
--- PASS: TestJWT_InvalidToken
```

```bash
go build ./...
```

**Expected**: No errors.
