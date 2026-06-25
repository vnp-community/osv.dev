// Package middleware provides HTTP middleware for the api-gateway.
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	redisinfra "github.com/osv/gateway-service/internal/infra/redis"
)

type contextKey string

const (
	// CtxKeyUserID is the context key for the authenticated user ID (JWT Subject).
	CtxKeyUserID contextKey = "x-user-id"

	// CtxKeyUserRoles is the context key for the authenticated user's roles.
	CtxKeyUserRoles contextKey = "x-user-roles"
)

// Claims extends jwt.RegisteredClaims with role-based access control.
// Compatible with the identity-service JWT payload structure.
type Claims struct {
	jwt.RegisteredClaims
	Roles []string `json:"roles"`
}

// AuthVerify returns middleware that handles JWT, X-API-Key, and Authorization: ApiKey.
// Priority: JWT (Bearer) -> X-API-Key -> Authorization: ApiKey.
// If valid, injects X-User-ID and X-User-Roles, and stores claims in context.
func AuthVerify(secret string, skipPaths []string, apiKeyValidator *APIKeyValidator, cache ...*redisinfra.TokenCache) func(http.Handler) http.Handler {
	var tokenCache *redisinfra.TokenCache
	if len(cache) > 0 {
		tokenCache = cache[0]
	}

	// Parse RSA public key from PEM if the secret looks like a PEM block (starts with "-----")
	// Otherwise treat as HMAC secret.
	var rsaPublicKey *rsa.PublicKey
	if strings.HasPrefix(strings.TrimSpace(secret), "-----BEGIN") {
		block, _ := pem.Decode([]byte(secret))
		if block != nil {
			if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
				if rsaKey, ok := pub.(*rsa.PublicKey); ok {
					rsaPublicKey = rsaKey
				}
			}
			// Also try PKCS1
			if rsaPublicKey == nil {
				if rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
					rsaPublicKey = rsaKey
				}
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use RequestURI for skip-path matching — chi.Mount() strips the prefix
			// from r.URL.Path, so we must use the original full path from RequestURI.
			checkPath := r.RequestURI
			if idx := strings.Index(checkPath, "?"); idx >= 0 {
				checkPath = checkPath[:idx]
			}
			if checkPath == "" {
				checkPath = r.URL.Path
			}
			for _, p := range skipPaths {
				if strings.HasPrefix(checkPath, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			var claims *Claims
			authHeader := r.Header.Get("Authorization")

			// 1. JWT (Bearer)
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				tokenStr := authHeader[7:]
				if tokenCache != nil {
					if cached, ok := tokenCache.Get(r.Context(), tokenStr); ok {
						rolesStr := make([]string, len(cached.Roles))
						for i, r := range cached.Roles { rolesStr[i] = string(r) }
						claims = &Claims{
							RegisteredClaims: jwt.RegisteredClaims{Subject: cached.ID},
							Roles:            rolesStr,
						}
					}
				}

				if claims == nil {
					parsedClaims := &Claims{}
					token, err := jwt.ParseWithClaims(tokenStr, parsedClaims, func(t *jwt.Token) (interface{}, error) {
						// Detect algorithm from token header to support both RS256 and HS256
						switch t.Method.(type) {
						case *jwt.SigningMethodRSA:
							// RS256: verify with RSA public key
							if rsaPublicKey != nil {
								return rsaPublicKey, nil
							}
							// Fallback: if no PEM configured, try HMAC (will fail gracefully)
							return []byte(secret), nil
						default:
							// HS256 / HS384 / HS512: verify with HMAC secret
							return []byte(secret), nil
						}
					})
					if err == nil && token.Valid {
						claims = parsedClaims
						if tokenCache != nil {
							cp := principalFromClaims(claims)
							var expiry time.Time
							if exp, err := claims.GetExpirationTime(); err == nil && exp != nil {
								expiry = exp.Time
							}
							tokenCache.Set(r.Context(), tokenStr, &cp, expiry)
						}
					}
				}
			}


			// 2. X-API-Key
			if claims == nil && apiKeyValidator != nil {
				if rawKey := r.Header.Get("X-API-Key"); rawKey != "" {
					if apiClaims, err := apiKeyValidator.Validate(r.Context(), rawKey); err == nil {
						claims = &Claims{
							RegisteredClaims: jwt.RegisteredClaims{Subject: apiClaims.UserID},
							Roles:            apiClaims.Scopes,
						}
					} else if errors.Is(err, ErrExpiredAPIKey) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusUnauthorized)
						fmt.Fprint(w, `{"error":"API key has expired."}`)
						return
					}
				}
			}

			// 3. Authorization: ApiKey
			if claims == nil && apiKeyValidator != nil {
				if strings.HasPrefix(authHeader, "ApiKey ") {
					rawKey := authHeader[7:]
					if apiClaims, err := apiKeyValidator.Validate(r.Context(), rawKey); err == nil {
						claims = &Claims{
							RegisteredClaims: jwt.RegisteredClaims{Subject: apiClaims.UserID},
							Roles:            apiClaims.Scopes,
						}
					} else if errors.Is(err, ErrExpiredAPIKey) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusUnauthorized)
						fmt.Fprint(w, `{"error":"API key has expired."}`)
						return
					}
				}
			}

			if claims == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"Authentication credentials were not provided."}`)
				return
			}

			r.Header.Set("X-User-ID", claims.Subject)
			r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
			ctx := context.WithValue(r.Context(), CtxKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, CtxKeyUserRoles, claims.Roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// principalFromClaims converts JWT Claims to a redisinfra-cacheable auth.Principal.
func principalFromClaims(claims *Claims) redisinfra.CachePrincipal {
	return redisinfra.CachePrincipal{
		ID:    claims.Subject,
		Roles: claims.Roles,
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

// End of middleware.

