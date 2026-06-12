# TASK-05: API Gateway — HTTP Handlers

**Phase**: 5 — HTTP API Layer  
**Ước tính**: 24 giờ  
**Phụ thuộc**: TASK-04 (gateway runner skeleton)  
**Output**: Toàn bộ REST API tương thích DefectDojo v2

---

## Mục tiêu

Implement HTTP REST API gateway hoàn toàn tương thích với **Django DefectDojo API v2**.
Tất cả clients, tools, CI/CD plugins của DefectDojo phải hoạt động với API này mà không cần thay đổi.

---

## T-05.1: Router & Middleware

**File**: `apps/DefectDojo/internal/gateway/router.go`  
**Ước tính**: 2h

```go
package gateway

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServiceClients holds gRPC clients for all backend services.
type ServiceClients struct {
    Auth         authpb.AuthServiceClient
    Product      productpb.ProductServiceClient
    Finding      findingpb.FindingServiceClient
    Scan         scanpb.ScanServiceClient
    Vuln         vulnpb.VulnerabilityServiceClient
    Search       searchpb.SearchServiceClient
    Notification notifpb.NotificationServiceClient
    Report       reportpb.ReportServiceClient
    Integration  integpb.IntegrationServiceClient
}

func NewRouter(svc *ServiceClients) http.Handler {
    r := chi.NewRouter()

    // ── Global Middleware ──────────────────────────────────────────────────
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(requestLogger)         // zerolog structured logging
    r.Use(middleware.Recoverer)
    r.Use(prometheusMiddleware)  // HTTP metrics
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
        ExposedHeaders:   []string{"Link"},
        AllowCredentials: true,
        MaxAge:           300,
    }))

    // ── Health & Observability ─────────────────────────────────────────────
    r.Get("/health",       healthHandler(svc))
    r.Get("/health/ready", readyHandler(svc))
    r.Get("/metrics",      promhttp.Handler())

    // ── DefectDojo API v2 ──────────────────────────────────────────────────
    r.Route("/api/v2", func(r chi.Router) {
        // Auth middleware: JWT or API key
        r.Use(authMiddleware(svc.Auth))

        // Mount all route groups
        mountAuth(r, svc)
        mountProducts(r, svc)
        mountEngagements(r, svc)
        mountTests(r, svc)
        mountScanImport(r, svc)
        mountFindings(r, svc)
        mountFindingGroups(r, svc)
        mountRiskAcceptances(r, svc)
        mountEndpoints(r, svc)
        mountNotifications(r, svc)
        mountAlerts(r, svc)
        mountReports(r, svc)
        mountSearch(r, svc)
        mountJIRA(r, svc)
        mountTools(r, svc)
        mountSLA(r, svc)
        mountSystem(r, svc)
    })

    return r
}
```

**Tasks**:
- [ ] Router setup với chi
- [ ] CORS configuration
- [ ] Request ID middleware
- [ ] Prometheus middleware (`prometheusMiddleware`)
- [ ] Zerolog request logger middleware
- [ ] Health/ready handlers
- [ ] `go build` pass

---

## T-05.2: Auth Middleware

**File**: `apps/DefectDojo/internal/gateway/middleware.go`  
**Ước tính**: 2h

```go
package gateway

import (
    "context"
    "net/http"
    "strings"

    authpb "github.com/osv/shared/proto/gen/go/auth/v1"
)

type contextKey string

const claimsKey contextKey = "user_claims"

// UserClaims holds validated auth claims.
type UserClaims struct {
    UserID      string
    Role        string
    Permissions []string
}

// authMiddleware validates JWT or API key on every request.
func authMiddleware(authClient authpb.AuthServiceClient) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractBearerToken(r)
            if token == "" {
                http.Error(w, `{"detail":"Authentication credentials were not provided."}`, http.StatusUnauthorized)
                return
            }

            var claims *UserClaims
            var authErr string

            if strings.HasPrefix(token, "ovs_") || strings.HasPrefix(token, "dd_") {
                // API Key validation
                resp, err := authClient.ValidateAPIKey(r.Context(), &authpb.ValidateAPIKeyRequest{ApiKey: token})
                if err != nil || !resp.Valid {
                    authErr = "Invalid API key"
                    if resp != nil {
                        authErr = resp.Error
                    }
                } else {
                    claims = &UserClaims{UserID: resp.UserId, Permissions: resp.Permissions}
                }
            } else {
                // JWT validation
                resp, err := authClient.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{Token: token})
                if err != nil || !resp.Valid {
                    authErr = "Invalid or expired token"
                    if resp != nil {
                        authErr = resp.Error
                    }
                } else {
                    claims = &UserClaims{UserID: resp.UserId, Role: resp.Role, Permissions: resp.Permissions}
                }
            }

            if claims == nil {
                w.Header().Set("Content-Type", "application/json")
                http.Error(w, `{"detail":"`+authErr+`"}`, http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), claimsKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func extractBearerToken(r *http.Request) string {
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    if strings.HasPrefix(auth, "Token ") {
        return strings.TrimPrefix(auth, "Token ")
    }
    return r.Header.Get("X-Api-Key") // Alternative header
}
```

