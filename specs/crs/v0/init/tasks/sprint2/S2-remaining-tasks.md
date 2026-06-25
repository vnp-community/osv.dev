# Sprint 2 — Remaining Tasks (Compact Reference)

> Các task này được gộp lại vì pattern tương tự nhau.
> Mỗi task cần đọc file spec tương ứng để biết chi tiết.

---

## S2-ID-02 — Password Reset Flow (identity-service)

**Spec**: `specs/develop/01_identity-service-upgrade.md` § "P1 — Thiếu: Password Reset Flow"

### Files to Create
```
services/identity-service/internal/usecase/forgot_password/forgot_password.go
services/identity-service/internal/usecase/reset_password/reset_password.go
services/identity-service/internal/infra/email/smtp_sender.go
services/identity-service/migrations/002_password_reset_tokens.sql
```

### Migration
```sql
CREATE TABLE auth.password_reset_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON auth.password_reset_tokens(token_hash);
```

### Use Case Flow (forgot_password)
```
1. Validate email exists in DB
2. Generate random token (crypto/rand, 32 bytes)
3. Hash token with SHA256 before storing
4. Store in password_reset_tokens table (expires in 1 hour)
5. Send email with reset link: https://app.example.com/reset?token=RAW_TOKEN
6. Return generic success (don't reveal if email exists)
```

### Use Case Flow (reset_password)
```
1. Hash incoming token
2. Find matching record (not expired, not used)
3. Validate new password meets requirements (min 8 chars, etc.)
4. Hash new password with Argon2id (reuse existing hasher)
5. Update user password
6. Mark token as used (set used_at = NOW())
7. Revoke all existing sessions for user (security)
8. Return success
```

### SMTP Sender
```go
// services/identity-service/internal/infra/email/smtp_sender.go
type SMTPSender struct {
    host     string
    port     int
    username string
    password string
    from     string
}

func (s *SMTPSender) SendPasswordReset(to, resetToken string) error
func (s *SMTPSender) SendWelcome(to, name string) error
```

### Routes to Add (router.go)
```
POST /api/v1/auth/forgot-password   ← payload: {"email":"..."}
POST /api/v1/auth/reset-password    ← payload: {"token":"...","new_password":"..."}
```

### Verification
```bash
# Test flow:
# 1. POST /api/v1/auth/forgot-password
# Expected: 200 OK (regardless of whether email exists)
# 2. Check email received or check DB for token
# 3. POST /api/v1/auth/reset-password with token
# Expected: 204 No Content
# 4. Login with new password
```

---

## S2-ID-03 — Email Verification Flow (identity-service)

**Spec**: `specs/develop/01_identity-service-upgrade.md` § "P1 — Thiếu: Email Verification"

### Files to Create
```
services/identity-service/internal/usecase/verify_email/send_verification.go
services/identity-service/internal/usecase/verify_email/confirm_email.go
services/identity-service/migrations/003_email_verification_tokens.sql
```

