// Package auth — API Key validator with Redis caching.
// Validates X-API-Key / Authorization: ApiKey <key> using SHA-256 hash lookup.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/osv/gateway-service/internal/domain/repository"
)

const apiKeyCacheTTL = 5 * time.Minute

// ErrInvalidAPIKey is returned when a key is missing, inactive, or not found.
var ErrInvalidAPIKey = errors.New("invalid or inactive api key")

// ErrExpiredAPIKey is returned when a key has passed its expiry time.
var ErrExpiredAPIKey = errors.New("api key has expired")

// APIKeyClaims contains authenticated identity extracted from an API key.
type APIKeyClaims struct {
	UserID    string   `json:"user_id"`
	Scopes    []string `json:"scopes"`
	AuthType  string   `json:"auth_type"` // always "api_key"
	KeyID     string   `json:"key_id,omitempty"`
	RateLimit *int     `json:"rate_limit,omitempty"` // nil = use global default
}

// APIKeyValidator validates API keys using Redis cache (hot path) + PostgreSQL (cold path).
type APIKeyValidator struct {
	repo  repository.APIKeyRepository
	cache *redis.Client
}

// NewAPIKeyValidator creates a new validator.
func NewAPIKeyValidator(repo repository.APIKeyRepository, cache *redis.Client) *APIKeyValidator {
	return &APIKeyValidator{repo: repo, cache: cache}
}

// Validate looks up an API key by hashing the raw key value.
// Flow: Redis cache (5m TTL) → PostgreSQL → async update last_used_at.
func (v *APIKeyValidator) Validate(ctx context.Context, rawKey string) (*APIKeyClaims, error) {
	if rawKey == "" {
		return nil, ErrInvalidAPIKey
	}

	hash := sha256Hex(rawKey)
	cacheKey := "apikey:v1:" + hash

	// 1. Redis cache (hot path — no crypto/DB on warm requests)
	if cached, err := v.cache.Get(ctx, cacheKey).Bytes(); err == nil {
		var claims APIKeyClaims
		if json.Unmarshal(cached, &claims) == nil {
			return &claims, nil
		}
	}

	// 2. PostgreSQL lookup (cold path)
	// MOCK-012 FIX: nil-check — repo not wired in embedded mode without identity-service DB
	if v.repo == nil {
		return nil, ErrInvalidAPIKey
	}
	key, err := v.repo.FindByHash(ctx, hash)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}
	if !key.IsActive {
		return nil, ErrInvalidAPIKey
	}
	if key.IsExpired() {
		return nil, ErrExpiredAPIKey
	}

	// 3. Async update last_used_at (fire-and-forget, non-blocking)
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		v.repo.UpdateLastUsed(updateCtx, key.ID) //nolint:errcheck
	}()

	claims := &APIKeyClaims{
		UserID:    key.OwnerID,
		Scopes:    key.Scopes,
		AuthType:  "api_key",
		KeyID:     key.ID,
		RateLimit: key.RateLimit,
	}

	// 4. Cache validated claims for 5 minutes
	if data, err := json.Marshal(claims); err == nil {
		v.cache.Set(ctx, cacheKey, data, apiKeyCacheTTL) //nolint:errcheck
	}

	return claims, nil
}

// sha256Hex returns the hex-encoded SHA-256 hash of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// CtxKeyAPIKeyClaims is the context key for injected API key claims.
const CtxKeyAPIKeyClaims contextKey = "x-apikey-claims"