**Tasks**:
- [ ] JWT validation via auth-service gRPC
- [ ] API key validation
- [ ] Claims injection into context
- [ ] 401 error format matches DefectDojo

---

## T-05.3: Common Response Helpers

**File**: `apps/DefectDojo/internal/gateway/response.go`  
**Ước tính**: 1h

```go
package gateway

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
)

// DDPaginatedResponse matches DefectDojo's standard paginated API response.
type DDPaginatedResponse struct {
    Count    int         `json:"count"`
    Next     *string     `json:"next"`
    Previous *string     `json:"previous"`
    Results  interface{} `json:"results"`
}

func jsonResponse(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, detail string) {
    jsonResponse(w, status, map[string]string{"detail": detail})
}

func paginatedResponse(w http.ResponseWriter, r *http.Request, count int, results interface{}, limit, offset int) {
    var next, prev *string
    if offset+limit < count {
        n := buildURL(r, limit, offset+limit)
        next = &n
    }
    if offset > 0 {
        prevOffset := offset - limit
        if prevOffset < 0 {
            prevOffset = 0
        }
        p := buildURL(r, limit, prevOffset)
        prev = &p
    }

    jsonResponse(w, http.StatusOK, DDPaginatedResponse{
        Count:    count,
        Next:     next,
        Previous: prev,
        Results:  results,
    })
}

func parsePageParams(r *http.Request) (limit, offset int) {
    limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
    if limit <= 0 || limit > 1000 {
        limit = 100
    }
    if offset < 0 {
        offset = 0
    }
    return
}

func buildURL(r *http.Request, limit, offset int) string {
    q := r.URL.Query()
    q.Set("limit", strconv.Itoa(limit))
    q.Set("offset", strconv.Itoa(offset))
    u := *r.URL
    u.RawQuery = q.Encode()
    return u.String()
}
```

---

## T-05.4: Finding Handlers ⭐ Most Complex

**File**: `apps/DefectDojo/internal/gateway/handlers/finding.go`  
**Ước tính**: 4h

### Sub-tasks:
- [ ] `GET /api/v2/findings/` — List với filtering (product, engagement, severity, active, ...)
- [ ] `POST /api/v2/findings/` — Create finding manually
- [ ] `GET /api/v2/findings/{id}/` — Get finding detail
- [ ] `PUT /api/v2/findings/{id}/` — Full update
- [ ] `PATCH /api/v2/findings/{id}/` — Partial update
- [ ] `DELETE /api/v2/findings/{id}/` — Delete
- [ ] `POST /api/v2/findings/{id}/close/` — Close finding
- [ ] `POST /api/v2/findings/{id}/reopen/` — Reopen
- [ ] `POST /api/v2/findings/{id}/accept_risks/` — Accept risk
- [ ] `POST /api/v2/findings/{id}/push_to_jira/` — Push to JIRA
- [ ] `GET/POST /api/v2/findings/{id}/notes/` — Notes
- [ ] `PUT /api/v2/findings/bulk_update/` — Bulk update

**Filtering params**: product, engagement, test, severity, active, verified, duplicate, false_positive, tags, component_name, cve, search, o (ordering), limit, offset

**Response format** must match DefectDojo exactly (numerical_severity as "S0"-"S4", dates as ISO 8601, etc.)

---

## T-05.5: Scan Import Handlers ⭐ Critical

**File**: `apps/DefectDojo/internal/gateway/handlers/scan_import.go`  
**Ước tính**: 3h

### Sub-tasks:
- [ ] `POST /api/v2/import-scan-results/` — Parse multipart form, pass to scan-service
  - Fields: scan_type, file, engagement, test_title, active, verified, minimum_severity, ...
  - Return: `{test_id, engagement_id, finding_count}`
- [ ] `POST /api/v2/reimport-scan-results/` — Reimport to existing test
- [ ] File upload handling (up to 500MB scan files)
- [ ] Async processing: submit to scan-service → return 202 Accepted + job_id
- [ ] OR sync processing: wait for result (timeout 120s) → return 201

---

## T-05.6: Product Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/product.go`  
**Ước tính**: 2h

### Sub-tasks:
- [ ] `GET/POST /api/v2/products/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/products/{id}/`
- [ ] `GET /api/v2/products/{id}/findings/`
- [ ] `GET/POST /api/v2/product_types/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/product_types/{id}/`
- [ ] `GET/POST /api/v2/product_members/`
- [ ] `DELETE /api/v2/product_members/{id}/`

---

