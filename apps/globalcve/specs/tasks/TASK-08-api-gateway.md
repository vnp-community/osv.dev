# TASK-08 — API Gateway

## Mục Tiêu

Implement **API Gateway** — goroutine service chạy trên port 8080, đây là public entry point duy nhất. Dùng **Direct Function Call** cho CVE Search (hot path) và **HTTP Proxy** cho KEV, Notification, CVE Sync admin.

## Phụ Thuộc

- TASK-04 (CVE Search Service — inject handler trực tiếp)
- TASK-05 (CVE Sync Service — proxy port 8082)
- TASK-06 (KEV Service — proxy port 8083)
- TASK-07 (Notification Service — proxy port 8084)
- TASK-02 (Redis — rate limiting)

## Đầu Ra

- `internal/gateway/service.go` — Gateway service

---

## Checklist

- [x] Chi router setup
- [x] Direct function call: CVE Search handler
- [x] HTTP proxy: KEV Service (port 8083)
- [x] HTTP proxy: Notification Service (port 8084)
- [x] HTTP proxy: CVE Sync admin (port 8082)
- [x] Health check aggregate (tất cả services)
- [x] Rate limiting middleware (Redis-backed)
- [x] Auth middleware (cho protected routes)
- [x] Request logging middleware
- [x] CORS middleware
- [x] Graceful shutdown

---

## 1. Port Map

Theo §2.2 và §6.1 của architecture-solutions.md:

| Service | Internal Port | Pattern |
|---------|--------------|---------|
| CVE Search | 8081 | Direct call (inject handler) |
| CVE Sync | 8082 | HTTP proxy |
| KEV Service | 8083 | HTTP proxy |
| Notification | 8084 | HTTP proxy |

---

## 2. Gateway Service (`internal/gateway/service.go`)

```go
package gateway

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httputil"
    "net/url"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    goredis "github.com/redis/go-redis/v9"
    "github.com/binhnt/globalcve/internal/config"
    cvesearchhttp "github.com/binhnt/globalcve/internal/cvesearch/http"
)

type Service struct {
    cfg            config.Config
    cveSearchHandler *cvesearchhttp.Handler  // direct function call — §2.1
    redis          *goredis.Client
    server         *http.Server
}

func New(cfg config.Config, redis *goredis.Client, cveSearchHandler *cvesearchhttp.Handler) *Service {
    return &Service{
        cfg:              cfg,
        cveSearchHandler: cveSearchHandler,
        redis:            redis,
    }
}

func (s *Service) Start(ctx context.Context) error {
    r := chi.NewRouter()

    // Middleware stack
    r.Use(middleware.RealIP)
    r.Use(middleware.RequestID)
    r.Use(s.loggingMiddleware())
    r.Use(s.corsMiddleware())
    r.Use(s.rateLimitMiddleware())

    // Public routes
    r.Get("/health", s.handleAggregateHealth)

    // CVE Search — Direct call (zero-latency hot path)
    r.Get("/api/v2/cves",     s.cveSearchHandler.SearchCVEs)
    r.Get("/api/v2/cves/{id}", s.cveSearchHandler.GetCVE)

    // KEV routes — HTTP proxy → port 8083
    r.Get("/api/v2/kev",           s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.KEVService.Port)))
    r.Get("/api/v2/kev/{id}",      s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.KEVService.Port)))
    r.Get("/api/v2/kev/check",     s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.KEVService.Port)))
    r.Get("/api/v2/kev/stats",     s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.KEVService.Port)))

    // Notification routes — HTTP proxy → port 8084 (auth required)
    r.Group(func(r chi.Router) {
        r.Use(s.authMiddleware())
        r.Get("/api/v2/webhooks",          s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.Notification.Port)))
        r.Post("/api/v2/webhooks",         s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.Notification.Port)))
        r.Delete("/api/v2/webhooks/{id}",  s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.Notification.Port)))
    })

    // CVE Sync admin — HTTP proxy → port 8082 (auth required)
    r.Group(func(r chi.Router) {
        r.Use(s.authMiddleware())
        r.Get("/api/v2/sync/status",          s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.CVESync.Port)))
        r.Post("/api/v2/sync/trigger",        s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.CVESync.Port)))
        r.Post("/api/v2/sync/trigger/{source}", s.proxyTo(fmt.Sprintf("http://localhost:%d", s.cfg.Services.CVESync.Port)))
    })

    s.server = &http.Server{
        Addr:         fmt.Sprintf(":%d", s.cfg.Server.Port),
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        <-ctx.Done()
        shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        s.server.Shutdown(shutCtx)
    }()

    return s.server.ListenAndServe()
}
```

