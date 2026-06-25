# Bug Report: Mock Data & Nil Wiring Issues in Services

> **Phạm vi rà soát**: `/services/` — tất cả các file `embedded.go`, handler và usecase liên quan.
> **Ngày rà soát**: 2026-06-22  
> **Loại lỗi**: Mock dữ liệu, Nil object, Stub implementation, Noop adapter không thực thi thực sự.

---

## Tóm tắt

| # | Service | File | Loại Bug | Mức độ |
|---|---------|------|----------|--------|
| MOCK-001 | finding-service | `embedded.go:L67` | `nilReportRepo` — repo mock thay thế MinIO | 🔴 High |
| MOCK-002 | finding-service | `embedded.go:L67` | `generateUC = nil` — Report Create/Download panicable | 🔴 High |
| MOCK-003 | finding-service | `embedded.go:L59` | `NewBulkHandler(..., nil, ...)` — EventBus bị bỏ qua | 🟡 Medium |
| MOCK-004 | finding-service | `embedded.go:L74-L81` | 7 handlers `nil` — các routes bị disabled hoàn toàn | 🟡 Medium |
| MOCK-005 | finding-service | `embedded.go:L34` | NATS fallback noop — events bị mất thầm lặng | 🟡 Medium |
| MOCK-006 | scan-service | `embedded.go:L19,22,26` | 3 handlers khởi tạo với `nil` repo | 🟡 Medium |
| MOCK-007 | scan-service | `agent_handler.go` | `RegisterAgent` / `SubmitReport` — fake in-memory data | 🔴 High |
| MOCK-008 | search-service | `embedded.go:L89` | `MockEmbedder{}` dùng trong production | 🔴 High |
| MOCK-009 | search-service | `embedded.go:L96` | `cvesearch.New(..., nil, ...)` — osClient nil, không dùng OpenSearch | 🟡 Medium |
| MOCK-010 | search-service | `embedded.go:L101` | `NewRouter(..., nil, ...)` — InternalHandler nil, route bị disabled | 🟡 Medium |
| MOCK-011 | identity-service | `embedded.go:L58-59` | OAuth providers khởi tạo với empty credentials `""` | 🔴 High |
| MOCK-012 | gateway-service | `embedded.go:L125` | `NewAPIKeyValidator(nil, rdb)` — repo nil, cold path sẽ panic | 🔴 High |
| MOCK-013 | gateway-service | `embedded.go:L81,97,145` | `search-service` hardcoded `http://localhost:8083` | 🟡 Medium |
| MOCK-014 | notification-service | `embedded.go:L48` | `SetupRouter(..., nil, nil, ...)` — AlertsHandler & SSEHandler nil | 🔴 High |
| MOCK-015 | asset-service | `embedded.go:L34,46` | `NoopFindingClient` + `noopEventPublisher` — risk scoring luôn = 0 | 🟡 Medium |

---

## Chi tiết từng Bug

---

### MOCK-001 — `nilReportRepo`: Report repository không được kết nối DB/Storage

**File**: `services/finding-service/embedded.go:L67-L112`  
**Loại**: Stub struct thay thế real implementation

```go
// Dòng 67 — embedded.go
reportHandler := httpdelivery.NewReportHandler(nil, &nilReportRepo{}, nil)

// nilReportRepo — không kết nối DB, chỉ trả lỗi cứng
func (n *nilReportRepo) Save(_ context.Context, _ *report.Report) error {
    return fmt.Errorf("report storage not configured")  // ← lỗi cứng
}
func (n *nilReportRepo) FindByID(_ context.Context, _ string) (*report.Report, error) {
    return nil, fmt.Errorf("report not found")           // ← luôn không tìm thấy
}
```

**Tác động**: 
- `POST /api/v1/reports` → lưu report vào đâu? Không có DB, không có MinIO.
- `GET /api/v1/reports/{id}` → luôn trả về 404.
- `GET /api/v1/reports` → trả về list rỗng (may mắn `ListByProduct` trả `[]` không lỗi).

