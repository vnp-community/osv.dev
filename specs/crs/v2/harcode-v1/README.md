# Change Requests — Hardcode & Mock Audit V1

**Phạm vi:** `/services/*` — toàn bộ services trong hệ thống  
**Mục tiêu:** Đưa tất cả services đạt **enterprise-level / production-grade** clean architecture  
**Tiêu chuẩn:**
- Không mock handler, không mock usecase, không mock data trong production code
- Mọi dữ liệu phải lưu vào database phù hợp thông qua repository pattern
- Không hardcode strings, credentials, timestamps, enum values trong handler layer
- Tuân thủ Clean Architecture: Domain → UseCase → Infra → Delivery

---

## Danh sách Change Requests

| CR | Service | Vấn đề | Độ ưu tiên |
|----|---------|---------|-----------|
| [CR-HC-001](CR-HC-001-ai-service-generate-embedding.md) | ai-service | `generate_embedding` usecase là shell rỗng (TODO) | 🔴 Critical |
| [CR-HC-002](CR-HC-002-search-service-mock-embedder.md) | search-service | `MockEmbedder` còn trong production code; search history không lưu DB | 🔴 Critical |
| [CR-HC-003](CR-HC-003-gateway-hardcoded-product-types.md) | gateway-service | `ProductTypes` trả hardcoded enum; `password_policy` hardcode `"medium"` | 🟠 High |
| [CR-HC-004](CR-HC-004-gateway-admin-settings-static.md) | gateway-service | `GetAdminSettings` trả static config không từ DB | 🟠 High |
| [CR-HC-005](CR-HC-005-finding-report-hardcoded-date.md) | finding-service | `report_handler.go` hardcode `"2026-06-22T00:00:00Z"` trong Create response | 🟠 High |
| [CR-HC-006](CR-HC-006-identity-admin-static-permissions.md) | identity-service | Permission categories và role metadata là static, không từ DB | 🟡 Medium |
| [CR-HC-007](CR-HC-007-scan-service-nil-handlers.md) | scan-service | `importHandler`, `parserHandler`, `scheduleHandler` nil trong embedded mode | 🟡 Medium |
| [CR-HC-008](CR-HC-008-data-service-cwe-repo.md) | data-service | `CWEHandler` là nil — repo chưa wired | 🟡 Medium |
| [CR-HC-009](CR-HC-009-data-service-avg-days-patch.md) | data-service | `AvgDaysToPatch = 0` hardcode — không tính từ DB | 🟡 Medium |
| [CR-HC-010](CR-HC-010-jira-service-stub-issues.md) | jira-service | Issue list là stub không lưu DB | 🟡 Medium |
| [CR-HC-011](CR-HC-011-identity-invitation-email.md) | identity-service | `InviteUser` TODO: không gửi email | 🟡 Medium |
| [CR-HC-012](CR-HC-012-search-history-not-persisted.md) | search-service | Search history không lưu vào DB/Redis | 🟡 Medium |
| [CR-HC-013](CR-HC-013-ai-service-batch-enrich.md) | ai-service | `batch_enrich` usecase TODO — không gọi enrich usecase | 🟠 High |
| [CR-HC-014](CR-HC-014-data-service-grpc-cvedb.md) | data-service | gRPC CVEDB client là TODO — không wire proto thật | 🟡 Medium |
| [CR-HC-015](CR-HC-015-gateway-health-grpc-ping.md) | gateway-service | `/health` không ping gRPC upstreams thật | 🟡 Medium |
