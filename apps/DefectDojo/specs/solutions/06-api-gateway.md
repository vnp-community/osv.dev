# REST API Gateway — DefectDojo v2 Compatible

## Thiết kế API Gateway

API Gateway (`unified-gateway`) expose REST endpoints hoàn toàn tương thích với **Django DefectDojo API v2**, cho phép tái sử dụng tất cả DefectDojo clients, CI/CD plugins và tools hiện có.

## Endpoint Reference

### Authentication Endpoints

```
POST   /api/v2/api-token-auth/          → Login (JWT)
POST   /api/v2/auth/login/              → Login with 2FA support
POST   /api/v2/auth/logout/             → Logout
POST   /api/v2/auth/refresh/            → Refresh JWT
POST   /api/v2/auth/password/reset/     → Password reset
GET    /api/v2/users/                   → List users
POST   /api/v2/users/                   → Create user
GET    /api/v2/users/{id}/              → Get user
PUT    /api/v2/users/{id}/              → Update user
DELETE /api/v2/users/{id}/              → Delete user
GET    /api/v2/users/me/                → Current user profile
GET    /api/v2/api-keys/                → List API keys
POST   /api/v2/api-keys/                → Create API key
DELETE /api/v2/api-keys/{id}/           → Delete API key
```

### Product Endpoints

```
GET    /api/v2/products/                → List products
POST   /api/v2/products/                → Create product
GET    /api/v2/products/{id}/           → Get product
PUT    /api/v2/products/{id}/           → Update product
PATCH  /api/v2/products/{id}/           → Partial update
DELETE /api/v2/products/{id}/           → Delete product
GET    /api/v2/products/{id}/findings/  → Product findings

GET    /api/v2/product_types/           → List product types
POST   /api/v2/product_types/           → Create product type
GET    /api/v2/product_types/{id}/      → Get product type
PUT    /api/v2/product_types/{id}/      → Update product type
DELETE /api/v2/product_types/{id}/      → Delete product type

GET    /api/v2/product_members/         → List product members
POST   /api/v2/product_members/         → Add member
DELETE /api/v2/product_members/{id}/    → Remove member
```

### Engagement Endpoints

```
GET    /api/v2/engagements/             → List engagements
POST   /api/v2/engagements/             → Create engagement
GET    /api/v2/engagements/{id}/        → Get engagement
PUT    /api/v2/engagements/{id}/        → Update engagement
PATCH  /api/v2/engagements/{id}/        → Partial update
DELETE /api/v2/engagements/{id}/        → Delete engagement
POST   /api/v2/engagements/{id}/close/  → Close engagement
POST   /api/v2/engagements/{id}/reopen/ → Reopen engagement
GET    /api/v2/engagements/{id}/findings/ → Engagement findings
```

### Test Endpoints

```
GET    /api/v2/tests/                   → List tests
POST   /api/v2/tests/                   → Create test
GET    /api/v2/tests/{id}/              → Get test
PUT    /api/v2/tests/{id}/              → Update test
PATCH  /api/v2/tests/{id}/              → Partial update
DELETE /api/v2/tests/{id}/              → Delete test

GET    /api/v2/test_types/              → List test types
POST   /api/v2/test_types/              → Create test type
GET    /api/v2/test_types/{id}/         → Get test type
```

### Scan Import Endpoints

```
POST   /api/v2/import-scan-results/     → Import scan (multipart form)
POST   /api/v2/reimport-scan-results/   → Re-import scan
POST   /api/v2/import-languages/        → Import language stats

# Request format (multipart/form-data):
# scan_type: "Trivy Scan" | "Semgrep JSON Report" | "Snyk Scan" | ...
# file: <scan output file>
# engagement: <engagement_id>
# product_name: "My App" (alternative to engagement)
# test_title: "CI Scan 2024-01-01"
# minimum_severity: "Info" | "Low" | "Medium" | "High" | "Critical"
# active: true
# verified: false
# close_old_findings: true
# deduplication_on_engagement: true
# push_to_jira: false
# version: "1.2.3"
# branch_tag: "main"
# commit_hash: "abc123"
```

### Finding Endpoints