**Fix cần thiết**: Wire PostgreSQL `ReportRepo` + MinIO `StorageAdapter` thực sự khi MinIO được cấu hình.

---

### MOCK-002 — `generateUC = nil`: Report Create sẽ panic khi được gọi

**File**: `services/finding-service/embedded.go:L67` + `services/finding-service/internal/delivery/http/report_handler.go:L88`  
**Loại**: Nil pointer dereference

```go
// embedded.go L67: generateUC = nil (1st argument)
reportHandler := httpdelivery.NewReportHandler(nil, &nilReportRepo{}, nil)

// report_handler.go L88: dereference trực tiếp không check nil
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
    rep, err := h.generateUC.Execute(...)  // ← PANIC nếu generateUC == nil
}
```

**Tác động**: `POST /api/v1/reports` hoặc `POST /api/v2/reports/generate` sẽ gây **runtime panic** (nil pointer dereference). Server crash nếu không có `middleware.Recoverer`.  

**Fix cần thiết**: Thêm nil-check trong `Create()` hoặc khởi tạo `generateUC` thực sự.

> ⚠️ CAUTION: Bug MOCK-002 là **crash risk**. Nếu Recoverer middleware bị tắt, toàn bộ goroutine sẽ panic.

---

### MOCK-003 — `NewBulkHandler(..., nil, ...)`: EventBus bị bỏ qua

**File**: `services/finding-service/embedded.go:L59`  
**Loại**: Nil dependency silently ignored

```go
// embedded.go L59: eventBus = nil
bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, nil, logger)

// bulk_handler.go L165: có nil-check, nên không panic, nhưng events bị mất
if h.eventBus != nil {
    h.eventBus.Publish(...)   // ← chỉ publish khi có eventBus
}
```

**Tác động**: `BulkReopen` không publish event `finding.status.changed`. Các consumer downstream (notification, audit) không nhận được thông báo khi trạng thái finding thay đổi hàng loạt.

**Fix cần thiết**: Wire NATS publisher thực sự hoặc dùng `noopPublisher` có logging.

---

### MOCK-004 — 7 nil handlers trong `NewRouter()`

**File**: `services/finding-service/embedded.go:L70-L87`  
**Loại**: Unimplemented features — routes bị disable hoàn toàn

```go
router := httpdelivery.NewRouter(
    findingHandler,   // ✅ có
    bulkHandler,      // ✅ có  
    noteHandler,      // ✅ có
    nil,              // ❌ EngagementHandler — /api/v2/engagements bị tắt
    nil,              // ❌ TestHandler — /api/v2/tests bị tắt
    nil,              // ❌ MemberHandler — /api/v2/members bị tắt
    nil,              // ❌ ToolHandler — /api/v2/tool-configurations bị tắt
    reportHandler,    // ✅ có (nhưng mock — xem MOCK-001)
    nil,              // ❌ RiskAcceptanceHandler — /api/v1/risk-acceptances bị tắt
    nil,              // ❌ InternalHandler — /internal/stats bị tắt
    nil,              // ❌ SLAHandler — /internal/sla-dashboard bị tắt
    productHandler,   // ✅ có
    productSeed,      // ✅ có
    findingSeed,      // ✅ có
    findingGroup,     // ✅ có
    logger,
)
```

**Tác động**: 6 nhóm endpoint hoàn toàn không hoạt động — trả về 404. Dashboard BFF fanout tới `/internal/stats` và `/internal/sla-breaches` sẽ nhận 404.

**Fix cần thiết**: Wire các handler còn thiếu: `EngagementHandler`, `TestHandler`, `RiskAcceptanceHandler`, `InternalHandler`.

---

### MOCK-005 — NATS fallback Noop Publisher: Events bị mất thầm lặng

**File**: `services/finding-service/embedded.go:L34-L48`  
**Loại**: Silent data loss — noop không log event dropped

```go
pub := mynats.NewNoopPublisher() // safe default — nhưng không log event nào bị drop
// ...
if err == nil {
    // dùng publisher thật
} else {
    logger.Warn().Err(err).Msg("NATS unreachable, using noop publisher")
    // pub vẫn là noop — không retry, không queue, events mất
}
```

