> **✅ COMPLETED** — go build && go vet passed.

# T17 — Security Hardening

## Thông tin
| | |
|---|---|
| **Phase** | 6 — Polish |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T04 (auth) |
| **Blocks** | T18 |

---

## Các bước thực hiện

### 17.1 JWT hardening

```go
// Thêm vào middleware/auth.go:

// 1. Kiểm tra token expiry tích cực
if claims.ExpiresAt.Before(time.Now()) {
    http.Error(w, `{"error":"token_expired"}`, http.StatusUnauthorized)
    return
}

// 2. Kiểm tra token revocation qua Redis
if a.sessionStore.IsRevoked(ctx, token) {
    http.Error(w, `{"error":"token_revoked"}`, http.StatusUnauthorized)
    return
}
```

### 17.2 Rate limiting

```go
import "golang.org/x/time/rate"

// Thêm vào router.go — rate limiter per IP
limiter := tollbooth.NewLimiter(10, nil) // 10 req/s per IP

r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow(r.RemoteAddr) {
            http.Error(w, `{"error":"rate_limit_exceeded"}`, http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
})
```

Hoặc dùng `github.com/ulule/limiter` với Redis backend.

### 17.3 Input validation

```go
// Thêm validation vào scan handler override (nếu scan-service chưa validate):
func validateScanRequest(req CreateScanRequest) error {
    if len(req.Targets) == 0 {
        return errors.New("targets cannot be empty")
    }
    if len(req.Targets) > 100 {
        return errors.New("max 100 targets per scan")
    }
    for _, t := range req.Targets {
        if !isValidTarget(t) {
            return fmt.Errorf("invalid target: %s", t)
        }
    }
    return nil
}

func isValidTarget(target string) bool {
    // Validate IP, CIDR, hostname
    if net.ParseIP(target) != nil { return true }
    if _, _, err := net.ParseCIDR(target); err == nil { return true }
    // Hostname validation
    return regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]{0,253}[a-zA-Z0-9]$`).MatchString(target)
}
```

### 17.4 CORS configuration (production)

```go
// Thay allowedOrigins từ "*" thành cụ thể:
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{"https://openvulnscan.example.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
    AllowCredentials: true,
    MaxAge:           300,
}))
```

### 17.5 Security headers middleware

```go
// internal/middleware/security_headers.go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        next.ServeHTTP(w, r)
    })
}
```

### 17.6 Request body size limit

```go
// Ngăn chặn large payload attacks
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
        next.ServeHTTP(w, r)
    })
})
```

### 17.7 API key authentication

```go
// Thêm support API key (từ auth-service/usecase/manage_api_key)
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Try JWT Bearer token
        if token := extractBearerToken(r); token != "" {
            if claims, err := m.validateUC.Execute(r.Context(), token); err == nil {
                next.ServeHTTP(w, r.WithContext(withUserContext(r.Context(), claims)))
                return
            }
        }

        // 2. Try API key header
        if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
            if user, err := m.apiKeyUC.ValidateAPIKey(r.Context(), apiKey); err == nil {
                next.ServeHTTP(w, r.WithContext(withUserContext(r.Context(), user)))
                return
            }
        }

        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
    })
}
```

### 17.8 Audit log tất cả write operations

```go
// Thêm middleware để log sensitive operations
func AuditLog(log zerolog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.Method != "GET" && r.Method != "OPTIONS" {
                userID := getUserIDFromContext(r.Context())
                log.Info().
                    Str("method", r.Method).
                    Str("path", r.URL.Path).
                    Str("user_id", userID.String()).
                    Str("ip", r.RemoteAddr).
                    Msg("audit")
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Output

- [x] JWT expiry + revocation check ✓ (auth_bridge.go: ExpiresAt, isJTIRevoked via Redis)
- [x] Rate limiting per IP ✓ (middleware/security.go: RateLimiter sliding window)
- [x] Input validation cho scan targets ✓ (ValidateScanTargets in security.go)
- [x] Security headers middleware ✓ (SecurityHeaders: X-Frame-Options, CSP, HSTS, etc.)
- [x] Request body size limit (1MB) ✓ (MaxBodySize in security.go)
- [x] API key authentication support ✓ (RequireAuthOrAPIKey: X-API-Key header → ValidateAPIKey gRPC)
- [x] Audit log middleware ✓ (AuditLog in security.go — logs all write ops)

## Acceptance Criteria

```bash
# Rate limiting
for i in $(seq 1 20); do
    curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/auth/login
done
# → sau ~10 requests: 429 Too Many Requests

# Security headers
curl -I http://localhost:8080/healthz
# → X-Content-Type-Options: nosniff
# → X-Frame-Options: DENY

# Large payload rejected
curl -X POST http://localhost:8080/api/v1/scans \
    -d "$(python3 -c "print('x'*2000000)")"
# → 413 Request Entity Too Large
```