### Migration
```sql
CREATE TABLE auth.email_verification_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

### send_verification.go
```
1. Check user exists and email NOT already verified
2. Delete any existing unused tokens for this user
3. Generate new token
4. Send verification email
5. Return success
```

### confirm_email.go
```
1. Hash token
2. Find record (not expired, not used)
3. Set users.is_verified = TRUE
4. Mark token as used
5. Publish user.email_verified NATS event (optional)
```

### Routes to Add
```
POST /api/v1/auth/verify-email/send      ← Re-send verification
GET  /api/v1/auth/verify-email/confirm   ← Confirm with ?token=TOKEN
```

### Verification
```bash
# After registration, check DB: is_verified = false
# POST /api/v1/auth/verify-email/send
# GET /api/v1/auth/verify-email/confirm?token=TOKEN
# Check DB: is_verified = true
```

---

## S2-ID-04 — Admin Use Cases + Handler (identity-service)

**Spec**: `specs/develop/01_identity-service-upgrade.md` § "P1 — Thiếu: Admin Use Cases"

### Files to Create
```
services/identity-service/internal/usecase/admin/list_users.go
services/identity-service/internal/usecase/admin/get_user.go
services/identity-service/internal/usecase/admin/update_user.go
services/identity-service/internal/usecase/admin/ban_user.go
services/identity-service/internal/adapter/handler/http/admin_handler.go
```

### Repository Extensions (add to existing user_repo.go interface):
```go
ListAll(ctx, filter UserFilter) ([]*entity.User, int64, error)
CountAll(ctx, filter UserFilter) (int64, error)
BanUser(ctx, userID uuid.UUID) error
UnbanUser(ctx, userID uuid.UUID) error
UpdateRole(ctx, userID uuid.UUID, role string) error
```

### AdminHandler Routes (add to router.go):
```
GET  /api/v1/admin/users            ← roles: admin
GET  /api/v1/admin/users/{id}       ← roles: admin
PUT  /api/v1/admin/users/{id}/role  ← body: {"role":"..."}
POST /api/v1/admin/users/{id}/ban
POST /api/v1/admin/users/{id}/unban
```

**Middleware required**: Check `role == "admin"` from JWT claims before all admin routes.

### Verification
```bash
# Login as admin user
# GET /api/v1/admin/users → list all users
# POST /api/v1/admin/users/{id}/ban → ban user
# Login as banned user → expected: 401 or 403
```

---

## S2-ID-05 — NATS Event Publisher (identity-service)

**Spec**: `specs/develop/01_identity-service-upgrade.md` § "P0 — Thiếu: NATS Event Publisher"

### Files to Create
```
services/identity-service/internal/infra/messaging/nats/event_publisher.go
```

### Events
```go
const (
    SubjectUserRegistered    = "user.registered"    // → notification-service
    SubjectUserEmailVerified = "user.email_verified" // → update downstream
    SubjectUserPasswordChanged = "user.password_changed" // → security alert
    SubjectUserMFAEnabled    = "user.mfa_enabled"   // → security confirmation
)