```
GET    /api/v2/findings/                → List findings (paginated, filterable)
POST   /api/v2/findings/                → Create finding manually
GET    /api/v2/findings/{id}/           → Get finding detail
PUT    /api/v2/findings/{id}/           → Update finding
PATCH  /api/v2/findings/{id}/           → Partial update
DELETE /api/v2/findings/{id}/           → Delete finding

POST   /api/v2/findings/{id}/close/            → Close finding
POST   /api/v2/findings/{id}/reopen/           → Reopen finding
POST   /api/v2/findings/{id}/accept_risks/     → Accept risk
POST   /api/v2/findings/{id}/push_to_jira/     → Push to JIRA
GET    /api/v2/findings/{id}/jira/             → Get linked JIRA issue
GET    /api/v2/findings/{id}/notes/            → Get notes
POST   /api/v2/findings/{id}/notes/            → Add note

# Bulk actions
PUT    /api/v2/findings/bulk_update/    → Bulk update (status, severity, etc.)
DELETE /api/v2/findings/bulk_delete/    → Bulk delete

# Filtering params:
# ?product={id}&engagement={id}&test={id}
# ?severity=Critical,High
# ?active=true&verified=true
# ?duplicate=false
# ?tags=frontend,api
# ?component_name=log4j
# ?cve=CVE-2021-44228
# ?o=-severity (ordering)
# ?limit=100&offset=0
```

### Finding Group Endpoints

```
GET    /api/v2/finding_groups/          → List finding groups
POST   /api/v2/finding_groups/          → Create group
GET    /api/v2/finding_groups/{id}/     → Get group
DELETE /api/v2/finding_groups/{id}/     → Delete group
POST   /api/v2/finding_groups/{id}/add_finding/    → Add finding to group
POST   /api/v2/finding_groups/{id}/remove_finding/ → Remove finding
POST   /api/v2/finding_groups/{id}/push_to_jira/   → Push group to JIRA
```

### Risk Acceptance Endpoints

```
GET    /api/v2/risk_acceptances/        → List risk acceptances
POST   /api/v2/risk_acceptances/        → Create risk acceptance
GET    /api/v2/risk_acceptances/{id}/   → Get risk acceptance
PUT    /api/v2/risk_acceptances/{id}/   → Update risk acceptance
DELETE /api/v2/risk_acceptances/{id}/   → Delete risk acceptance
POST   /api/v2/risk_acceptances/{id}/add_finding/    → Add finding
POST   /api/v2/risk_acceptances/{id}/remove_finding/ → Remove finding
```

### Endpoint (Attack Surface) Endpoints

```
GET    /api/v2/endpoints/               → List endpoints
POST   /api/v2/endpoints/              → Create endpoint
GET    /api/v2/endpoints/{id}/          → Get endpoint
PUT    /api/v2/endpoints/{id}/          → Update endpoint
DELETE /api/v2/endpoints/{id}/          → Delete endpoint
POST   /api/v2/endpoint_status/         → Update endpoint status
```

### Notification Endpoints

```
GET    /api/v2/notifications/           → List notification rules
POST   /api/v2/notifications/           → Create notification rule
GET    /api/v2/notifications/{id}/      → Get rule
PUT    /api/v2/notifications/{id}/      → Update rule
DELETE /api/v2/notifications/{id}/      → Delete rule

GET    /api/v2/alerts/                  → List alerts (user inbox)
DELETE /api/v2/alerts/{id}/             → Dismiss alert
```

### Report Endpoints

```
GET    /api/v2/reports/                 → List generated reports
POST   /api/v2/reports/                 → Generate report
GET    /api/v2/reports/{id}/            → Download report
DELETE /api/v2/reports/{id}/            → Delete report

# Report options:
# report_type: "Product" | "Engagement" | "Test" | "Findings List" | "Custom"
# include_finding_notes: true
# include_finding_images: false
# include_executive_summary: true
# include_table_of_contents: true
# include_findings: true
# by_severity: true
# format: "html" | "pdf" | "csv" | "json"
```

### Search Endpoints

```
GET    /api/v2/search/                  → Global search
# ?q=log4j&type=finding,product,engagement
```

### JIRA Integration Endpoints

```
GET    /api/v2/jira_instances/          → List JIRA instances
POST   /api/v2/jira_instances/          → Create JIRA instance
GET    /api/v2/jira_instances/{id}/     → Get instance
PUT    /api/v2/jira_instances/{id}/     → Update instance

GET    /api/v2/jira_projects/           → List JIRA projects
POST   /api/v2/jira_projects/           → Link project
GET    /api/v2/jira_projects/{id}/      → Get project config
PUT    /api/v2/jira_projects/{id}/      → Update config

GET    /api/v2/jira_issues/             → List JIRA issues
POST   /api/v2/jira_issues/             → Create/link issue
```

### Tool Endpoints

```
GET    /api/v2/tool_types/              → List tool types
POST   /api/v2/tool_types/              → Create tool type

GET    /api/v2/tool_configurations/     → List tool configs
POST   /api/v2/tool_configurations/     → Create config
GET    /api/v2/tool_configurations/{id}/ → Get config
PUT    /api/v2/tool_configurations/{id}/ → Update config

GET    /api/v2/tool_product_settings/   → List tool-product settings
POST   /api/v2/tool_product_settings/   → Create setting
```

### SLA Endpoints

