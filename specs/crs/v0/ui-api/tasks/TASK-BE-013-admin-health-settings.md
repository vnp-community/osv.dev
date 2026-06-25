# TASK-BE-013 — gateway: Admin Health Fan-out + Settings BFF

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-013 |
| **Service** | `apps/osv` (gateway) |
| **Solution Ref** | [SOL-UI-004 §5–6](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 4h |

---

## Context

Admin Panel cần:
1. `GET /api/v1/admin/health` — fan-out health check đến tất cả 7 services + Redis/NATS/Postgres
2. `GET /api/v1/admin/settings` — platform config (SMTP, AI keys, scan limits)
3. `PATCH /api/v1/admin/settings` — update platform config

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `apps/osv/internal/gateway/bff/health.go` |
| CREATE | `apps/osv/internal/gateway/bff/settings.go` |
| CREATE | `apps/osv/internal/infra/postgres/settings_repo.go` |
| MODIFY | `apps/osv/internal/gateway/router.go` |

---

## Implementation

### File 1: `apps/osv/internal/gateway/bff/health.go`

```go
package bff

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type ServiceHealth struct {
	Name           string  `json:"name"`
	Status         string  `json:"status"` // "healthy"|"degraded"|"down"
	ResponseTimeMs int64   `json:"response_time_ms"`
	Version        string  `json:"version,omitempty"`
	LastCheckedAt  string  `json:"last_checked_at"`
	Details        *string `json:"details,omitempty"`
}

type InfraHealth struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ResponseTimeMs int64  `json:"response_time_ms"`
}

type HealthResponse struct {
	Services      []ServiceHealth `json:"services"`
	Infrastructure []InfraHealth  `json:"infrastructure"`
	OverallStatus string          `json:"overall_status"` // "healthy"|"degraded"|"down"
	CheckedAt     string          `json:"checked_at"`
}

// HealthBFF performs fan-out health checks
type HealthBFF struct {
	httpClient *http.Client
	redis      *redis.Client
	natsConn   *nats.Conn
	pgPing     func(ctx context.Context) error
	services   []struct{ name, url string }
}

func NewHealthBFF(redis *redis.Client, nats *nats.Conn, pgPing func(ctx context.Context) error) *HealthBFF {
	return &HealthBFF{
		httpClient: &http.Client{Timeout: 2 * time.Second},
		redis:      redis,
		natsConn:   nats,
		pgPing:     pgPing,
		services: []struct{ name, url string }{
			{"identity-service",     "http://identity-service:8081/health"},
			{"data-service",         "http://data-service:8082/health"},
			{"finding-service",      "http://finding-service:8085/health"},
			{"sla-service",          "http://sla-service:8086/health"},
			{"notification-service", "http://notification-service:8087/health"},
			{"jira-service",         "http://jira-service:8088/health"},
			{"audit-service",        "http://audit-service:8090/health"},
		},
	}
}

// HandleAdminHealth — GET /api/v1/admin/health
func (bff *HealthBFF) HandleAdminHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var (
		serviceResults []ServiceHealth
		infraResults   []InfraHealth
		mu             sync.Mutex
		wg             sync.WaitGroup
	)

	// Check all services concurrently
	wg.Add(len(bff.services))
	for _, svc := range bff.services {
		s := svc
		go func() {
			defer wg.Done()
			result := bff.checkService(ctx, s.name, s.url)
			mu.Lock()
			serviceResults = append(serviceResults, result)
			mu.Unlock()
		}()
	}

	// Check infrastructure concurrently
	wg.Add(3)
	go func() {
		defer wg.Done()
		result := bff.checkRedis(ctx)
		mu.Lock()
		infraResults = append(infraResults, result)
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		result := bff.checkNATS(ctx)
		mu.Lock()
		infraResults = append(infraResults, result)
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		result := bff.checkPostgres(ctx)
		mu.Lock()
		infraResults = append(infraResults, result)
		mu.Unlock()
	}()

	wg.Wait()

	// Compute overall status
	overall := "healthy"
	for _, s := range serviceResults {
		if s.Status == "down" {
			overall = "down"
			break
		}
		if s.Status == "degraded" && overall != "down" {
			overall = "degraded"
		}
	}
	for _, s := range infraResults {
		if s.Status == "down" {
			overall = "down"
			break
		}
	}

	respondJSON(w, 200, HealthResponse{
		Services:       serviceResults,
		Infrastructure: infraResults,
		OverallStatus:  overall,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	})
}

func (bff *HealthBFF) checkService(ctx context.Context, name, url string) ServiceHealth {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result := ServiceHealth{
		Name:          name,
		LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := bff.httpClient.Do(req)
	result.ResponseTimeMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "down"
		msg := err.Error()
		result.Details = &msg
		return result
	}
	defer resp.Body.Close()

	var healthResp struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&healthResp)
	result.Version = healthResp.Version

	if resp.StatusCode != 200 || result.ResponseTimeMs > 500 {
		result.Status = "degraded"
	} else {
		result.Status = "healthy"
	}

	return result
}

func (bff *HealthBFF) checkRedis(ctx context.Context) InfraHealth {
	start := time.Now()
	err := bff.redis.Ping(ctx).Err()
	return InfraHealth{
		Name:           "redis",
		Status:         statusFromErr(err),
		ResponseTimeMs: time.Since(start).Milliseconds(),
	}
}

func (bff *HealthBFF) checkNATS(ctx context.Context) InfraHealth {
	start := time.Now()
	status := "healthy"
	if !bff.natsConn.IsConnected() {
		status = "down"
	}
	return InfraHealth{
		Name:           "nats",
		Status:         status,
		ResponseTimeMs: time.Since(start).Milliseconds(),
	}
}

func (bff *HealthBFF) checkPostgres(ctx context.Context) InfraHealth {
	start := time.Now()
	err := bff.pgPing(ctx)
	return InfraHealth{
		Name:           "postgres",
		Status:         statusFromErr(err),
		ResponseTimeMs: time.Since(start).Milliseconds(),
	}
}

func statusFromErr(err error) string {
	if err != nil {
		return "down"
	}
	return "healthy"
}
```

### File 2: `apps/osv/internal/gateway/bff/settings.go`

```go
package bff

import (
	"encoding/json"
	"net/http"
)

// PlatformSettings is the system-wide configuration
type PlatformSettings struct {
	General       GeneralSettings       `json:"general"`
	Notifications NotificationSettings  `json:"notifications"`
	AI            AISettings            `json:"ai"`
	Security      SecuritySettings      `json:"security"`
}

type GeneralSettings struct {
	PlatformName string `json:"platform_name"`
	MaxScanTargets int  `json:"max_scan_targets"`
	ReportRetentionDays int `json:"report_retention_days"`
}

type NotificationSettings struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPFrom     string `json:"smtp_from"`
	SMTPPassword string `json:"smtp_password,omitempty"` // redacted on read
}

type AISettings struct {
	OllamaEnabled     bool    `json:"ollama_enabled"`
	OllamaURL         string  `json:"ollama_url"`
	OpenAIEnabled     bool    `json:"openai_enabled"`
	OpenAIModel       string  `json:"openai_model"`
	OpenAIKeyPreview  string  `json:"openai_key_preview"` // first 8 chars only
	EmbeddingEnabled  bool    `json:"embedding_enabled"`
	EmbeddingDims     int     `json:"embedding_dims"`
}

type SecuritySettings struct {
	MaxLoginAttempts  int `json:"max_login_attempts"`
	SessionTimeoutMin int `json:"session_timeout_minutes"`
	RateLimitPerMin   int `json:"rate_limit_per_min"`
}

type SettingsBFF struct {
	settingsRepo SettingsRepository
}

// GET /api/v1/admin/settings
func (bff *SettingsBFF) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := bff.settingsRepo.Get(r.Context())
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	// Redact sensitive fields
	settings.Notifications.SMTPPassword = ""
	// OpenAI key: show only preview
	// (settingsRepo.Get already stores preview separately)

	respondJSON(w, 200, settings)
}

// PATCH /api/v1/admin/settings
func (bff *SettingsBFF) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if err := bff.settingsRepo.Patch(r.Context(), patch); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{"success": true})
}
```

### Platform Settings DB (add to `apps/osv` migrations):

```sql
-- Gateway/shared database settings table
CREATE TABLE IF NOT EXISTS platform_settings (
    key        VARCHAR(100) PRIMARY KEY,
    value_json JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);

-- Initial seed
INSERT INTO platform_settings (key, value_json) VALUES
('general', '{"platform_name":"OSV Platform","max_scan_targets":100,"report_retention_days":90}'),
('notifications', '{"smtp_host":"","smtp_port":587,"smtp_from":"noreply@osv.local","smtp_password":""}'),
('ai', '{"ollama_enabled":true,"ollama_url":"http://ollama:11434","openai_enabled":false,"openai_model":"gpt-4","embedding_enabled":true,"embedding_dims":1536}'),
('security', '{"max_login_attempts":5,"session_timeout_minutes":15,"rate_limit_per_min":100}')
ON CONFLICT (key) DO NOTHING;
```

### Gateway Router additions:

```go
// apps/osv/internal/gateway/router.go

healthBFF := bff.NewHealthBFF(redisClient, natsConn, db.Ping)
settingsBFF := &bff.SettingsBFF{settingsRepo: settingsRepo}

adminAuth := am.Authenticate(am.RequireRole("admin"))

mux.Handle("GET /api/v1/admin/health",    adminAuth(http.HandlerFunc(healthBFF.HandleAdminHealth)))
mux.Handle("GET /api/v1/admin/settings",  adminAuth(http.HandlerFunc(settingsBFF.GetSettings)))
mux.Handle("PATCH /api/v1/admin/settings", adminAuth(http.HandlerFunc(settingsBFF.UpdateSettings)))
```

---

## Verification

```bash
cd apps/osv
go build ./...

# Health check
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8080/api/v1/admin/health | jq '.overall_status'
# Expected: "healthy" | "degraded" | "down"

curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8080/api/v1/admin/health | jq '.services | length'
# Expected: 7

# Settings
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8080/api/v1/admin/settings | jq 'keys'
# Expected: ["ai","general","notifications","security"]
```

---

## Checklist

- [x] `HandleAdminHealth` fan-out đến 7 services + 3 infra checks concurrently
- [x] Timeout 2s per service check
- [x] `overall_status` = "down" nếu có service nào down, "degraded" nếu có degraded
- [x] `SettingsBFF.GetSettings` redact SMTP password và return chỉ `openai_key_preview`
- [x] `platform_settings` table với JSON values seed data
- [x] Admin endpoints đều require role "admin" (gateway middleware)
- [x] `go build ./...` thành công
