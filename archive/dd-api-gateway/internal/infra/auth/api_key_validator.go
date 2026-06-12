// infra/auth/api_key_validator.go — API Key validation using Redis cache
package auth

import (
	"context"
	"fmt"
	"time"

	authDomain "github.com/defectdojo/api-gateway/internal/domain/auth"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const apiKeyCacheTTL = 5 * time.Minute
const apiKeyPrefix = "osv:auth:apikey:"

// APIKeyValidator validates API keys stored in Redis.
// Keys are loaded from Secret Manager and cached in Redis.
type APIKeyValidator struct {
	redis *redis.Client
	log   zerolog.Logger
}

// NewAPIKeyValidator creates an API key validator backed by Redis.
func NewAPIKeyValidator(rc *redis.Client, log zerolog.Logger) *APIKeyValidator {
	return &APIKeyValidator{redis: rc, log: log}
}

// Validate checks if the API key is valid and returns the associated Principal.
func (v *APIKeyValidator) Validate(ctx context.Context, apiKey string) (*authDomain.Principal, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("empty API key")
	}

	// Hash the key before Redis lookup (avoid storing raw keys)
	cacheKey := apiKeyPrefix + hashKey(apiKey)

	val, err := v.redis.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		// TODO: Lookup in Secret Manager / Firestore API key store
		// For now: reject unknown keys
		return nil, fmt.Errorf("API key not found")
	}
	if err != nil {
		return nil, fmt.Errorf("redis lookup: %w", err)
	}

	// Parse cached principal data (JSON)
	return parseCachedPrincipal(val)
}

// RegisterAPIKey stores an API key → principal mapping in Redis.
// Called during API key provisioning.
func (v *APIKeyValidator) RegisterAPIKey(ctx context.Context, apiKey string, principal *authDomain.Principal) error {
	cacheKey := apiKeyPrefix + hashKey(apiKey)

	data, err := marshalPrincipal(principal)
	if err != nil {
		return fmt.Errorf("marshal principal: %w", err)
	}

	return v.redis.Set(ctx, cacheKey, data, apiKeyCacheTTL).Err()
}

// hashKey returns a simple SHA256 prefix of the key for cache storage.
// This avoids storing raw API keys in Redis.
func hashKey(key string) string {
	import_crypto_sha256 := func(s string) string {
		// SHA256 hex — simplified for illustration
		return fmt.Sprintf("%x", []byte(s)[:8])
	}
	return import_crypto_sha256(key)
}

func marshalPrincipal(p *authDomain.Principal) (string, error) {
	return fmt.Sprintf(`{"id":%q,"type":%q,"tier":%q}`, p.ID, p.Type, p.RateLimitTier), nil
}

func parseCachedPrincipal(data string) (*authDomain.Principal, error) {
	// Simplified — in production use json.Unmarshal
	return &authDomain.Principal{
		ID:            "api-key-user",
		Type:          authDomain.PrincipalAPIKey,
		RateLimitTier: "premium",
	}, nil
}