type UserRegisteredEvent struct {
    UserID    uuid.UUID `json:"user_id"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Wire in register UseCase
```go
// usecase/register/register.go — after user created:
if uc.publisher != nil {
    go uc.publisher.PublishUserRegistered(ctx, UserRegisteredEvent{
        UserID: user.ID, Email: user.Email, CreatedAt: time.Now(),
    })
}
```

---

## S2-DATA-01 — Git Source Sync (data-service)

**Spec**: `specs/develop/02_data-service-upgrade.md` § "P1 — Thêm: Git Source Sync"

### Files to Create
```
services/data-service/internal/sync/git/git_source.go
services/data-service/internal/sync/git/config.go
services/data-service/internal/sync/git/walker.go
services/data-service/config/git_sources.yaml
```

### go.mod
```bash
cd services/data-service && go get github.com/go-git/go-git/v5
```

### git_source.go (core pattern)
```go
type GitSource struct {
    Name        string
    URL         string
    Branch      string
    PathPattern string    // glob, e.g. "advisories/**/*.json"
    LastSynced  string    // last synced commit hash (stored in DB)
}

func (s *GitSource) FetchChanges(ctx context.Context) ([][]byte, string, error) {
    // 1. Clone or pull repo (go-git)
    // 2. Walk commits from LastSynced to HEAD
    // 3. Collect added/modified files matching PathPattern
    // 4. Return file contents + new HEAD hash
}
```

### walker.go
```go
// Walk git log between fromHash and HEAD
// For each commit, collect changed files matching PathPattern
// Return diff (added/modified files)
```

### Verification
```bash
# Test clone:
cd services/data-service
go test ./internal/sync/git/...
# Expected: clone works, files returned
```

---

## S2-DATA-02 — GCS Bucket Sync (data-service)

**Spec**: `specs/develop/02_data-service-upgrade.md` § "P1 — Thêm: GCS Bucket Source"

### Files to Create
```
services/data-service/internal/sync/gcs/gcs_source.go
services/data-service/internal/sync/gcs/config.go
services/data-service/config/gcs_sources.yaml
```

### gcs_source.go (core pattern)
```go
type GCSSource struct {
    Name        string
    Bucket      string
    Prefix      string    // e.g., "Go/"
    LastSynced  string    // last synced object generation
}

func (s *GCSSource) FetchChanges(ctx context.Context) ([][]byte, error) {
    // 1. List objects in bucket/prefix with generation > LastSynced
    // 2. Download each object
    // 3. Return file contents
}
```

---

## S2-DATA-04 — OSV v1 API Endpoints (data-service)

**Spec**: `specs/develop/02_data-service-upgrade.md` § "P1 — Thêm: OSV v1 API Endpoints"

**Note**: data-service serves the raw OSV records; search-service handles queries.
data-service exposes GET endpoint for individual CVE lookup in OSV format.

### Files to Create
```
services/data-service/internal/delivery/http/osv_v1_handler.go
```

### Routes to Add
```
GET /v1/vulns/{id}    ← Serve OSV JSON for a specific CVE
                      ← Delegate to existing getbyid/query UC
```

---

## S2-SEARCH-02 — DetermineVersion Endpoint (search-service)

**Spec**: `specs/develop/03_search-service-upgrade.md` § "P1 — Thêm: DetermineVersion"

### Files to Create
```
services/search-service/internal/usecase/determine_version/usecase.go
services/search-service/internal/usecase/determine_version/dto.go
services/search-service/internal/adapter/grpcclient/data_client.go
```

### dto.go
```go
type DetermineVersionRequest struct {
    Name       string      `json:"name"`
    HashValues []HashValue `json:"hash_values"`
}

type HashValue struct {
    Sha256  string `json:"sha256"`
    GitBlob string `json:"git_blob"`
}
```

### Route to Add
```
POST /v1/determine-version
```

---

## S2-SEARCH-03 — NATS Update/Withdrawn Consumer (search-service)

**Spec**: `specs/develop/03_search-service-upgrade.md` § "P0 — Thêm: Subscribe Additional NATS Events"

### Files to Create
```
services/search-service/internal/infra/messaging/nats/cve_update_consumer.go
```

### Subscriptions
```go
// Subscribe: data.cve.updated → re-index updated CVE in OpenSearch
// Subscribe: data.cve.withdrawn → remove from OpenSearch index

func (c *CVEUpdateConsumer) Start(ctx context.Context) error {
    c.js.Subscribe("data.cve.updated", c.handleUpdated, nats.Durable("search-update"))
    c.js.Subscribe("data.cve.withdrawn", c.handleWithdrawn, nats.Durable("search-withdraw"))
    <-ctx.Done()
    return ctx.Err()
}
```

---

## S2-SCAN-01 — Agent Management UCs (scan-service)

**Spec**: `specs/develop/04_scan-service-upgrade.md` § "P1 — Thêm: Agent Management"

### Files to Create
```
services/scan-service/internal/usecase/register_agent/usecase.go
services/scan-service/internal/usecase/agent_heartbeat/usecase.go
services/scan-service/internal/usecase/get_agent_status/usecase.go
services/scan-service/internal/infra/messaging/nats/agent_consumer.go
services/scan-service/internal/scheduler/agent_offline_checker.go
services/scan-service/migrations/004_agent_heartbeat.up.sql
```

### Migration (ALTER TABLE only — never DROP)
```sql
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_heartbeat TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_online BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS capabilities TEXT[];
ALTER TABLE agents ADD COLUMN IF NOT EXISTS running_jobs INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_agents_heartbeat ON agents(last_heartbeat) WHERE is_online = TRUE;
```

### Agent NATS Consumer subscriptions
```
scan.agent.heartbeat  → agent_heartbeat UC
scan.agent.registered → register_agent UC
scan.agent.result     → process_scan_result UC
```

---

## S2-SCAN-02 — Scan Result Processing (scan-service)

**Spec**: `specs/develop/04_scan-service-upgrade.md` § "P1 — Thêm: Scan Result Processing"

### Files to Create
```
services/scan-service/internal/usecase/process_scan_result/usecase.go
services/scan-service/internal/usecase/process_scan_result/dto.go
services/scan-service/internal/adapters/grpcclient/finding_client.go
```

### Flow
```
1. Receive raw agent output (via NATS scan.agent.result)
2. Detect format (Nmap XML? ZAP JSON? CycloneDX SBOM?)
3. Parse using existing parsers/ package
4. Call finding-service.ImportScanResult (gRPC)
5. Update scan.status = COMPLETED
6. Publish scan.job.completed event
```

---

## S2-SCAN-03 — SBOM Ingest Endpoint (scan-service)

**Spec**: `specs/develop/04_scan-service-upgrade.md` § "P1 — Thêm: SBOM Ingest HTTP Endpoint"

### Files to Create
```
services/scan-service/internal/delivery/http/sbom_handler.go
```

### Routes to Add
```
POST /api/v1/scan/sbom/upload          ← multipart/form-data: file + format + product_id
GET  /api/v1/scan/sbom/{scan_id}/status
```

### Handler Flow
```
1. Parse multipart form
2. Detect format from Content-Type or form field
3. Call sbom.Parse() (existing package)
4. Extract components → create scan job
5. Return {"scan_id": "..."}
```

---

## S2-FIND-02 — Risk Acceptance UC (finding-service)

**Spec**: `specs/develop/05_finding-service-upgrade.md` § "P1 — Thêm: Risk Acceptance UC"

### Files to Create
```
services/finding-service/internal/usecase/risk_acceptance/accept.go
services/finding-service/internal/usecase/risk_acceptance/review.go
services/finding-service/migrations/007_risk_acceptance.up.sql
```

### Migration
```sql
ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS risk_acceptance_expiry TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS risk_justification TEXT;

CREATE TABLE IF NOT EXISTS risk_acceptances (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id    UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    accepted_by   UUID NOT NULL,
    justification TEXT,
    expires_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### accept.go flow
```
1. Validate finding exists and is active
2. Check user has permission (product owner or admin)
3. Set finding.risk_accepted = TRUE
4. Set finding.risk_acceptance_expiry (if provided)
5. Insert risk_acceptance audit record
6. Publish finding.risk_accepted NATS event
```

---

## S2-FIND-03 — NATS Subscribers (finding-service)

**Spec**: `specs/develop/05_finding-service-upgrade.md` § "P1 — Thêm: NATS Subscribers"

### Files to Create
```
services/finding-service/internal/infra/messaging/nats/scan_result_subscriber.go
services/finding-service/internal/infra/messaging/nats/cve_update_subscriber.go
```

### scan_result_subscriber.go
```
Subscribe: scan.job.completed
→ Trigger: orchestrator/import UC (existing)
→ Parse scan result → create findings
```

### cve_update_subscriber.go
```
Subscribe: data.cve.updated
→ When CVE severity changes → update active findings
→ Publish finding.status_changed if severity changed
```

---

## S2-GW-02 — Aggregated Health Check (gateway-service)

**Spec**: `specs/develop/08_gateway-service-upgrade.md` § "P1 — Thêm: Aggregated Health"

### Files to Create
```
services/gateway-service/internal/health/aggregated_health.go
```

### aggregated_health.go
```go
// GET /health → parallel HTTP health checks for all 7 downstream services
// Timeout per service: 2 seconds
// Overall timeout: 5 seconds
// Returns 200 if all healthy, 207 (Multi-Status) if some degraded, 503 if critical down

type ServiceHealth struct {
    Name    string `json:"name"`
    Status  string `json:"status"`  // healthy | degraded | unhealthy
    Latency string `json:"latency"`
    Error   string `json:"error,omitempty"`
}
```

### Routes to Add
```
GET /health → aggregate check (NEW)
// Keep existing:
GET /health/live
GET /health/ready
```

---

## S2-GW-03 — Circuit Breaker per Upstream (gateway-service)

**Spec**: `specs/develop/08_gateway-service-upgrade.md` § "P1 — Thêm: Circuit Breaker"

### Files to Create
```
services/gateway-service/internal/proxy/circuit_breaker.go
```

### Note
`shared/pkg/resilience/` đã có circuit breaker implementation. Chỉ cần:
1. Wrap proxy calls với resilience.CircuitBreaker
2. Config per upstream (threshold, timeout, halfOpen wait)

```go
// proxy/circuit_breaker.go
// Không xóa http_proxy.go cũ
// Thêm wrapper middleware:

type CircuitBreakerMiddleware struct {
    breakers map[string]*resilience.CircuitBreaker  // per-upstream
}

func (m *CircuitBreakerMiddleware) Wrap(upstream string, handler http.Handler) http.Handler {
    cb := m.getOrCreate(upstream)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !cb.Allow() {
            http.Error(w, `{"error":"service temporarily unavailable"}`, http.StatusServiceUnavailable)
            return
        }
        handler.ServeHTTP(w, r)
        cb.Success()  // or cb.Failure() based on response code
    })
}
```

---

## S2-AI-01 — Complete HTTP Handlers (ai-service)

**Spec**: `specs/develop/06_ai-service-upgrade.md` § "P1 — Thêm: Complete HTTP Handlers"

### Files to Create
```
services/ai-service/internal/delivery/http/enrich_handler.go
services/ai-service/internal/delivery/http/epss_handler.go
services/ai-service/internal/delivery/http/triage_handler.go
services/ai-service/internal/delivery/http/admin_handler.go
```

### Routes to Add (existing router.go)
```
POST  /enrich/{cve_id}    → trigger enrichment
GET   /enrich/{cve_id}    → get cached result
GET   /epss/{cve_id}      → get EPSS score
POST  /epss/batch         → batch EPSS scores
POST  /triage/finding     → triage a finding
POST  /admin/batch-enrich → trigger batch enrichment
GET   /admin/stats        → enrichment statistics
```

---

## S2-AI-02 — NATS Consumers/Publishers (ai-service)

**Spec**: `specs/develop/06_ai-service-upgrade.md` § "P1 — Thêm: NATS Enhancement"

### Files to Create
```
services/ai-service/internal/infra/messaging/nats/cve_updated_consumer.go
services/ai-service/internal/infra/messaging/nats/enrichment_publisher.go
```

### cve_updated_consumer.go
```
Subscribe: data.cve.updated → re-enrich CVE nếu description thay đổi
Subscribe: finding.created  → auto-triage new findings
```

### enrichment_publisher.go
```go
SubjectEnrichmentCompleted = "ai.enrichment.completed"
SubjectEPSSUpdated         = "ai.epss.updated"
```

---

## S2-NOTIF-02 — Retry Delivery Logic (notification-service)

**Spec**: `specs/develop/07_notification-service-upgrade.md` § "P1 — Thêm: Retry Use Case + Cron"

### Files to Create
```
services/notification-service/internal/usecase/retry_delivery/usecase.go
services/notification-service/internal/scheduler/retry_worker.go
services/notification-service/migrations/006_delivery_retry.up.sql
```

### Migration
```sql
ALTER TABLE delivery_records ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;
ALTER TABLE delivery_records ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 5;
CREATE INDEX IF NOT EXISTS idx_delivery_retry ON delivery_records(next_retry_at)
    WHERE status = 'failed' AND attempts < max_attempts;
```

### Retry strategy (exponential backoff)
```
Attempt 1: immediate (already done)
Attempt 2: +5 minutes
Attempt 3: +30 minutes
Attempt 4: +2 hours
Attempt 5: +24 hours → mark as permanently failed
```

### retry_worker.go
```go
// Cron: every 5 minutes
// Find delivery_records WHERE status='failed' AND next_retry_at <= NOW() AND attempts < max_attempts
// For each: retry dispatch → update status/attempts/next_retry_at
```