**Tác động**: Nếu NATS không available lúc khởi động, toàn bộ finding events (status changes, bulk updates) sẽ bị **mất vĩnh viễn** mà không có cơ chế retry hoặc dead-letter queue.

**Fix cần thiết**: Thêm outbox pattern hoặc cơ chế retry khi NATS khả dụng trở lại.

---

### MOCK-006 — Scan handlers với `nil` repo: Tất cả scans đều trả stub data

**File**: `services/scan-service/embedded.go:L19-L26`  
**Loại**: Nil repository — không kết nối DB

```go
agentHandler := httpdelivery.NewAgentHandler(nil, logger)    // nil repo
scanHandler := httpdelivery.NewScanAPIHandler(nil, logger)   // nil repo
statsHandler := httpdelivery.NewStatsHandler(nil, logger)    // nil repo — luôn 0
```

**Tác động**: Toàn bộ `scan-service` embedded không dùng database. Dữ liệu không được lưu trữ.

**Fix cần thiết**: Khởi tạo `ScanRepo`, `AgentRepo`, `StatsRepo` từ PostgreSQL pool.

---

### MOCK-007 — `AgentHandler.RegisterAgent()`: Dữ liệu giả, không lưu DB

**File**: `services/scan-service/internal/delivery/http/agent_handler.go:L60-L92`  
**Loại**: Fake implementation trong production handler

```go
// comment nói rõ: "Fake implementation for seed purposes"
// Fake implementation for seed purposes
apiKeyPlaintext := "ak_live_" + uuid.New().String()
agent := AgentDto{
    ID: uuid.New(),   // ← random UUID, không được lưu vào DB
    ...
}
h.writeJSON(w, 201, map[string]any{...})  // trả 201 nhưng không có gì được lưu
```

**Tác động**: 
- Agent đăng ký thành công (201) nhưng không được lưu vào DB.
- `GET /api/v1/agents` luôn trả `{"agents": [], "count": 0}`.
- API Key được generate nhưng không validate được lần sau.
- `POST /api/v1/agents/{id}/reports` không xử lý report thực sự.

> ⚠️ WARNING: Bug này làm cho toàn bộ agent management flow không hoạt động trong production.

---

### MOCK-008 — `MockEmbedder{}` dùng trong production path

**File**: `services/search-service/embedded.go:L89` + `services/search-service/internal/infra/pgvector/semantic_search.go:L57-62`  
**Loại**: Mock embedder trong production code

```go
// embedded.go L89:
semanticUC = pgvector.NewUseCase(searcher, &pgvector.MockEmbedder{})

// MockEmbedder returns zero vectors (development/testing only).
type MockEmbedder struct{}
func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
    return make([]float32, 1536), nil  // ← all-zero vector!
}
```

**Tác động**: Semantic search (`POST /api/v2/cves/search/semantic`) hoạt động nhưng trả về kết quả **vô nghĩa** — tất cả CVEs đều có similarity score bằng nhau (all-zero embedding). Người dùng không biết kết quả là sai.

> ⚠️ CAUTION: Đây là bug nghiêm trọng về correctness — người dùng tin tưởng kết quả tìm kiếm nhưng thực tế là random/garbage.

---

### MOCK-009 — `cvesearch.New(..., nil, ...)`: OpenSearch bị bỏ qua

**File**: `services/search-service/embedded.go:L96`  
**Loại**: Nil dependency — fallback không được document rõ

```go
// osClient = nil → chỉ dùng Postgres full-text, không dùng OpenSearch
searchUC := cvesearch.New(cveRepo, cacheRepo, nil, log)
```

**Tác động**: OpenSearch không được dùng cho search dù có thể đã được cấu hình. Hiệu năng và relevance tệ hơn.

---

### MOCK-010 — `NewRouter(..., nil, ...)`: InternalHandler nil — Indexing routes bị tắt

**File**: `services/search-service/embedded.go:L101`  
**Loại**: Nil handler — routes bị disable

