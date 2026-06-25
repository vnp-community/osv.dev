# ✅ COMPLETED — TASK-DD-014 — OpenAPI Aggregation + Error Standardization

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-014 |
| **Service** | `apps/osv` |
| **CR** | CR-DD-011 |
| **Phase** | 1 — Foundation |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-013 |
| **Estimated effort** | 0.5 ngày |

## Context

Implement `GET /api/v2/schema` để aggregate OpenAPI specs từ tất cả downstream services. Cũng standardize error responses theo DefectDojo format `{"detail": "..."}`.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/
```

## Files to Create

```
internal/gateway/
├── openapi/
│   └── aggregator.go       # Fetch + merge OpenAPI specs
└── errors/
    └── handler.go          # Standard error response helpers
```

## Implementation Spec

### `internal/gateway/openapi/aggregator.go`

```go
package openapi

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

type ServiceSpec struct {
    Name    string
    SpecURL string  // e.g., "http://finding-service:8085/openapi.json"
}

var serviceSpecs = []ServiceSpec{
    {"finding", "http://finding-service:8085/openapi.json"},
    {"scan", "http://scan-service:8084/openapi.json"},
    {"sla", "http://sla-service:8086/openapi.json"},
    {"notification", "http://notification-service:8087/openapi.json"},
    {"jira", "http://jira-service:8088/openapi.json"},
    {"audit", "http://audit-service:8090/openapi.json"},
}

type OpenAPIAggregator struct {
    services []ServiceSpec
    cached   []byte
    cachedAt time.Time
    mu       sync.RWMutex
    ttl      time.Duration  // default 1h
    client   *http.Client
}

func NewOpenAPIAggregator() *OpenAPIAggregator {
    return &OpenAPIAggregator{
        services: serviceSpecs,
        ttl:      time.Hour,
        client:   &http.Client{Timeout: 10 * time.Second},
    }
}

// GetAggregatedSpec returns merged OpenAPI 3.0 JSON (cached)
func (a *OpenAPIAggregator) GetAggregatedSpec(ctx context.Context) ([]byte, error) {
    a.mu.RLock()
    if a.cached != nil && time.Since(a.cachedAt) < a.ttl {
        defer a.mu.RUnlock()
        return a.cached, nil
    }
    a.mu.RUnlock()

    a.mu.Lock()
    defer a.mu.Unlock()

    merged := map[string]interface{}{
        "openapi": "3.0.3",
        "info": map[string]interface{}{
            "title":   "OpenVulnScan API",
            "version": "2.0",
            "description": "Unified API — CVE Search + DefectDojo capabilities",
        },
        "paths":      map[string]interface{}{},
        "components": map[string]interface{}{"schemas": map[string]interface{}{}},
    }

    paths := merged["paths"].(map[string]interface{})
    schemas := merged["components"].(map[string]interface{})["schemas"].(map[string]interface{})

    for _, svc := range a.services {
        spec, err := a.fetchSpec(ctx, svc.SpecURL)
        if err != nil {
            continue // skip unavailable services
        }
        // Merge paths
        if p, ok := spec["paths"].(map[string]interface{}); ok {
            for path, item := range p {
                paths[path] = item
            }
        }
        // Merge schemas (prefixed with service name to avoid collisions)
        if comp, ok := spec["components"].(map[string]interface{}); ok {
            if s, ok := comp["schemas"].(map[string]interface{}); ok {
                for name, schema := range s {
                    schemas[fmt.Sprintf("%s_%s", svc.Name, name)] = schema
                }
            }
        }
    }

    data, err := json.MarshalIndent(merged, "", "  ")
    if err != nil {
        return nil, err
    }

    a.cached = data
    a.cachedAt = time.Now()
    return data, nil
}

func (a *OpenAPIAggregator) fetchSpec(ctx context.Context, url string) (map[string]interface{}, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := a.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)

    var spec map[string]interface{}
    return spec, json.Unmarshal(body, &spec)
}

// HandleSchema is the HTTP handler for GET /api/v2/schema
func (a *OpenAPIAggregator) HandleSchema(w http.ResponseWriter, r *http.Request) {
    data, err := a.GetAggregatedSpec(r.Context())
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "public, max-age=3600")
    w.Write(data)
}
```

### `internal/gateway/errors/handler.go`

```go
package errors

import (
    "encoding/json"
    "net/http"
)

// ErrorResponse is the standard error format (DefectDojo compatible)
type ErrorResponse struct {
    Detail string `json:"detail"`
}

// Standard error responses
var (
    ErrUnauthorized = &ErrorResponse{Detail: "Authentication credentials were not provided."}
    ErrForbidden    = &ErrorResponse{Detail: "You do not have permission to perform this action."}
    ErrNotFound     = &ErrorResponse{Detail: "Not found."}
    ErrBadRequest   = &ErrorResponse{Detail: "Invalid request."}
    ErrThrottled    = &ErrorResponse{Detail: "Request was throttled."}
    ErrUnavailable  = &ErrorResponse{Detail: "Service temporarily unavailable."}
)

func WriteError(w http.ResponseWriter, status int, detail string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(ErrorResponse{Detail: detail})
}

func Write401(w http.ResponseWriter) {
    WriteError(w, http.StatusUnauthorized, ErrUnauthorized.Detail)
}
func Write403(w http.ResponseWriter) {
    WriteError(w, http.StatusForbidden, ErrForbidden.Detail)
}
func Write404(w http.ResponseWriter) {
    WriteError(w, http.StatusNotFound, ErrNotFound.Detail)
}
func Write503(w http.ResponseWriter) {
    WriteError(w, http.StatusServiceUnavailable, ErrUnavailable.Detail)
}
```

## Acceptance Criteria

- [x] `GET /api/v2/schema` → 200 với merged OpenAPI JSON
- [x] Schema có paths từ finding-service (e.g., `/api/v2/findings`)
- [x] Schema có paths từ scan-service (e.g., `/api/v2/import-scan`)
- [x] Schema cached — second request không gọi upstream lại (TTL 1h)
- [x] One service unavailable → schema vẫn trả về (chỉ thiếu paths của service đó)
- [x] `Content-Type: application/json` header set
- [x] `Cache-Control: public, max-age=3600` header set
- [x] Unauthorized → `{"detail": "Authentication credentials were not provided."}`
- [x] Service unavailable → `{"detail": "Service temporarily unavailable."}`
- [x] Rate limited → `{"detail": "Request was throttled."}`

## Implementation Status: ✅ DONE

> `apps/osv/internal/gateway/openapi/aggregator.go` — OpenAPIAggregator, GetAggregatedSpec (TTL cache 1h), fetchSpec, HandleSchema
> `apps/osv/internal/gateway/gwerrors/handler.go` — WriteError, Write401/403/404/503 standard error helpers
> 6 downstream services aggregated: finding, scan, sla, notification, jira, audit
