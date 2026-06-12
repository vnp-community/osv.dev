// Package auth provides production JWT validation with complete RSA signature verification.
// Uses golang-jwt/jwt/v5 for cryptographic verification against JWKS.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	authDomain "github.com/osv/gateway-service/internal/domain/auth"
	"github.com/rs/zerolog"
)

// JWTValidator validates OAuth2 Bearer tokens against JWKS with full RSA signature verification.
type JWTValidator struct {
	jwksURL    string
	audience   string // expected aud claim
	issuer     string // expected iss claim (or multiple, comma-separated)

	keys      map[string]*rsa.PublicKey
	keysMu    sync.RWMutex
	lastFetch time.Time
	cacheTTL  time.Duration
	http      *http.Client
	log       zerolog.Logger
}

// NewJWTValidator creates a production JWT validator.
func NewJWTValidator(jwksURL, audience, issuer string, log zerolog.Logger) *JWTValidator {
	v := &JWTValidator{
		jwksURL:  jwksURL,
		audience: audience,
		issuer:   issuer,
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: 1 * time.Hour,
		http:     &http.Client{Timeout: 10 * time.Second},
		log:      log,
	}
	// Pre-fetch keys at startup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := v.fetchJWKS(ctx); err != nil {
		log.Warn().Err(err).Str("jwks_url", jwksURL).Msg("initial JWKS fetch failed, will retry on first request")
	}
	// Background refresh goroutine
	go v.backgroundRefresh()
	return v
}

// osvClaims extends standard JWT claims with OSV-specific fields.
type osvClaims struct {
	jwt.RegisteredClaims
	Scope  string   `json:"scope"`
	Roles  []string `json:"roles"`
	Email  string   `json:"email"`
	OrgID  string   `json:"org_id"`
}

// Validate parses and cryptographically verifies a JWT Bearer token.
func (v *JWTValidator) Validate(ctx context.Context, bearerToken string) (*authDomain.Principal, error) {
	tokenStr := strings.TrimPrefix(bearerToken, "Bearer ")
	if tokenStr == "" {
		return nil, fmt.Errorf("empty bearer token")
	}

	// Ensure JWKS are loaded
	if err := v.refreshIfStale(ctx); err != nil {
		v.log.Warn().Err(err).Msg("JWKS refresh failed, using cached keys")
	}

	// Parse with full verification
	token, err := jwt.ParseWithClaims(tokenStr, &osvClaims{}, v.keyFunc,
		jwt.WithAudience(v.audience),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second), // clock skew tolerance
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	claims, ok := token.Claims.(*osvClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Map scopes to roles
	roles := make([]authDomain.Role, 0, len(claims.Roles))
	for _, r := range claims.Roles {
		roles = append(roles, authDomain.Role(r))
	}
	for _, scope := range strings.Fields(claims.Scope) {
		roles = append(roles, authDomain.Role("scope:"+scope))
	}

	return &authDomain.Principal{
		ID:            claims.Subject,
		Type:          authDomain.PrincipalOAuth2,
		Email:         claims.Email,
		OrgID:         claims.OrgID,
		Roles:         roles,
		RateLimitTier: tierFromRoles(roles),
		Metadata: map[string]string{
			"iss": claims.Issuer,
			"kid": extractKID(tokenStr),
		},
	}, nil
}

// keyFunc provides the RSA public key for JWT verification.
func (v *JWTValidator) keyFunc(token *jwt.Token) (interface{}, error) {
	// Verify algorithm is RS256 or RS384 or RS512
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}

	kid, _ := token.Header["kid"].(string)
	v.keysMu.RLock()
	key, exists := v.keys[kid]
	v.keysMu.RUnlock()

	if !exists {
		// Force refresh on unknown kid (key rotation)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := v.fetchJWKS(ctx); err != nil {
			return nil, fmt.Errorf("key not found and JWKS refresh failed: kid=%q", kid)
		}
		v.keysMu.RLock()
		key, exists = v.keys[kid]
		v.keysMu.RUnlock()
		if !exists {
			return nil, fmt.Errorf("key not found in JWKS: kid=%q", kid)
		}
	}
	return key, nil
}

// fetchJWKS fetches and caches RSA public keys from the JWKS endpoint.
func (v *JWTValidator) fetchJWKS(ctx context.Context) error {
	v.keysMu.Lock()
	defer v.keysMu.Unlock()

	// Double-check under write lock
	if time.Since(v.lastFetch) < v.cacheTTL {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("build JWKS request: %w", err)
	}
	req.Header.Set("User-Agent", "OSV-APIGateway/1.0")

	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS from %s: %w", v.jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read JWKS body: %w", err)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("parse JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := jwkToRSA(k.N, k.E)
		if err != nil {
			v.log.Warn().Err(err).Str("kid", k.Kid).Msg("skip invalid JWKS key")
			continue
		}
		newKeys[k.Kid] = pub
	}

	if len(newKeys) == 0 {
		return fmt.Errorf("no valid RSA signing keys found in JWKS")
	}

	v.keys = newKeys
	v.lastFetch = time.Now()
	v.log.Info().Int("keys", len(newKeys)).Str("jwks_url", v.jwksURL).Msg("JWKS refreshed")
	return nil
}

func (v *JWTValidator) refreshIfStale(ctx context.Context) error {
	v.keysMu.RLock()
	stale := time.Since(v.lastFetch) >= v.cacheTTL
	v.keysMu.RUnlock()
	if stale {
		return v.fetchJWKS(ctx)
	}
	return nil
}

func (v *JWTValidator) backgroundRefresh() {
	ticker := time.NewTicker(50 * time.Minute) // refresh before TTL expires
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := v.fetchJWKS(ctx); err != nil {
			v.log.Warn().Err(err).Msg("background JWKS refresh failed")
		}
		cancel()
	}
}

// jwkToRSA converts JWK n+e to *rsa.PublicKey.
func jwkToRSA(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	if e == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

func extractKID(tokenStr string) string {
	parts := strings.SplitN(tokenStr, ".", 2)
	if len(parts) < 1 {
		return ""
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ""
	}
	var header struct{ Kid string `json:"kid"` }
	json.Unmarshal(headerBytes, &header) //nolint:errcheck
	return header.Kid
}

func tierFromRoles(roles []authDomain.Role) string {
	for _, r := range roles {
		switch r {
		case "admin", "scope:admin":
			return "unlimited"
		case "premium", "scope:premium":
			return "premium"
		}
	}
	return "standard"
}
