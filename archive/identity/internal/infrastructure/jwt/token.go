// Package jwt provides RS256 JWT generation and validation for the auth service.
// Tokens are signed with a 4096-bit RSA private key; public key is published via JWKS.
package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/defectdojo/identity/internal/domain/entity"
	"github.com/defectdojo/identity/internal/domain/valueobject"
)

// Claims is the JWT payload for OpenVulnScan access tokens.
type Claims struct {
	jwtlib.RegisteredClaims

	// UserID is the authenticated user's UUID.
	UserID string `json:"uid"`

	// Role is the user's primary role.
	Role string `json:"role"`

	// Permissions is the flat list of permissions derived from the role.
	Permissions []string `json:"perms"`
}

// ExpiryTime returns the token's expiry as a time.Time (nil-safe).
func (c *Claims) ExpiryTime() time.Time {
	if c.RegisteredClaims.ExpiresAt == nil {
		return time.Time{}
	}
	return c.RegisteredClaims.ExpiresAt.Time
}

// Config holds JWT service configuration.
type Config struct {
	// PrivateKeyPath is the path to the PEM-encoded RSA private key file.
	PrivateKeyPath string

	// Issuer is the "iss" claim, e.g. "https://auth.openvulnscan.io".
	Issuer string

	// Audience is the list of valid "aud" claims.
	Audience []string

	// AccessTokenTTL is the access token lifetime (default 15m).
	AccessTokenTTL time.Duration

	// RefreshTokenTTL is the refresh token lifetime (default 7d).
	RefreshTokenTTL time.Duration
}

// Service handles JWT signing and validation.
type Service struct {
	privateKey *rsa.PrivateKey
	cfg        Config
}

// NewService loads the RSA private key from disk and creates a JWT service.
func NewService(cfg Config) (*Service, error) {
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = 7 * 24 * time.Hour
	}

	keyData, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key file %q: %w", cfg.PrivateKeyPath, err)
	}

	key, err := jwtlib.ParseRSAPrivateKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}

	return &Service{privateKey: key, cfg: cfg}, nil
}

// GenerateAccessToken creates a signed RS256 JWT access token for the given user.
// Returns the signed token string and the JTI (JWT ID) for blacklisting.
func (s *Service) GenerateAccessToken(user *entity.User) (tokenStr, jti string, err error) {
	jtiID := uuid.New().String()
	now := time.Now()

	claims := &Claims{
		RegisteredClaims: jwtlib.RegisteredClaims{
			ID:        jtiID,
			Subject:   user.ID.String(),
			Issuer:    s.cfg.Issuer,
			Audience:  s.cfg.Audience,
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(s.cfg.AccessTokenTTL)),
		},
		UserID:      user.ID.String(),
		Role:        user.Role,
		Permissions: valueobject.PermissionsFor(user.Role),
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, jtiID, nil
}

// GenerateRefreshToken generates a cryptographically random refresh token.
// The returned token is the raw value (must be hashed before DB storage).
func (s *Service) GenerateRefreshToken() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// ValidateToken parses and validates a signed JWT string.
// Only performs RSA signature verification + claims validation.
// JTI blacklist check must be done separately via Redis.
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtlib.Token) (any, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &s.privateKey.PublicKey, nil
	}, jwtlib.WithIssuer(s.cfg.Issuer))

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// PublicKeyJWKS returns the RSA public key as a JWKS JSON document.
// Served at /.well-known/jwks.json for external JWT validators.
func (s *Service) PublicKeyJWKS() ([]byte, error) {
	pub := &s.privateKey.PublicKey
	n := pub.N.Bytes()
	e := big.NewInt(int64(pub.E)).Bytes()

	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": "ovs-auth-key-v1",
				"n":   encodeB64URL(n),
				"e":   encodeB64URL(e),
			},
		},
	}
	return json.Marshal(jwks)
}

// encodeB64URL encodes bytes as base64url without padding (RFC 7517 §2).
func encodeB64URL(b []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 0, (len(b)*4+2)/3)
	for i := 0; i < len(b); i += 3 {
		var b0, b1, b2 byte
		b0 = b[i]
		if i+1 < len(b) {
			b1 = b[i+1]
		}
		if i+2 < len(b) {
			b2 = b[i+2]
		}
		result = append(result,
			chars[b0>>2],
			chars[((b0&0x3)<<4)|b1>>4],
			chars[((b1&0xf)<<2)|b2>>6],
			chars[b2&0x3f],
		)
	}
	// Trim padding characters
	l := len(b) % 3
	if l == 1 {
		result = result[:len(result)-2]
	} else if l == 2 {
		result = result[:len(result)-1]
	}
	return string(result)
}
