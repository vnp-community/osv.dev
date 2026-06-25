# Hardcode Bug Index — v1

> Audit ngày: 2026-06-22  
> Scope: `/services/**/*.go` (942 files)  
> Tổng số bugs: **12**

---

## Bảng Tổng Hợp

| ID | Severity | Service | Loại | Mô tả ngắn | File chính |
|----|----------|---------|------|-------------|-----------|
| [BUG-001](./BUG-001-gateway-hardcoded-service-urls.md) | 🔴 High | gateway-service | URL Hardcode | `search-service` URL không có trong `EmbeddedConfig`, luôn là localhost | `embedded.go:81,97,145` |
| [BUG-002](./BUG-002-gateway-hardcoded-product-types.md) | 🟡 Medium | gateway-service | Data Hardcode | ProductTypes và AdminRoles trả về JSON tĩnh hardcode trong handler | `handler_ui_api.go:608,809` |
| [BUG-003](./BUG-003-osv-handler-hardcoded-addresses.md) | 🔴 High | gateway-service | URL Hardcode | OSV handler fallback localhost gRPC/HTTP, timeout không configurable | `osv_handler.go:46,50,77` |
| [BUG-004](./BUG-004-multi-service-hardcoded-infra-addresses.md) | 🔴 High | multi-service | Credentials + URL | Dev credentials (`osv:osv_dev`) trong DSN default; localhost fallbacks không có warning | `notification-service/main.go:50` |
| [BUG-005](./BUG-005-hardcoded-prometheus-metrics-ports.md) | 🟢 Low | multi-service | Port Hardcode | Prometheus ports hardcode integer literals, không configurable | 4 services |
| [BUG-006](./BUG-006-hardcoded-service-version-strings.md) | 🟢 Low | multi-service | Version Hardcode | `"1.0.0"` hardcode trong logger, tracer, health endpoint | 3 services |
| [BUG-007](./BUG-007-ai-service-hardcoded-model-config.md) | 🟡 Medium | ai-service | Config Hardcode | Ollama URL inconsistency (`localhost` vs `ollama`); model names hardcode | `ollama_adapter.go`, `embed.go` |
| [BUG-008](./BUG-008-finding-service-inconsistent-pagination.md) | 🟡 Medium | finding-service | Magic Number | Default/max limit khác nhau giữa endpoints (20 vs 50 vs 5); clamping ở cả handler lẫn repo | 4 files |
| [BUG-009](./BUG-009-finding-service-hardcoded-grade-logic.md) | 🟡 Medium | finding-service | Business Logic | Grade "A" không bao giờ đạt được; map tạo mới mỗi call; magic number `5` | `product_handler.go:187-201` |
| [BUG-010](./BUG-010-finding-service-nil-report-repo-stub.md) | 🔴 High | finding-service | Stub Object | `nilReportRepo` trả về inconsistent errors: Save lỗi nhưng List trả về empty OK | `embedded.go:96-112` |
| [BUG-011](./BUG-011-data-service-hardcoded-empty-stats.md) | 🟡 Medium | data-service | Incomplete Data | KEV stats `by_vendor` và EPSS `history` luôn trả về `[]` thay vì query thực | `kev_handler.go:199`, `epss_handler.go:142` |
| [BUG-012](./BUG-012-scan-service-hardcoded-zap-config.md) | 🔴 High | scan-service | Config Hardcode | ZAP URL localhost hardcode; timeout duplicated ở 2 files; scheduler intervals không configurable | `zap_client.go`, `cron_worker.go` |

---

## Phân Loại Theo Mức Độ Ưu Tiên

### 🔴 High (phải fix trước khi production)

1. **BUG-001**: Search service URL không thể override → CVE search breaks trong container
2. **BUG-003**: OSV handler gRPC fallback → data lookup fails trong container
3. **BUG-004**: Dev credentials trong source code → security violation
4. **BUG-010**: nilReportRepo trả về empty OK → user confusion và data loss illusion
5. **BUG-012**: ZAP URL localhost → web scanning fails trong container

### 🟡 Medium (fix trước release)

6. **BUG-002**: Hardcoded enums → không thể extend roles/types mà không deploy
7. **BUG-007**: Ollama URL inconsistency → AI fails unpredictably by init path
8. **BUG-008**: Inconsistent pagination → API contract instability
9. **BUG-009**: Grade "A" unreachable bug → scorecard always wrong
10. **BUG-011**: Empty KEV/EPSS stats → dashboard misleading

### 🟢 Low (technical debt)

11. **BUG-005**: Hardcoded metrics ports → port conflicts with multiple instances
12. **BUG-006**: Hardcoded version → observability blindness

---

## Pattern Phổ Biến Nhất

### 1. Localhost fallback không có warning log
```go
// PATTERN (wrong): silent localhost fallback
addr := os.Getenv("SERVICE_ADDR")
if addr == "" {
    addr = "localhost:50060"  // no log, no warning
}
```

Xuất hiện ở: `asset-service`, `finding-service`, `notification-service`, `search-service`, `gateway-service`

### 2. Magic numbers trong pagination
Magic numbers `20`, `50`, `100`, `200`, `5` scatter khắp finding-service
mà không có shared constants.

### 3. Stub objects dùng như production code
`nilReportRepo` trong finding-service là ví dụ điển hình — stub được dùng
trong `embedded.go` production code thay vì chỉ trong tests.

---

## Các TODO Chưa Resolve (Liên Quan)

Ngoài hardcode bugs, có các TODO comments chỉ ra logic chưa implement:

| File | Line | TODO |
|------|------|------|
| `ai-service/cmd/server/main.go` | 52, 62 | Register AIServiceServer; wire use cases |
| `scan-service/cmd/server/main.go` | 43 | Register domain services after proto generation |
| `finding-service/cmd/server/main.go` | 38 | Register handlers after proto generation |
| `data-service/internal/delivery/http/kev_handler.go` | 199-200 | GetStatsByVendor, GetRecentAdditions |
| `data-service/internal/delivery/http/epss_handler.go` | 142 | GetHistory |
| `data-service/internal/infra/persistence/postgres/kev_repo.go` | 327 | AvgDaysToPatch |
| `notification-service/internal/integrations/jira/usecase/use_cases.go` | 150 | JIRA_ENCRYPTION_KEY validation |