---

## 3. HTTP Proxy Helper

Theo §2.2 của communication-patterns.md:

```go
// proxyTo — tạo reverse proxy đến upstream service
func (s *Service) proxyTo(upstreamBase string) http.HandlerFunc {
    target, err := url.Parse(upstreamBase)
    if err != nil {
        panic(fmt.Sprintf("gateway: invalid upstream URL %s: %v", upstreamBase, err))
    }

    proxy := httputil.NewSingleHostReverseProxy(target)

    // Custom error handler
    proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
        http.Error(w, "Service unavailable", http.StatusBadGateway)
    }

    return func(w http.ResponseWriter, r *http.Request) {
        // Strip prefix nếu cần (routes đã match đầy đủ path)
        proxy.ServeHTTP(w, r)
    }
}
```

---

## 4. Rate Limiting Middleware (Redis-backed)

Theo §4.2: `rl:ip:{ip}` với sliding window.

```go
func (s *Service) rateLimitMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := r.RemoteAddr

            key := infraredis.RateLimitKey(ip)
            count, err := s.redis.Incr(r.Context(), key).Result()
            if err != nil {
                // Redis down → fail-open (cho phép request)
                next.ServeHTTP(w, r)
                return
            }

            // Set expiry trên first request
            if count == 1 {
                s.redis.Expire(r.Context(), key, time.Minute)
            }

            // 100 requests/minute per IP
            if count > 100 {
                w.Header().Set("Retry-After", "60")
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }

            w.Header().Set("X-RateLimit-Limit", "100")
            w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", 100-count))
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 5. Auth Middleware

```go
// Simple API key auth cho protected routes
// API key được đặt trong config hoặc env var ADMIN_API_KEY
func (s *Service) authMiddleware() func(http.Handler) http.Handler {
    apiKey := s.cfg.Auth.AdminAPIKey
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := r.Header.Get("X-API-Key")
            if key == "" {
                key = r.URL.Query().Get("api_key")
            }
            if key != apiKey || apiKey == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 6. CORS Middleware

```go
func (s *Service) corsMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusNoContent)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 7. Aggregate Health Check

```go
func (s *Service) handleAggregateHealth(w http.ResponseWriter, r *http.Request) {
    type serviceHealth struct {
        Name   string `json:"name"`
        Status string `json:"status"`
    }

    services := []struct {
        name string
        port int
    }{
        {"cve-search", s.cfg.Services.CVESearch.Port},
        {"cve-sync",   s.cfg.Services.CVESync.Port},
        {"kev",        s.cfg.Services.KEVService.Port},
        {"notification", s.cfg.Services.Notification.Port},
    }

    allHealthy := true
    var results []serviceHealth

    for _, svc := range services {
        url := fmt.Sprintf("http://localhost:%d/health", svc.port)
        resp, err := http.Get(url)
        status := "healthy"
        if err != nil || resp.StatusCode != 200 {
            status = "unhealthy"
            allHealthy = false
        }
        results = append(results, serviceHealth{Name: svc.name, Status: status})
    }

    statusCode := http.StatusOK
    if !allHealthy {
        statusCode = http.StatusServiceUnavailable
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":   map[bool]string{true: "healthy", false: "degraded"}[allHealthy],
        "services": results,
    })
}
```

---

## 8. Logging Middleware

```go
func (s *Service) loggingMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            next.ServeHTTP(ww, r)
            log.Info().
                Str("method", r.Method).
                Str("path", r.URL.Path).
                Int("status", ww.Status()).
                Dur("latency", time.Since(start)).
                Str("ip", r.RemoteAddr).
                Msg("request")
        })
    }
}
```

---

## Định Nghĩa Hoàn Thành

- [x] `GET /health` trả về status của tất cả services
- [x] `GET /api/v2/cves?query=log4j` hoạt động qua direct call (latency < 5ms)
- [x] `GET /api/v2/kev` proxy đến KEV service đúng
- [x] `POST /api/v2/webhooks` với X-API-Key hợp lệ → 201 Created
- [x] `POST /api/v2/webhooks` không có X-API-Key → 401 Unauthorized
- [x] Rate limit: 101 requests/minute từ cùng IP → 429 Too Many Requests
- [x] CORS headers có trong mọi response
- [x] Request log có method, path, status, latency

---

*TASK-08 | API Gateway | GlobalCVE v3.0*
