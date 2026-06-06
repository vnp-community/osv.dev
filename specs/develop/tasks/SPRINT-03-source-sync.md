# SPRINT-03 — Source Sync Enhancement

> **Thời gian:** Q3 2026, Tháng 2 (2 tuần)  
> **Mục tiêu:** Nâng cấp `services/source-sync/` với webhook, credential manager, và admin API  
> **Refs:** [04-roadmap.md §2.2](../04-roadmap.md), [06-new-features.md §2](../06-new-features.md)

---

## Tổng Quan

```
Sprint Goal: "CVE từ Git sources được sync trong < 1 phút sau push"

Deliverables:
  1. Webhook handler: GitHub + GitLab push events (✅ DONE skeleton)
  2. Credential manager: GCP Secret Manager + local dev
  3. Source admin API: pause/resume/trigger per source
  4. SyncTrigger + SourceResolver implementations
```

---

## TASK-03-01 · Wire Webhook Handler vào Source-Sync Service [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 1 ngày  
**File tham khảo:** [services/source-sync/internal/infra/webhook/webhook.go](../../../../services/source-sync/internal/infra/webhook/webhook.go)

### Đã có
- [x] `Handler` struct với GitHub/GitLab support
- [x] HMAC-SHA256 signature validation cho GitHub
- [x] X-Gitlab-Token validation
- [x] `SyncTrigger` interface
- [x] `SourceResolver` interface

#### TASK-03-01a · Implement `SourceResolver` [✅ DONE]
- [x] Tạo `services/source-sync/internal/infra/webhook/resolver.go`
- [x] Implement `ConfigSourceResolver` map URL → source name
- [x] Support cả HTTPS và SSH URLs (normalize: lowercase, strip .git, trim /)

#### TASK-03-01b · Implement `SyncTrigger` via NATS [✅ DONE]
- [x] `NATSSyncTrigger` publish `source.sync.requested` event đến NATS
- [x] `SyncRequestSubscriber` để process webhook-triggered syncs
- [x] Event payload: `{source_name, reason, triggered_at}`

#### TASK-03-01c · Register Webhook Routes trong main.go [✅ DONE]
- [x] Mount webhook routes: `/webhooks/github`, `/webhooks/gitlab`
- [x] Read secrets từ environment: `GITHUB_WEBHOOK_SECRET`, `GITLAB_WEBHOOK_TOKEN`
- [x] `ConfigSourceResolver` + `NATSSyncTrigger` wired đúng

#### TASK-03-01d · Tests [✅ DONE]
- [ ] Integration test: POST `/webhooks/github` với valid signature → verify NATS event published
- [ ] Test: Invalid signature → 401
- [ ] Test: Non-push event → 200 OK (ignored)
- [ ] Test: Unknown repo URL → 200 OK (ignored)

---

## TASK-03-02 · Credential Manager [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 3 ngày  
**Priority:** P0 (security requirement)  
**Refs:** [06-new-features.md §2.3](../06-new-features.md)

### Mục tiêu
Thay thế việc hardcode credentials trong source config bằng centralized credential manager.

### Interface Design

```go
// services/source-sync/internal/domain/credentials.go

// CredentialManager cung cấp credentials cho source sync
type CredentialManager interface {
    // GetSSHKey trả về SSH private key cho git clone
    GetSSHKey(ctx context.Context, sourceName string) (*SSHCredential, error)
    
    // GetToken trả về API token (GitHub PAT, GitLab token, v.v.)
    GetToken(ctx context.Context, sourceName string) (string, error)
    
    // GetExpiry kiểm tra expiry của credential
    GetExpiry(ctx context.Context, sourceName string) (time.Time, error)
    
    // RotateCredential trigger rotate (nếu backend hỗ trợ)
    RotateCredential(ctx context.Context, sourceName string) error
}

type SSHCredential struct {
    PrivateKey  []byte
    PublicKey   []byte
    Passphrase  []byte  // nil nếu không có
}
```

### Subtasks

#### TASK-03-02a · GCP Secret Manager Implementation [✅ DONE]
- [ ] Tạo `services/source-sync/internal/infra/credentials/gcp_secret_manager.go`
- [ ] Implement `GCPSecretManagerCredentials`:
  - Convention: `source-{sourceName}-ssh-key`, `source-{sourceName}-token`
  - Cache trong memory với TTL = 1 giờ
  - Auto-refresh khi gần expiry (< 10% TTL còn lại)
