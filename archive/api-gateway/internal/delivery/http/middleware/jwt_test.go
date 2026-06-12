package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osv/api-gateway/internal/delivery/http/middleware"
)

const testSecret = "test-secret-key-for-unit-tests-32bytes"

func makeToken(t *testing.T, subject string, roles []string, secret string, expired bool) string {
	t.Helper()
	claims := middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: subject,
		},
		Roles: roles,
	}
	if expired {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	} else {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

// ── JWTVerify tests ────────────────────────────────────────────────────────

func TestJWTVerify_ValidToken_InjectsHeaders(t *testing.T) {
	token := makeToken(t, "user-123", []string{"user"}, testSecret, false)
	var gotUserID, gotRoles string
	handler := middleware.JWTVerify(testSecret, []string{"/health"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUserID = r.Header.Get("X-User-ID")
			gotRoles = r.Header.Get("X-User-Roles")
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cve/CVE-2021-44228", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-123", gotUserID)
	assert.Equal(t, "user", gotRoles)
}

func TestJWTVerify_MissingAuthHeader_Returns401(t *testing.T) {
	handler := middleware.JWTVerify(testSecret, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cve/CVE-2021-44228", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "authorization header required")
}

func TestJWTVerify_NoBearerPrefix_Returns401(t *testing.T) {
	handler := middleware.JWTVerify(testSecret, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cve/CVE-2021-44228", nil)
	req.Header.Set("Authorization", "Basic abc123") // Not Bearer
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTVerify_ExpiredToken_Returns401(t *testing.T) {
	token := makeToken(t, "user-123", nil, testSecret, true)
	handler := middleware.JWTVerify(testSecret, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cve/CVE-2021-44228", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTVerify_WrongSecret_Returns401(t *testing.T) {
	token := makeToken(t, "user-123", nil, "wrong-secret", false)
	handler := middleware.JWTVerify(testSecret, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cve/CVE-2021-44228", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTVerify_SkipPublicPath_NoTokenRequired(t *testing.T) {
	handler := middleware.JWTVerify(testSecret, []string{"/health", "/api/v1/auth"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// /health should be accessible without token
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// /api/v1/auth/login should also skip
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestJWTVerify_MultipleRoles(t *testing.T) {
	token := makeToken(t, "admin-1", []string{"admin", "user"}, testSecret, false)
	var gotRoles string
	handler := middleware.JWTVerify(testSecret, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRoles = r.Header.Get("X-User-Roles")
			w.WriteHeader(http.StatusOK)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cve/CVE-2021-44228", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "admin,user", gotRoles)
}

// ── RequireRole tests ─────────────────────────────────────────────────────

func TestRequireRole_HasRole_Passes(t *testing.T) {
	token := makeToken(t, "admin-1", []string{"admin", "user"}, testSecret, false)
	handler := middleware.JWTVerify(testSecret, nil)(
		middleware.RequireRole("admin")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/trigger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_MissingRole_Returns403(t *testing.T) {
	token := makeToken(t, "user-1", []string{"user"}, testSecret, false)
	handler := middleware.JWTVerify(testSecret, nil)(
		middleware.RequireRole("admin")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/trigger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "forbidden")
}