## T-05.7: Engagement & Test Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/engagement.go`  
**Ước tính**: 2h

### Sub-tasks:
- [ ] `GET/POST /api/v2/engagements/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/engagements/{id}/`
- [ ] `POST /api/v2/engagements/{id}/close/`
- [ ] `POST /api/v2/engagements/{id}/reopen/`
- [ ] `GET/POST /api/v2/tests/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/tests/{id}/`
- [ ] `GET/POST /api/v2/test_types/`

---

## T-05.8: Auth & User Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/auth.go`  
**Ước tính**: 2h

### Sub-tasks:
- [ ] `POST /api/v2/api-token-auth/` — Login (return JWT)
- [ ] `POST /api/v2/auth/login/` — Login with 2FA check
- [ ] `POST /api/v2/auth/logout/` — Logout (blacklist token)
- [ ] `POST /api/v2/auth/refresh/` — Refresh token
- [ ] `GET/POST /api/v2/users/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/users/{id}/`
- [ ] `GET /api/v2/users/me/`
- [ ] `GET/POST /api/v2/api-keys/`
- [ ] `DELETE /api/v2/api-keys/{id}/`

---

## T-05.9: Notification Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/notification.go`  
**Ước tính**: 1.5h

### Sub-tasks:
- [ ] `GET/POST /api/v2/notifications/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/notifications/{id}/`
- [ ] `GET /api/v2/alerts/`
- [ ] `DELETE /api/v2/alerts/{id}/`

---

## T-05.10: Report Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/report.go`  
**Ước tính**: 2h

### Sub-tasks:
- [ ] `GET /api/v2/reports/`
- [ ] `POST /api/v2/reports/` — Generate report (async, return job_id)
- [ ] `GET /api/v2/reports/{id}/` — Download report
- [ ] `DELETE /api/v2/reports/{id}/`
- [ ] Stream large reports via HTTP chunked transfer

---

## T-05.11: JIRA Integration Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/jira.go`  
**Ước tính**: 1.5h

### Sub-tasks:
- [ ] `GET/POST /api/v2/jira_instances/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/jira_instances/{id}/`
- [ ] `GET/POST /api/v2/jira_projects/`
- [ ] `GET/PUT/PATCH/DELETE /api/v2/jira_projects/{id}/`
- [ ] `GET /api/v2/jira_issues/`

---

## T-05.12: System & Misc Handlers

**File**: `apps/DefectDojo/internal/gateway/handlers/system.go`  
**Ước tính**: 1h

### Sub-tasks:
- [ ] `GET/PUT /api/v2/system_settings/` (singleton)
- [ ] `GET /api/v2/development_environments/`
- [ ] `GET /api/v2/regulations/`
- [ ] `GET /api/v2/note_types/`
- [ ] `GET /api/v2/tags/`
- [ ] `GET /api/v2/search/`
- [ ] `GET/POST /api/v2/sla_configurations/`
- [ ] `GET/POST /api/v2/tool_configurations/`
- [ ] `GET/POST /api/v2/tool_types/`
- [ ] `GET/POST /api/v2/endpoints/`
- [ ] `GET/POST /api/v2/risk_acceptances/`
- [ ] `GET/POST /api/v2/finding_groups/`

---

## T-05.13: API Compatibility Verification

**Ước tính**: 2h

```bash
# Sử dụng DefectDojo's own Python client để test
pip install defectdojo-api-client
python3 - << 'EOF'
import defectdojo_api
api = defectdojo_api.DefectDojoAPI(
    host='http://localhost:8080',
    api_key='your-api-key',
    verify_ssl=False
)

# Test core flows
products = api.list_products()
assert products.success, f"List products failed: {products.message}"
print(f"Products: {products.data['count']}")

findings = api.list_findings()
assert findings.success
print("API compatibility verified!")
EOF
```

- [ ] DefectDojo Python client tests pass
- [ ] DefectDojo Jenkins plugin compatible
- [ ] Pagination headers correct
- [ ] Error format matches (detail field)
- [ ] Date/datetime format ISO 8601

---

## Definition of Done — TASK-05

- [ ] T-05.1 Router + all middleware
- [ ] T-05.2 Auth middleware (JWT + API key)
- [ ] T-05.3 Response helpers (pagination, errors)
- [ ] T-05.4 Finding handlers (CRUD + bulk + notes)
- [ ] T-05.5 Scan import handlers (multipart + async)
- [ ] T-05.6 Product handlers
- [ ] T-05.7 Engagement & test handlers
- [ ] T-05.8 Auth handlers
- [ ] T-05.9 Notification handlers
- [ ] T-05.10 Report handlers
- [ ] T-05.11 JIRA handlers
- [ ] T-05.12 System handlers
- [ ] T-05.13 API compatibility verified
- [ ] `go build ./...` pass
- [ ] Unit tests cho handlers
