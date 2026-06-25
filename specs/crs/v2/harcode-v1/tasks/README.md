# Tasks Index — Hardcode V1 Change Requests

> **Nguồn:** `specs/crs/v2/harcode-v1/solutions/`  
> **Quy ước:** Mỗi task là đơn vị công việc độc lập, AI có thể thực thi không cần hỏi thêm.  
> **Trạng thái:** `🔲 TODO` | `🔄 IN_PROGRESS` | `✅ DONE` | `❌ BLOCKED`

---

## Sprint 1 — Quick Wins (độ phức tạp thấp, impact cao)

| Task | Mô tả | Service | Ước lượng | Status |
|------|-------|---------|-----------|--------|
| [TASK-HC-001](TASK-HC-001-fix-report-hardcoded-date.md) | Fix `created_at` hardcode trong report handler | finding-service | 30 phút | ✅ DONE |
| [TASK-HC-002](TASK-HC-002-wire-cwe-repo.md) | Wire CWERepo vào data-service embed | data-service | 1 giờ | ✅ DONE |
| [TASK-HC-003](TASK-HC-003-implement-avg-days-patch.md) | Implement AvgDaysToPatch từ SQL JOIN | data-service | 1 giờ | ✅ DONE |
| [TASK-HC-004](TASK-HC-004-gateway-health-ready.md) | Implement `/health/ready` với gRPC ping | gateway-service | 2 giờ | ✅ DONE |
| [TASK-HC-005](TASK-HC-005-remove-mock-embedder.md) | Xóa MockEmbedder + trả 503 khi AI down | search-service | 1 giờ | ✅ DONE |

---

## Sprint 2 — Core Architecture (độ phức tạp trung bình)

| Task | Mô tả | Service | Ước lượng | Status |
|------|-------|---------|-----------|--------|
| [TASK-HC-006](TASK-HC-006-generate-embedding-usecase.md) | Implement generate_embedding UseCase thật | ai-service | 3 giờ | 🔲 TODO |
| [TASK-HC-007](TASK-HC-007-search-history-persistence.md) | Search history migration + repository + handler | search-service | 3 giờ | 🔲 TODO |
| [TASK-HC-008](TASK-HC-008-product-types-from-db.md) | ProductTypes từ DB (migration + repo + handler + proxy) | product+gateway | 4 giờ | 🔲 TODO |
| [TASK-HC-009](TASK-HC-009-admin-settings-from-db.md) | Admin Settings từ DB (migration + repo + handler) | identity-service | 4 giờ | 🔲 TODO |
| [TASK-HC-010](TASK-HC-010-rbac-matrix-from-db.md) | RBAC Matrix từ DB (role_metadata + permission_categories) | identity-service | 3 giờ | 🔲 TODO |
| [TASK-HC-011](TASK-HC-011-scan-nil-handlers.md) | Wire scan nil handlers + ScheduleHandler thật | scan-service | 4 giờ | 🔲 TODO |
| [TASK-HC-012](TASK-HC-012-ai-batch-enrich.md) | AI Batch Enrichment UseCase với real concurrency | ai-service | 4 giờ | 🔲 TODO |

---

## Sprint 3 — Complex Features (độ phức tạp cao)

| Task | Mô tả | Service | Ước lượng | Status |
|------|-------|---------|-----------|--------|
| [TASK-HC-013](TASK-HC-013-jira-issue-crud.md) | Jira Issue CRUD thật (API client + usecase + repo) | jira-service | 6 giờ | 🔲 TODO |
| [TASK-HC-014](TASK-HC-014-user-invitation-email.md) | User Invitation với email SMTP thật | identity-service | 6 giờ | 🔲 TODO |
| [TASK-HC-015](TASK-HC-015-grpc-cvedb-server.md) | gRPC CVEDB PopulateDB server implementation | data-service | 8 giờ | 🔲 TODO |

---

## Nguyên tắc thực thi cho AI Agent

1. **Đọc solution** trước khi bắt đầu — solution chứa code mẫu đầy đủ
2. **Migration trước** — luôn chạy SQL migration trước khi implement repository  
3. **Build check** — chạy `go build ./...` sau mỗi task
4. **Cập nhật status** trong file này sau khi hoàn thành
5. **Test ngay** — chạy integration test sau mỗi task

## Thứ tự ưu tiên thực thi

```
TASK-HC-001 → TASK-HC-002 → TASK-HC-003 → TASK-HC-004 → TASK-HC-005
     ↓              (độc lập)
TASK-HC-006 phụ thuộc vào TASK-HC-005 (provider chain)
TASK-HC-007 độc lập
TASK-HC-008 → (TASK-HC-009 + TASK-HC-010) song song
TASK-HC-011 độc lập
TASK-HC-012 phụ thuộc vào TASK-HC-006
TASK-HC-013 → TASK-HC-014 độc lập
TASK-HC-015 độc lập
```
