# API Seed Tasks

> **Cập nhật:** 2026-06-18  
> **Nguồn:** [solutions/](../solutions/)  
> **Mục đích:** Danh sách tác vụ atomic cho AI agent thực thi tuần tự.

---

## Danh sách Tasks theo thứ tự thực thi

| # | Task File | Service | Phụ thuộc | Ước lượng |
|---|-----------|---------|-----------|-----------|
| 1 | [TASK-SEED-001-A](./TASK-SEED-001-A-db-migration-role-assignments.md) | identity-service | — | 15 phút |
| 2 | [TASK-SEED-001-B](./TASK-SEED-001-B-identity-domain-layer.md) | identity-service | 001-A | 20 phút |
| 3 | [TASK-SEED-001-C](./TASK-SEED-001-C-identity-infra-postgres.md) | identity-service | 001-B | 30 phút |
| 4 | [TASK-SEED-001-D](./TASK-SEED-001-D-identity-usecase-handlers-gateway.md) | identity-service + gateway | 001-C | 45 phút |
| 5 | [TASK-SEED-002](./TASK-SEED-002-products-hierarchy.md) | finding-service + gateway | 001-D | 60 phút |
| 6 | [TASK-SEED-003](./TASK-SEED-003-findings-bulk-import.md) | finding-service + gateway | SEED-002 | 60 phút |
| 7 | [TASK-SEED-004](./TASK-SEED-004-cve-data-seed.md) | data-service + ranking-service + gateway | 001-D | 60 phút |
| 8 | [TASK-SEED-005-A](./TASK-SEED-005-A-asset-db-migration.md) | asset-service | — | 15 phút |
| 9 | [TASK-SEED-005-B](./TASK-SEED-005-B-asset-service-implementation.md) | asset-service + gateway | 005-A | 60 phút |
| 10 | [TASK-SEED-005-C](./TASK-SEED-005-C-scan-agent-scheduled.md) | scan-service + gateway | 001-D | 30 phút |
| 11 | [TASK-SEED-006](./TASK-SEED-006-config-bulk.md) | sla + notification + jira + gateway | 001-D, SEED-002 | 45 phút |

**Tổng:** ~7.5 giờ

---

## Dependency Graph

```
TASK-SEED-001-A (migration)
    │
    └─→ TASK-SEED-001-B (domain)
            │
            └─→ TASK-SEED-001-C (infra/postgres)
                    │
                    └─→ TASK-SEED-001-D (usecase + handlers + gateway)
                            │
                            ├─→ TASK-SEED-002 (products)
                            │       │
                            │       └─→ TASK-SEED-003 (findings)
                            │
                            ├─→ TASK-SEED-004 (cve data)   [parallel với SEED-002]
                            │
                            ├─→ TASK-SEED-005-C (scan agents) [parallel]
                            │
                            └─→ TASK-SEED-006 (config bulk)  [sau SEED-002]

TASK-SEED-005-A (asset migration)   [độc lập, bắt đầu bất kỳ lúc nào]
    │
    └─→ TASK-SEED-005-B (asset impl + gateway)
```

**Tasks có thể chạy song song:**
- SEED-004, SEED-005-A, SEED-005-C không phụ thuộc lẫn nhau → có thể assign cho nhiều agents cùng lúc.

---

## Checklist tổng quan cho AI agent

### Trước khi bắt đầu mỗi task:
1. Đọc toàn bộ task file
2. Chạy lệnh `khảo sát` trong "Bước 1" để hiểu code hiện tại
3. Verify compile sau mỗi thay đổi: `go build ./...` trong service directory

### Quy tắc quan trọng:
- **Route ordering**: Literal paths PHẢI đứng TRƯỚC wildcard `{id}` paths
- **207 Multi-Status**: Bulk endpoints KHÔNG bao giờ trả 500 cho partial failures
- **Security**: Không log passwords, không trả về `api_key` sau lần đầu, encrypt JIRA tokens
- **Idempotency**: Dùng `ON CONFLICT DO NOTHING` hoặc `DO UPDATE` cho upsert

### Sau khi hoàn thành mỗi task:
1. Verify: `go build ./...`
2. Verify: `go vet ./...`  
3. Test manual theo Acceptance Criteria trong task
4. Cập nhật task file: đánh dấu `[x]` vào checklist

---

## Quy ước đặt tên file

```
TASK-SEED-{số CR}-{thứ tự}-{mô tả ngắn}.md

Ví dụ:
  TASK-SEED-001-A-db-migration-role-assignments.md
  TASK-SEED-001-B-identity-domain-layer.md
  TASK-SEED-002-products-hierarchy.md     ← task duy nhất cho CR
```

Các CR phức tạp (001, 005) được chia thành sub-tasks A, B, C để mỗi task có scope nhỏ hơn, dễ review và test độc lập.
