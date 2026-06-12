// Package auth provides OpenVulnScan-specific auth validation via gRPC.
// GRPCAuthValidator replaces the JWKS-based JWTValidator for OpenVulnScan deployments,
// delegating all token and API key validation to the auth-service gRPC endpoint.
//
// Caching strategy:
//   - JWT Bearer tokens: cached in Redis for 60s (key: "gw:token:{sha256(token)[:16]}")
//   - API keys: NOT cached (must be live — ensures revocation takes effect immediately)
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	authv1 "github.com/osv/api-gateway/internal/infra/auth/genproto/auth/v1"
	authDomain "github.com/osv/api-gateway/internal/domain/auth"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	tokenCacheTTL = 60 * time.Second
	tokenCachePrefix = "gw:token:"
)

// cachedValidation is the structure stored in Redis for validated tokens.
type cachedValidation struct {
	UserID      string   `json:"uid"`
	Role        string   `json:"role"`
	Permissions []string `json:"perms"`
	ExpiresAt   int64    `json:"exp"` // unix timestamp
}

// GRPCAuthValidator validates Bearer tokens and API keys by calling auth-service gRPC.
// It caches token validation results in Redis to avoid a gRPC call on every request.
type GRPCAuthValidator struct {
	client   authv1.AuthServiceClient
	cache    *redis.Client
	cacheTTL time.Duration
	log      zerolog.Logger
}

// NewGRPCAuthValidator connects to auth-service and returns a validator.
// authServiceAddr should be "host:port", e.g. "auth-service:9001".
func NewGRPCAuthValidator(authServiceAddr string, rdb *redis.Client, log zerolog.Logger) (*GRPCAuthValidator, error) {
	conn, err := grpc.NewClient(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc connect to auth-service: %w", err)
	}

	return &GRPCAuthValidator{
		client:   authv1.NewAuthServiceClient(conn),
		cache:    rdb,
		cacheTTL: tokenCacheTTL,
		log:      log,
	}, nil
}

// ValidateToken validates a JWT Bearer token.
// Checks Redis cache first; on cache miss calls auth-service.ValidateToken gRPC.
func (v *GRPCAuthValidator) ValidateToken(ctx context.Context, bearerToken string) (*authDomain.Principal, error) {
	token := strings.TrimPrefix(bearerToken, "Bearer ")
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// Cache lookup
	cacheKey := tokenCachePrefix + tokenHash(token)
	if cached, err := v.getCachedValidation(ctx, cacheKey); err == nil {
		return cachedToPrincipal(cached, authDomain.PrincipalOAuth2), nil
	}

	// gRPC call
	resp, err := v.client.ValidateToken(ctx, &authv1.ValidateTokenRequest{Token: token})
	if err != nil {
		return nil, fmt.Errorf("auth-service.ValidateToken: %w", err)
	}
	if !resp.Valid {
		return nil, fmt.Errorf("invalid token: %s", resp.Error)
	}

	p := &authDomain.Principal{
		ID:            resp.UserId,
		Type:          authDomain.PrincipalOAuth2,
		Roles:         []authDomain.Role{authDomain.Role(resp.Role)},
		Permissions:   resp.Permissions,
		RateLimitTier: tierFromRole(resp.Role),
	}

	// Cache the result
	v.setCachedValidation(ctx, cacheKey, &cachedValidation{
		UserID:      resp.UserId,
		Role:        resp.Role,
		Permissions: resp.Permissions,
	})

	return p, nil
}

// ValidateAPIKey validates an API key (ovs_ prefix).
// NOT cached — revocation must take effect immediately.
func (v *GRPCAuthValidator) ValidateAPIKey(ctx context.Context, apiKey string) (*authDomain.Principal, error) {
	if !strings.HasPrefix(apiKey, "ovs_") {
		return nil, fmt.Errorf("invalid API key format: must start with ovs_")
	}

	resp, err := v.client.ValidateAPIKey(ctx, &authv1.ValidateAPIKeyRequest{ApiKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("auth-service.ValidateAPIKey: %w", err)
	}
	if !resp.Valid {
		return nil, fmt.Errorf("invalid API key: %s", resp.Error)
	}

	return &authDomain.Principal{
		ID:            resp.UserId,
		Type:          authDomain.PrincipalAPIKey,
		Permissions:   resp.Permissions,
		APIKeyID:      resp.KeyId,
		RateLimitTier: "standard",
	}, nil
}

// getCachedValidation retrieves a cached token validation from Redis.
func (v *GRPCAuthValidator) getCachedValidation(ctx context.Context, key string) (*cachedValidation, error) {
	val, err := v.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var cv cachedValidation
	if err := json.Unmarshal(val, &cv); err != nil {
		return nil, err
	}
	// Verify not expired (belt-and-suspenders)
	if cv.ExpiresAt > 0 && time.Now().Unix() > cv.ExpiresAt {
		v.cache.Del(ctx, key)
		return nil, fmt.Errorf("cached token expired")
	}
	return &cv, nil
}

// setCachedValidation stores a validated token in Redis with TTL.
func (v *GRPCAuthValidator) setCachedValidation(ctx context.Context, key string, cv *cachedValidation) {
	cv.ExpiresAt = time.Now().Add(v.cacheTTL).Unix()
	data, err := json.Marshal(cv)
	if err != nil {
		v.log.Warn().Err(err).Msg("failed to marshal token cache entry")
		return
	}
	if err := v.cache.Set(ctx, key, data, v.cacheTTL).Err(); err != nil {
		v.log.Warn().Err(err).Str("key", key).Msg("failed to cache token validation")
	}
}

// cachedToPrincipal converts a cached validation entry to a Principal.
func cachedToPrincipal(cv *cachedValidation, typ authDomain.PrincipalType) *authDomain.Principal {
	return &authDomain.Principal{
		ID:            cv.UserID,
		Type:          typ,
		Roles:         []authDomain.Role{authDomain.Role(cv.Role)},
		Permissions:   cv.Permissions,
		RateLimitTier: tierFromRole(cv.Role),
	}
}

// tokenHash returns a short hash of the token for use as a cache key.
// Uses first 16 hex chars of SHA-256 — collision resistant enough for a cache key.
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:8])
}

// tierFromRole maps an OVS role to a rate limit tier.
func tierFromRole(role string) string {
	switch role {
	case "admin":
		return "unlimited"
	case "user":
		return "standard"
	case "readonly":
		return "standard"
	default:
		return "standard"
	}
}
