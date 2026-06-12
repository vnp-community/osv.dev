// Package grpc provides the gRPC server handler for the AuthService.
// This is the fast path called by api-gateway on every request.
// Target: < 1ms when token is valid and JTI not revoked.
package grpc

import (
	"context"
	"time"

	authv1 "github.com/osv/identity-service/internal/infra/auth/genproto/auth/v1"
	"github.com/osv/identity-service/internal/domain/repository"
	jwtpkg "github.com/osv/identity-service/internal/infrastructure/jwt"
	"github.com/osv/identity-service/internal/infrastructure/cache"
	"github.com/osv/identity-service/internal/infrastructure/crypto"
	"github.com/osv/identity-service/internal/domain/valueobject"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthGRPCHandler implements authv1.AuthServiceServer.
// Fast path: JWT RSA verify + Redis JTI blacklist check (no DB access).
type AuthGRPCHandler struct {
	authv1.UnimplementedAuthServiceServer

	jwtSvc     *jwtpkg.Service
	tokenCache *cache.TokenCache
	apiKeyRepo repository.APIKeyRepository
	log        zerolog.Logger
}

// NewAuthGRPCHandler creates a new gRPC handler.
func NewAuthGRPCHandler(
	jwtSvc *jwtpkg.Service,
	tokenCache *cache.TokenCache,
	apiKeyRepo repository.APIKeyRepository,
	log zerolog.Logger,
) *AuthGRPCHandler {
	return &AuthGRPCHandler{
		jwtSvc:     jwtSvc,
		tokenCache: tokenCache,
		apiKeyRepo: apiKeyRepo,
		log:        log,
	}
}

// ValidateToken verifies a JWT Bearer token.
// Fast path: RSA signature verify + JTI blacklist check only (no DB call).
func (h *AuthGRPCHandler) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &authv1.ValidateTokenResponse{
			Valid:  false,
			Error:  "token is required",
		}, nil
	}

	// Step 1: RSA signature + expiry validation (fast, no I/O)
	claims, err := h.jwtSvc.ValidateToken(req.Token)
	if err != nil {
		return &authv1.ValidateTokenResponse{Valid: false, Error: err.Error()}, nil
	}

	// Step 2: JTI blacklist check (single Redis GET)
	revoked, err := h.tokenCache.IsJTIRevoked(ctx, claims.ID)
	if err != nil {
		h.log.Warn().Err(err).Str("jti", claims.ID).Msg("JTI blacklist check failed")
		// Fail open: log warning but allow (avoid denying valid tokens on Redis failure)
	} else if revoked {
		return &authv1.ValidateTokenResponse{Valid: false, Error: "token has been revoked"}, nil
	}

	// Build response
	resp := &authv1.ValidateTokenResponse{
		Valid:       true,
		UserId:      claims.UserID,
		Role:        claims.Role,
		Permissions: claims.Permissions,
	}
	if claims.ExpiresAt != nil && !claims.ExpiresAt.Time.IsZero() {
		resp.ExpiresAt = timestamppb.New(claims.ExpiresAt.Time)
	}
	return resp, nil
}

// ValidateAPIKey verifies an API key and returns its permissions.
// Performs a DB lookup by key prefix, then verifies the SHA-256 hash.
func (h *AuthGRPCHandler) ValidateAPIKey(ctx context.Context, req *authv1.ValidateAPIKeyRequest) (*authv1.ValidateAPIKeyResponse, error) {
	if req.ApiKey == "" {
		return &authv1.ValidateAPIKeyResponse{Valid: false, Error: "api_key is required"}, nil
	}

	if len(req.ApiKey) < 12 || req.ApiKey[:4] != "ovs_" {
		return &authv1.ValidateAPIKeyResponse{Valid: false, Error: "invalid api key format"}, nil
	}

	// Extract prefix (first 12 chars: "ovs_" + 8 chars)
	prefix := req.ApiKey[:12]

	// DB lookup by prefix
	apiKey, err := h.apiKeyRepo.FindByPrefix(ctx, prefix)
	if err != nil {
		return &authv1.ValidateAPIKeyResponse{Valid: false, Error: "api key not found"}, nil
	}

	if !apiKey.IsActive() {
		return &authv1.ValidateAPIKeyResponse{Valid: false, Error: "api key is revoked or expired"}, nil
	}

	// Constant-time hash verification
	if !crypto.ValidateAPIKey(req.ApiKey, apiKey.KeyHash) {
		return &authv1.ValidateAPIKeyResponse{Valid: false, Error: "api key invalid"}, nil
	}

	// Update last used timestamp (async, don't block response)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := h.apiKeyRepo.UpdateLastUsed(ctx2, apiKey.ID); err != nil {
			h.log.Warn().Err(err).Str("key_id", apiKey.ID.String()).Msg("failed to update api key last_used")
		}
	}()

	return &authv1.ValidateAPIKeyResponse{
		Valid:       true,
		UserId:      apiKey.UserID.String(),
		KeyId:       apiKey.ID.String(),
		Permissions: apiKey.Permissions,
	}, nil
}

// EnsurePermission checks if a gRPC call has the required permission.
// Used for internal auth between services.
func EnsurePermission(perms []string, required string) error {
	for _, p := range perms {
		if p == required {
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied, "missing permission: %s", required)
}

// permissionsContain checks if a required permission is in a list.
func permissionsContain(perms []valueobject.Permission, required string) bool {
	for _, p := range perms {
		if p == required {
			return true
		}
	}
	return false
}