```go
// internalH = nil (4th argument)
router := deliveryhttp.NewRouter(h, taxH, vendorH, nil, statsH, log)
// → /internal/opensearch/index và /internal/opensearch/bulk bị tắt
```

**Tác động**: Không thể trigger OpenSearch re-indexing qua internal API.

---

### MOCK-011 — OAuth providers khởi tạo với empty credentials

**File**: `services/identity-service/embedded.go:L58-59`  
**Loại**: Hardcoded empty credentials

```go
// clientID, clientSecret, redirectURL đều là ""
googleProvider := oauth.NewGoogleProvider("", "", "")
githubProvider := oauth.NewGitHubProvider("", "", "")
```

**Tác động**: 
- OAuth login với Google/GitHub sẽ thất bại với lỗi "invalid_client" từ OAuth server.
- `/api/v1/auth/oauth/google` và `/api/v1/auth/oauth/github` không hoạt động.

**Fix cần thiết**: Đọc credentials từ environment variables (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, etc.).

---

### MOCK-012 — `NewAPIKeyValidator(nil, rdb)`: Cold path sẽ panic

**File**: `services/gateway-service/embedded.go:L125` + `services/gateway-service/internal/auth/apikey_validator.go:L65`  
**Loại**: Nil pointer dereference trên cold path

```go
// embedded.go L125: repo = nil
apiKeyValidator := auth.NewAPIKeyValidator(nil, rdb)

// apikey_validator.go L65: dereference repo trực tiếp không check nil
func (v *APIKeyValidator) Validate(ctx context.Context, rawKey string) (*APIKeyClaims, error) {
    // Hot path: Redis cache hit → OK
    if cached, err := v.cache.Get(ctx, cacheKey).Bytes(); err == nil { ... return ... }

    // Cold path: Redis miss → gọi v.repo.FindByHash() → PANIC nếu repo == nil
    key, err := v.repo.FindByHash(ctx, hash)  // ← nil dereference!
}
```

**Tác động**: Khi Redis cache miss, API Key validation sẽ **panic**. Xảy ra khi Redis restart hoặc API key dùng lần đầu.

> ⚠️ CAUTION: Bug MOCK-012 là **production crash risk**.

---

### MOCK-013 — `search-service` hardcoded `http://localhost:8083`

**File**: `services/gateway-service/embedded.go:L81,97,145`  
**Loại**: Hardcoded URL không đọc từ config

```go
// Dòng 81:
"search-service": "http://localhost:8083",  // ← hardcoded (không dùng cfg.SearchAddr)
// Dòng 97:
"search": "http://localhost:8083",
// Dòng 145:
"search": "http://localhost:8083",
```

**Tác động**: `EmbeddedConfig` có field `SearchAddr` nhưng không được sử dụng. Khi deploy staging/production với search-service trên host khác → proxy fail.

**Fix cần thiết**: Thêm `SearchAddr` vào `EmbeddedConfig` và dùng `coalesce(cfg.SearchAddr, "http://localhost:8083")`.

---

### MOCK-014 — `AlertsHandler` và `SSEHandler` nil: In-app notifications không hoạt động + panic risk

**File**: `services/notification-service/embedded.go:L48` + `services/notification-service/internal/delivery/http/router.go:L73-99`  
**Loại**: Nil handlers — routes bị disable + panic risk

```go
// embedded.go L48: ah=nil, sse=nil
r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, nil, nil, rhHandler, dhHandler)

// router.go L73-78: KHÔNG có nil-check, đăng ký route trực tiếp
r.Route("/api/v2/notifications", func(r chi.Router) {
    r.Get("/", ah.ListNotifications)  // ← PANIC nếu ah == nil
    r.Get("/stream", sse.Stream)      // ← PANIC nếu sse == nil
    ...
})
```

**Tác động**:
- `/api/v2/notifications` → panic ngay khi có request (nil pointer dereference).
- `/api/v2/notifications/stream` (SSE) → panic.
- Toàn bộ in-app notification flow bị phá vỡ.