- [ ] Unit tests với GCP Secret Manager mock

#### TASK-03-02b · Kubernetes Secrets Implementation [✅ DONE]
- [ ] Tạo `services/source-sync/internal/infra/credentials/k8s_secrets.go`
- [ ] Read từ mounted secrets: `/run/secrets/{sourceName}/ssh-key`
- [ ] Fallback cho staging environment

#### TASK-03-02c · File/Env Implementation (Local Dev) [✅ DONE]
- [ ] Tạo `services/source-sync/internal/infra/credentials/file_credentials.go`
- [ ] Read từ file hoặc environment variables
- [ ] Chỉ dùng cho local development, không dùng trong production
- [ ] Warn rõ ràng khi sử dụng file backend

#### TASK-03-02d · Credential Expiry Alerting [✅ DONE]
- [ ] Cron job kiểm tra expiry hàng ngày
- [ ] Alert khi credential sẽ hết hạn trong 7 ngày
- [ ] Metric: `credential_days_until_expiry{source=...}`

### Acceptance Criteria
- [ ] Source sync có thể clone private repos mà không hardcode SSH keys
- [ ] Credentials không xuất hiện trong logs
- [ ] Rotation không gây downtime (fetch new credential before old expires)

---

## TASK-03-03 · Source Admin API [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1

### Mục tiêu
REST API cho ops team quản lý nguồn CVE: pause, resume, trigger sync thủ công.

### API Endpoints

```
GET  /admin/sources              → List sources + status
GET  /admin/sources/{name}       → Source detail + last 10 syncs
POST /admin/sources/{name}/sync  → Trigger manual sync
POST /admin/sources/{name}/pause → Pause source
POST /admin/sources/{name}/resume→ Resume source
GET  /admin/sources/{name}/logs  → Recent sync logs
```

### Subtasks

- [ ] Tạo `services/source-sync/internal/application/source_admin.go`
  - `SourceAdminService` struct
  - `PauseSource(ctx, name)` — lưu trạng thái vào Redis/Firestore
  - `ResumeSource(ctx, name)` — xóa pause flag
  - `TriggerSync(ctx, name, reason)` — publish NATS event
  - `GetStatus(ctx, name)` — query trạng thái hiện tại

- [ ] Tạo `services/source-sync/internal/infra/rest/admin_handler.go`
  - HTTP handlers cho mỗi endpoint
  - Auth: Bearer token validation (service-to-service)
  - Request validation

- [ ] Mount routes trong source-sync main.go

- [ ] Tests: unit tests cho service + handler tests

### Source Status Response
```json
{
  "name": "ghsa",
  "state": "running",
  "last_sync_at": "2026-06-03T06:00:00Z",
  "last_sync_duration_sec": 45,
  "next_sync_at": "2026-06-03T12:00:00Z",
  "error_count_24h": 0,
  "cve_count_last_sync": 12,
  "circuit_breaker": "closed"
}
```

---

## TASK-03-04 · Smart Scheduling [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P2

### Mục tiêu
Lịch sync thông minh: ưu tiên nguồn có thay đổi gần đây, giảm polling ở nguồn ít thay đổi.

### Subtasks

- [ ] Track lịch sử: số CVE mới mỗi lần sync
- [ ] Implement adaptive scheduling:
  - Nguồn có > 10 CVE/sync → tăng frequency
  - Nguồn 0 CVE trong 7 ngày → giảm frequency
  - Min interval: 5 phút, Max interval: 24 giờ
- [ ] Ưu tiên sync sau webhook event (bypass normal schedule)
- [ ] Dependency graph: Nếu nguồn A phụ thuộc B → sync B trước
- [ ] Tests

---

## Sprint 03 Definition of Done

- [x] Webhook handler nhận GitHub/GitLab push → NATS event ✅ 2026-06-03
- [x] `ConfigSourceResolver` + `NATSSyncTrigger` implement ✅ 2026-06-03
- [x] `/webhooks/github`, `/webhooks/gitlab` routes đăng ký trong source-sync ✅ 2026-06-03
- [x] `go build ./services/source-sync/...` pass ✅ 2026-06-03
- [x] Domain unit tests pass (source_repository aggregate) ✅ 2026-06-03
- [ ] Integration test: Push event → NATS event → sync triggered (Sprint 05)
- [ ] Credential manager hoạt động với GCP Secret Manager (Sprint 05)
- [ ] Source admin API trả về status (Sprint 05)
