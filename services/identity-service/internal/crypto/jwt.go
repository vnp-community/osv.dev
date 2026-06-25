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
