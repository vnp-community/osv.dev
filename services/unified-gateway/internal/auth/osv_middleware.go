// Package middleware provides HTTP middleware for the api-gateway.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	// CtxKeyUserID is the context key for the authenticated user ID (JWT Subject).
	CtxKeyUserID contextKey = "x-user-id"

	// CtxKeyUserRoles is the context key for the authenticated user's roles.
	CtxKeyUserRoles contextKey = "x-user-roles"
)

// Claims extends jwt.RegisteredClaims with role-based access control.
// Compatible with the auth-service JWT payload structure.
type Claims struct {
	jwt.RegisteredClaims
	Roles []string `json:"roles"`
}

// JWTVerify returns middleware that:
//  1. Skips verification for paths in skipPaths (public endpoints)
//  2. Verifies Bearer JWT token with HMAC-SHA256 (HS256)
//  3. Injects X-User-ID and X-User-Roles headers for upstream services
//  4. Stores claims in request context for downstream middleware (e.g. RequireRole)
func JWTVerify(secret string, skipPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is in the public skip list
			for _, p := range skipPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondUnauthorized(w, "authorization header required")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			tokenStr = strings.TrimPrefix(tokenStr, "bearer ")
			if tokenStr == authHeader {
				// No "Bearer " prefix found
				respondUnauthorized(w, "bearer token required")
				return
			}

			// Parse and verify JWT signature
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				respondUnauthorized(w, "invalid or expired token")
				return
			}

			// Inject user info into upstream request headers
			r.Header.Set("X-User-ID", claims.Subject)
			r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))

			// Store in context for RequireRole middleware
			ctx := context.WithValue(r.Context(), CtxKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, CtxKeyUserRoles, claims.Roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that blocks requests if the authenticated user
// lacks the required role. Must be chained AFTER JWTVerify.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles, _ := r.Context().Value(CtxKeyUserRoles).([]string)
			for _, userRole := range roles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":"forbidden","message":"role %q required"}`, role)
		})
	}
}

func respondUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":"unauthorized","message":%q}`, msg)
}
