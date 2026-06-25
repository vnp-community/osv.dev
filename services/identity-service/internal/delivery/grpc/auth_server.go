package grpc

import (
    "context"
    "crypto/sha256"
    "crypto/subtle"
    "encoding/hex"
    "strings"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/rs/zerolog"
    "google.golang.org/protobuf/types/known/timestamppb"

    "github.com/osv/identity-service/internal/crypto"
    "github.com/osv/identity-service/internal/domain/apikey"
    authpb "github.com/osv/shared/proto/gen/go/auth/v1"
)

// Metrics
var (
    validateTokenDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "auth_validate_token_duration_seconds",
        Help:    "Duration of ValidateToken gRPC calls",
        Buckets: []float64{0.0005, 0.001, 0.002, 0.005, 0.01},
    })
    validateTokenTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "auth_validate_token_total",
        Help: "Total ValidateToken calls",
    }, []string{"result"}) // result: valid|invalid|error

    validateAPIKeyTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "auth_validate_apikey_total",
        Help: "Total ValidateAPIKey calls",
    }, []string{"result"})
)

// JTICache interface — only requires Redis GET+EXISTS
type JTICache interface {
    CheckJTI(ctx context.Context, jti string) (bool, error)
}

// APIKeyRepository interface — prefix lookup
type APIKeyRepository interface {
    FindByPrefix(ctx context.Context, prefix string) (*apikey.APIKey, error)
    UpdateLastUsed(ctx context.Context, keyID string) error
}

// AuthGRPCServer implements the AuthService gRPC interface
type AuthGRPCServer struct {
    authpb.UnimplementedAuthServiceServer
    jwtManager *crypto.JWTManager
    jtiCache   JTICache
    apiKeyRepo APIKeyRepository
    logger     zerolog.Logger
}

// NewAuthGRPCServer creates the gRPC auth server
func NewAuthGRPCServer(
    jwtManager *crypto.JWTManager,
    jtiCache JTICache,
    apiKeyRepo APIKeyRepository,
    logger zerolog.Logger,
) *AuthGRPCServer {
    return &AuthGRPCServer{
        jwtManager: jwtManager,
        jtiCache:   jtiCache,
        apiKeyRepo: apiKeyRepo,
        logger:     logger,
    }
}

// ValidateToken — HOT PATH: must complete in < 1ms
// Steps:
//  1. Parse JWT (RS256 crypto verify) — ~0.1ms CPU
//  2. Check JTI in Redis — ~0.3ms network
//  3. Return claims
func (s *AuthGRPCServer) ValidateToken(
    ctx context.Context,
    req *authpb.ValidateTokenRequest,
) (*authpb.ValidateTokenResponse, error) {
    start := time.Now()
    defer func() {
        validateTokenDuration.Observe(time.Since(start).Seconds())
    }()

    invalid := &authpb.ValidateTokenResponse{Valid: false}

    if req.Token == "" {
        validateTokenTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    // Step 1: Parse and verify JWT signature + expiry (CPU-only, no I/O)
    claims, err := s.jwtManager.Parse(req.Token)
    if err != nil {
        validateTokenTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    // Step 2: Check JTI exists in Redis (ensures logout/revocation works)
    exists, err := s.jtiCache.CheckJTI(ctx, claims.ID)
    if err != nil {
        s.logger.Error().Err(err).Str("jti", claims.ID).Msg("redis jti check failed")
        validateTokenTotal.WithLabelValues("error").Inc()
        // Fail open or closed? Fail CLOSED for security
        return invalid, nil
    }
    if !exists {
        validateTokenTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    validateTokenTotal.WithLabelValues("valid").Inc()
    return &authpb.ValidateTokenResponse{
        Valid:       true,
        UserId:      claims.Subject,
        Role:        claims.Role,
        Permissions: claims.Permissions,
        ExpiresAt:   timestamppb.New(claims.ExpiresAt.Time),
    }, nil
}

// ValidateAPIKey validates an ovs_xxx API key
// Steps:
//  1. Check prefix format
//  2. DB lookup by prefix (indexed)
//  3. Constant-time compare hash
//  4. Check revocation + expiry
func (s *AuthGRPCServer) ValidateAPIKey(
    ctx context.Context,
    req *authpb.ValidateAPIKeyRequest,
) (*authpb.ValidateAPIKeyResponse, error) {
    invalid := &authpb.ValidateAPIKeyResponse{Valid: false}

    key := req.ApiKey
    if key == "" || !strings.HasPrefix(key, apikey.KeyPrefix) {
        validateAPIKeyTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    if len(key) < apikey.PrefixDisplayLength {
        validateAPIKeyTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    // Step 2: Lookup by prefix (avoids full table scan)
    prefix := key[:apikey.PrefixDisplayLength]
    storedKey, err := s.apiKeyRepo.FindByPrefix(ctx, prefix)
    if err != nil {
        validateAPIKeyTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    // Step 3: Constant-time hash comparison (prevents timing attacks)
    inputHash := sha256Hex(key)
    if subtle.ConstantTimeCompare(
        []byte(inputHash),
        []byte(storedKey.KeyHash),
    ) != 1 {
        validateAPIKeyTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    // Step 4: Check active status
    if !storedKey.IsActive() {
        validateAPIKeyTotal.WithLabelValues("invalid").Inc()
        return invalid, nil
    }

    // Step 5: Update last_used_at (async — does NOT block response)
    go func() {
        ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := s.apiKeyRepo.UpdateLastUsed(ctx2, storedKey.ID.String()); err != nil {
            s.logger.Warn().Err(err).Str("key_id", storedKey.ID.String()).
                Msg("failed to update api key last_used_at")
        }
    }()

    validateAPIKeyTotal.WithLabelValues("valid").Inc()
    return &authpb.ValidateAPIKeyResponse{
        Valid:       true,
        UserId:      storedKey.UserID.String(),
        Permissions: storedKey.Permissions,
        KeyId:       storedKey.ID.String(),
    }, nil
}

// sha256Hex returns the lowercase hex SHA-256 of a string
func sha256Hex(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])
}
