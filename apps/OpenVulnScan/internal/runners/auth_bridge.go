// Package runners — auth_bridge.go
// authBridge implements shared/proto AuthServiceServer.
// Delegate đến auth-service internal packages.
// NOTE: Do Go "internal" restriction, chúng ta chỉ có thể import auth-service packages
// nếu chúng ta là trong cùng module tree. Vì dùng go.work workspace,
// auth-service/internal/ KHÔNG thể import từ module ngoài.
//
// Giải pháp thực tế: import các packages KHÔNG nằm trong internal/:
// - adapter/repository/postgres (public)
// - cmd/server (public, nhưng là main)
// Còn lại sẽ implement lại JWT validation trực tiếp.
package runners

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sharedauthv1 "github.com/osv/shared/proto/gen/go/auth/v1"
)

// jwtClaims auth-service JWT payload.
type jwtClaims struct {
	jwt.RegisteredClaims
	UserID      string   `json:"uid"`
	Role        string   `json:"role"`
	Permissions []string `json:"perms"`
}

// authBridge implements sharedauthv1.AuthServiceServer bằng cách
// trực tiếp implement JWT validation và API key lookup.
type authBridge struct {
	sharedauthv1.UnimplementedAuthServiceServer
	cfg        AuthRunnerConfig
	db         *pgxpool.Pool
	rdb        *redis.Client
	log        zerolog.Logger
	privateKey *rsa.PrivateKey
	jwtIssuer  string
	jwtAud     []string
}

func newAuthBridge(cfg AuthRunnerConfig, db *pgxpool.Pool, rdb *redis.Client, l zerolog.Logger) *authBridge {
	return &authBridge{
		cfg:       cfg,
		db:        db,
		rdb:       rdb,
		log:       l,
		jwtIssuer: cfg.JWTIssuer,
		jwtAud:    cfg.JWTAudience,
	}
}

// init loads RSA private key.
func (b *authBridge) init() error {
	keyData, err := os.ReadFile(b.cfg.JWTPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("read jwt key: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
	if err != nil {
		return fmt.Errorf("parse jwt key: %w", err)
	}
	b.privateKey = key
	return nil
}

// ValidateToken verifies a JWT Bearer token.
func (b *authBridge) ValidateToken(ctx context.Context, req *sharedauthv1.ValidateTokenRequest) (*sharedauthv1.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &sharedauthv1.ValidateTokenResponse{Valid: false, Error: "token is required"}, nil
	}

	claims := &jwtClaims{}
	token, err := jwt.ParseWithClaims(req.Token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &b.privateKey.PublicKey, nil
	}, jwt.WithIssuer(b.jwtIssuer))

	if err != nil || !token.Valid {
		return &sharedauthv1.ValidateTokenResponse{Valid: false, Error: "invalid token"}, nil
	}

	// JTI blacklist check (Redis)
	if revoked, err := b.isJTIRevoked(ctx, claims.ID); err == nil && revoked {
		return &sharedauthv1.ValidateTokenResponse{Valid: false, Error: "token revoked"}, nil
	}

	return &sharedauthv1.ValidateTokenResponse{
		Valid:       true,
		UserId:      claims.UserID,
		Role:        claims.Role,
		Permissions: claims.Permissions,
	}, nil
}

// ValidateAPIKey verifies an API key.
func (b *authBridge) ValidateAPIKey(ctx context.Context, req *sharedauthv1.ValidateAPIKeyRequest) (*sharedauthv1.ValidateAPIKeyResponse, error) {
	if req.ApiKey == "" {
		return &sharedauthv1.ValidateAPIKeyResponse{Valid: false, Error: "api_key is required"}, nil
	}
	if len(req.ApiKey) < 12 || !strings.HasPrefix(req.ApiKey, "ovs_") {
		return &sharedauthv1.ValidateAPIKeyResponse{Valid: false, Error: "invalid api key format"}, nil
	}

	prefix := req.ApiKey[:12]

	// Lookup by prefix (auth schema)
	var (
		keyID       uuid.UUID
		userID      uuid.UUID
		keyHash     string
		permissions []string
		revokedAt   *time.Time
		expiresAt   *time.Time
	)
	row := b.db.QueryRow(ctx,
		`SELECT id, user_id, key_hash, permissions, revoked_at, expires_at
		 FROM auth.api_keys WHERE prefix=$1`, prefix)
	if err := row.Scan(&keyID, &userID, &keyHash, &permissions, &revokedAt, &expiresAt); err != nil {
		return &sharedauthv1.ValidateAPIKeyResponse{Valid: false, Error: "api key not found"}, nil
	}

	if revokedAt != nil {
		return &sharedauthv1.ValidateAPIKeyResponse{Valid: false, Error: "api key revoked"}, nil
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return &sharedauthv1.ValidateAPIKeyResponse{Valid: false, Error: "api key expired"}, nil
	}

	// Constant-time hash verification (SHA-256 hex)
	h := sha256.Sum256([]byte(req.ApiKey))
	computedHash := hex.EncodeToString(h[:])
	if computedHash != keyHash {
		return &sharedauthv1.ValidateAPIKeyResponse{Valid: false, Error: "api key invalid"}, nil
	}

	// Update last_used async
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := b.db.Exec(ctx2,
			`UPDATE auth.api_keys SET last_used_at=$1 WHERE id=$2`,
			time.Now().UTC(), keyID,
		); err != nil {
			b.log.Warn().Err(err).Str("key_id", keyID.String()).Msg("update last_used failed")
		}
	}()

	return &sharedauthv1.ValidateAPIKeyResponse{
		Valid:       true,
		UserId:      userID.String(),
		KeyId:       keyID.String(),
		Permissions: permissions,
	}, nil
}

// isJTIRevoked checks Redis for a revoked JWT ID.
func (b *authBridge) isJTIRevoked(ctx context.Context, jti string) (bool, error) {
	key := "jti:revoked:" + jti
	val, err := b.rdb.Exists(ctx, key).Result()
	return val > 0, err
}

// ensurePermission helper for internal gRPC calls.
func (b *authBridge) ensurePermission(perms []string, required string) error {
	for _, p := range perms {
		if p == required {
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied, "missing permission: %s", required)
}
