// Package middleware — auth.go
// JWT middleware dùng AuthRunner's gRPC bufconn client để validate token.
// Không cần network call — in-process via bufconn.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	sharedauthv1 "github.com/osv/shared/proto/gen/go/auth/v1"
)

// contextKey là type riêng để tránh conflict với khác packages.
type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyRole   contextKey = "user_role"
	ContextKeyEmail  contextKey = "user_email"
)

// AuthMiddleware validate JWT token qua gRPC auth client (in-process bufconn).
type AuthMiddleware struct {
	authClient sharedauthv1.AuthServiceClient
}

// NewAuthMiddleware tạo JWT middleware với gRPC auth client.
func NewAuthMiddleware(authClient sharedauthv1.AuthServiceClient) *AuthMiddleware {
	return &AuthMiddleware{authClient: authClient}
}

// RequireAuth middleware: validate JWT, inject user info vào context.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeUnauthorized(w, "missing token")
			return
		}

		resp, err := m.authClient.ValidateToken(r.Context(), &sharedauthv1.ValidateTokenRequest{
			Token: token,
		})
		if err != nil {
			log.Error().Err(err).Msg("auth validate error")
			writeUnauthorized(w, "auth service error")
			return
		}
		if !resp.Valid {
			writeUnauthorized(w, resp.Error)
			return
		}

		// Inject claims vào context
		ctx := context.WithValue(r.Context(), ContextKeyUserID, resp.UserId)
		ctx = context.WithValue(ctx, ContextKeyRole, resp.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin middleware: chỉ cho phép admin role.
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ContextKeyRole).(string)
		if role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// extractToken lấy JWT token từ Authorization header hoặc cookie.
func extractToken(r *http.Request) string {
	// 1. Authorization: Bearer <token>
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// 2. Cookie
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","detail":"` + msg + `"}`)) //nolint:errcheck
}

// UserID lấy user ID từ context (sau khi qua RequireAuth).
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyUserID).(string)
	return v
}

// UserRole lấy role từ context.
func UserRole(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyRole).(string)
	return v
}

// RequireAuthOrAPIKey accepts either JWT Bearer token or X-API-Key header.
// API key is validated via gRPC ValidateAPIKey call (in-process bufconn).
func (m *AuthMiddleware) RequireAuthOrAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Try X-API-Key header first
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			resp, err := m.authClient.ValidateAPIKey(r.Context(), &sharedauthv1.ValidateAPIKeyRequest{
				ApiKey: apiKey,
			})
			if err != nil {
				log.Error().Err(err).Msg("api key validate error")
				writeUnauthorized(w, "api key validation error")
				return
			}
			if !resp.Valid {
				writeUnauthorized(w, "invalid api key")
				return
			}
			ctx := context.WithValue(r.Context(), ContextKeyUserID, resp.UserId)
			ctx = context.WithValue(ctx, ContextKeyRole, "api")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 2. Fall back to JWT Bearer token
		m.RequireAuth(next).ServeHTTP(w, r)
	})
}