```
GET    /api/v2/sla_configurations/      → List SLA configs
POST   /api/v2/sla_configurations/      → Create SLA config
GET    /api/v2/sla_configurations/{id}/ → Get config
PUT    /api/v2/sla_configurations/{id}/ → Update config
DELETE /api/v2/sla_configurations/{id}/ → Delete config
```

### System Endpoints

```
GET    /api/v2/system_settings/         → Get system settings (singleton)
PUT    /api/v2/system_settings/1/       → Update system settings

GET    /api/v2/development_environments/ → List environments
POST   /api/v2/development_environments/ → Create environment

GET    /api/v2/regulations/             → List regulations
GET    /api/v2/note_types/              → List note types
GET    /api/v2/tags/                    → List all tags

GET    /health                          → Health check
GET    /metrics                         → Prometheus metrics
```

## Handler Implementation Pattern

```go
// apps/DefectDojo/internal/gateway/finding_handler.go

package gateway

import (
    "encoding/json"
    "net/http"
    
    findingpb "github.com/defectdojo/proto/finding/v1"
    "github.com/go-chi/chi/v5"
)

type FindingHandler struct {
    client findingpb.FindingServiceClient
    auth   *AuthMiddleware
}

// List handles GET /api/v2/findings/
func (h *FindingHandler) List(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    
    req := &findingpb.ListFindingsRequest{
        ProductId:    q.Get("product"),
        EngagementId: q.Get("engagement"),
        TestId:       q.Get("test"),
        Severity:     q["severity"],
        ActiveOnly:   q.Get("active") == "true",
        Limit:        parseIntDefault(q.Get("limit"), 100),
        Offset:       parseIntDefault(q.Get("offset"), 0),
        Search:       q.Get("search"),
    }
    
    resp, err := h.client.ListFindings(r.Context(), req)
    if err != nil {
        httpError(w, err)
        return
    }
    
    // Convert to DefectDojo-compatible response format
    result := &DDPaginatedResponse{
        Count:   int(resp.Total),
        Next:    paginationURL(r, req.Offset+req.Limit, int(resp.Total)),
        Previous: paginationURL(r, req.Offset-req.Limit, int(resp.Total)),
        Results: toFindingList(resp.Findings),
    }
    
    jsonResponse(w, http.StatusOK, result)
}

// DDPaginatedResponse matches DefectDojo's standard pagination format.
type DDPaginatedResponse struct {
    Count    int         `json:"count"`
    Next     *string     `json:"next"`
    Previous *string     `json:"previous"`
    Results  interface{} `json:"results"`
}
```

## Response Format Compatibility

```json
// GET /api/v2/findings/?product=uuid&active=true
{
    "count": 42,
    "next": "http://localhost:8080/api/v2/findings/?limit=100&offset=100",
    "previous": null,
    "results": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "title": "SQL Injection in login endpoint",
            "description": "...",
            "severity": "Critical",
            "numerical_severity": "S0",
            "cve": "CVE-2021-44228",
            "cwe": 89,
            "cvssv3": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
            "cvssv3_score": 9.8,
            "active": true,
            "verified": false,
            "false_positive": false,
            "duplicate": false,
            "out_of_scope": false,
            "mitigated": null,
            "risk_accepted": false,
            "date": "2024-01-15",
            "sla_expiration_date": "2024-01-22",
            "test": "test-uuid",
            "test_object": {...},
            "found_by": [...],
            "tags": ["api", "injection"],
            "component_name": "com.example.webapp",
            "component_version": "1.0.0",
            "file_path": "/src/auth/login.java",
            "line": 42,
            "hash_code": "abc123...",
            "created": "2024-01-15T10:30:00Z",
            "updated": "2024-01-15T10:30:00Z"
        }
    ]
}
```

## Auth Middleware

```go
// apps/DefectDojo/internal/gateway/middleware.go

package gateway

func AuthMiddleware(authClient authpb.AuthServiceClient) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)
            if token == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            var claims *UserClaims
            var err error
            
            if strings.HasPrefix(token, "ovs_") || strings.HasPrefix(token, "dd_") {
                // API Key
                resp, err := authClient.ValidateAPIKey(r.Context(), &authpb.ValidateAPIKeyRequest{
                    ApiKey: token,
                })
                if err != nil || !resp.Valid {
                    http.Error(w, "Invalid API Key", http.StatusUnauthorized)
                    return
                }
                claims = &UserClaims{UserID: resp.UserId, Permissions: resp.Permissions}
            } else {
                // JWT
                resp, err := authClient.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{
                    Token: token,
                })
                if err != nil || !resp.Valid {
                    http.Error(w, "Invalid Token", http.StatusUnauthorized)
                    return
                }
                claims = &UserClaims{UserID: resp.UserId, Role: resp.Role, Permissions: resp.Permissions}
            }
            
            ctx := context.WithValue(r.Context(), userClaimsKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```
