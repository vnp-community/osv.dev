// Package app — auth_http.go
// authHTTPHandler implement Login, Register, Logout, Refresh HTTP endpoints.
// Auth proto (shared/proto/auth/v1) chỉ có ValidateToken + ValidateAPIKey cho internal gRPC.
// Login/Register/etc là HTTP-only và dùng direct Postgres/Redis access.
package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// authHTTPHandler implement auth HTTP endpoints.
type authHTTPHandler struct {
	db         *pgxpool.Pool
	rdb        *redis.Client
	privateKey *rsa.PrivateKey
	cfg        *Config
}

func newAuthHTTPHandler(db *pgxpool.Pool, rdb *redis.Client, cfg *Config) *authHTTPHandler {
	h := &authHTTPHandler{db: db, rdb: rdb, cfg: cfg}
	// Load JWT key (non-fatal if not found — generate ephemeral key)
	keyData, err := os.ReadFile(cfg.Auth.JWTPrivateKeyPath)
	if err != nil {
		// Generate ephemeral RSA key for dev
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		h.privateKey = key
	} else {
		key, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
		if err == nil {
			h.privateKey = key
		} else {
			key, _ := rsa.GenerateKey(rand.Reader, 2048)
			h.privateKey = key
		}
	}
	return h
}

// login handles POST /api/v1/auth/login
func (h *authHTTPHandler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}

	// Lookup user
	var (
		userID       uuid.UUID
		passwordHash string
		role         string
		isActive     bool
	)
	err := h.db.QueryRow(r.Context(), `
		SELECT id, password_hash, role, is_active
		FROM auth.users WHERE email = $1
	`, req.Email).Scan(&userID, &passwordHash, &role, &isActive)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if !isActive {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "account disabled"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	// Issue JWT
	accessToken, err := h.issueToken(userID.String(), req.Email, role, h.cfg.Auth.JWTAccessTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token issue failed"})
		return
	}
	refreshToken, err := h.issueToken(userID.String(), req.Email, role, h.cfg.Auth.JWTRefreshTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token issue failed"})
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(h.cfg.Auth.JWTAccessTTL.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user_id":       userID.String(),
		"role":          role,
		"expires_in":    int(h.cfg.Auth.JWTAccessTTL.Seconds()),
	})
}

// register handles POST /api/v1/auth/register
func (h *authHTTPHandler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password too short (min 8)"})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash error"})
		return
	}

	userID := uuid.New()
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO auth.users (id, email, password_hash, name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'user', true, NOW(), NOW())
	`, userID, req.Email, string(hash), req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "registration failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user_id": userID.String(),
		"email":   req.Email,
		"message": "registered successfully",
	})
}

// logout handles POST /api/v1/auth/logout
func (h *authHTTPHandler) logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		// Parse JTI to revoke
		claims := &jwt.RegisteredClaims{}
		jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) { //nolint:errcheck
			return &h.privateKey.PublicKey, nil
		})
		if claims.ID != "" {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				h.rdb.Set(r.Context(), "jti:revoked:"+claims.ID, 1, ttl) //nolint:errcheck
			}
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "access_token", Value: "", MaxAge: -1, Path: "/"})
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// refresh handles POST /api/v1/auth/refresh
func (h *authHTTPHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "refresh_token required"})
		return
	}

	type authClaims struct {
		jwt.RegisteredClaims
		UserID string `json:"uid"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	claims := &authClaims{}
	_, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected method")
		}
		return &h.privateKey.PublicKey, nil
	})
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	// Issue new access token
	accessToken, err := h.issueToken(claims.UserID, claims.Email, claims.Role, h.cfg.Auth.JWTAccessTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token issue failed"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(h.cfg.Auth.JWTAccessTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"expires_in":   int(h.cfg.Auth.JWTAccessTTL.Seconds()),
	})
}

// issueToken tạo JWT token với RSA private key.
func (h *authHTTPHandler) issueToken(userID, email, role string, ttl time.Duration) (string, error) {
	type authClaims struct {
		jwt.RegisteredClaims
		UserID string `json:"uid"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}

	now := time.Now().UTC()
	jti := uuid.New().String()

	// Hash JTI for storage
	h24 := sha256.Sum256([]byte(jti))
	_ = hex.EncodeToString(h24[:])

	claims := authClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    h.cfg.Auth.JWTIssuer,
			Subject:   userID,
			Audience:  h.cfg.Auth.JWTAudience,
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
		UserID: userID,
		Email:  email,
		Role:   role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(h.privateKey)
}

// changePassword handles password change (internal use).
func (h *authHTTPHandler) changePassword(ctx context.Context, userID, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = h.db.Exec(ctx,
		`UPDATE auth.users SET password_hash = $1, updated_at = NOW() WHERE id = $2::uuid`,
		string(hash), userID)
	return err
}