> ⚠️ WARNING: Khác với các service khác có nil-check, router.go đăng ký `ah.*` trực tiếp gây **panic ngay khi có request**.

---

### MOCK-015 — `NoopFindingClient` + `noopEventPublisher`: Risk scoring luôn = 0

**File**: `services/asset-service/embedded.go:L34-46`  
**Loại**: Noop adapters — business logic bị vô hiệu hoá

```go
// 1. FindingClient noop — risk score = 0
var fc ucasset.FindingClient = &mygrpc.NoopFindingClient{}
// NoopFindingClient.CountBySeverity luôn trả {} → risk score = 0

// 2. EventPublisher noop — events bị drop
crudUC := ucasset.NewAssetCRUDUseCase(repo, &noopEventPublisher{})
```

**Tác động**:
- Asset risk score luôn = 0, không phản ánh mức độ nghiêm trọng thực tế.
- Asset CRUD events không publish → audit log, notification bị thiếu.

**Fix cần thiết**: Wire gRPC `FindingClient` thực sự (env `FINDING_SERVICE_GRPC`). Wire NATS event publisher.

---

## Phân loại theo mức độ

### 🔴 High — Crash hoặc Data Correctness

| Bug ID | Mô tả ngắn |
|--------|------------|
| MOCK-002 | Report Create gây panic (nil dereference) |
| MOCK-007 | Agent registration: fake data, không lưu DB |
| MOCK-008 | Semantic search dùng MockEmbedder → kết quả garbage |
| MOCK-011 | OAuth login không hoạt động (empty credentials) |
| MOCK-012 | API Key cold-path panic (nil repo) |
| MOCK-014 | Notification routes panic ngay khi có request |

### 🟡 Medium — Feature Disabled hoặc Silent Data Loss

| Bug ID | Mô tả ngắn |
|--------|------------|
| MOCK-001 | Report storage mock — không lưu được report |
| MOCK-003 | BulkReopen không publish events |
| MOCK-004 | 7 handlers nil — engagements/tests/members/tools disabled |
| MOCK-005 | NATS noop — events bị mất thầm lặng khi NATS unreachable |
| MOCK-006 | Scan service: 3 handlers không có DB → không lưu data |
| MOCK-009 | OpenSearch bị bỏ qua — chỉ dùng Postgres |
| MOCK-010 | Indexing routes bị tắt |
| MOCK-013 | search-service hardcoded localhost |
| MOCK-015 | Risk scoring luôn 0, asset events mất |

---

## Khuyến nghị Fix theo thứ tự ưu tiên

### P0 — Ngay lập tức (Crash Risks)
1. **MOCK-002**: Thêm nil-check trong `ReportHandler.Create()` → trả 503 thay vì panic.
2. **MOCK-012**: Thêm nil-check trong `APIKeyValidator.Validate()` → trả `ErrInvalidAPIKey` thay vì panic.
3. **MOCK-014**: Thêm nil-check trong `notification-service/router.go` trước khi đăng ký notification routes.

### P1 — Ngắn hạn (Data Correctness)
4. **MOCK-008**: Wire AI embedding service thực sự hoặc disable semantic search route khi dùng MockEmbedder.
5. **MOCK-011**: Đọc OAuth credentials từ env vars (`GOOGLE_CLIENT_ID`, `GITHUB_CLIENT_ID`...).
6. **MOCK-007**: Wire agent repository PostgreSQL.

### P2 — Trung hạn (Feature Completion)
7. **MOCK-001**: Wire PostgreSQL ReportRepo + MinIO Storage.
8. **MOCK-004**: Wire EngagementHandler, TestHandler, RiskAcceptanceHandler, InternalHandler.
9. **MOCK-006**: Wire ScanRepo, StatsRepo cho scan-service.
10. **MOCK-013**: Đưa SearchAddr vào EmbeddedConfig.

### P3 — Dài hạn (Resilience)
11. **MOCK-005**: Implement outbox/retry pattern cho NATS publisher.
12. **MOCK-003**, **MOCK-015**: Wire event publishers thực sự.
