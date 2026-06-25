# Solutions — Hardcode Bug Fix v1

> **Ngày tạo**: 2026-06-22  
> **Scope**: 12 bugs trong `/services/**/*.go`  
> **Phương pháp**: Phân tích từng bug → đề xuất giải pháp có code mẫu → ưu tiên theo severity

---

## Danh sách Solution Files

| File | Bug IDs | Nhóm | Severity |
|------|---------|------|----------|
| [SOL-GROUP-A-config-externalization.md](./SOL-GROUP-A-config-externalization.md) | BUG-001, 003, 004, 005 | Config & Network Hardcode | 🔴🔴🟢 |
| [SOL-GROUP-B-gateway-bff-data.md](./SOL-GROUP-B-gateway-bff-data.md) | BUG-002, 011 | Gateway BFF & Data Stubs | 🟡🟡 |
| [SOL-GROUP-C-ai-service-config.md](./SOL-GROUP-C-ai-service-config.md) | BUG-006, 007 | Version & AI Config | 🟢🟡 |
| [SOL-GROUP-D-finding-service-logic.md](./SOL-GROUP-D-finding-service-logic.md) | BUG-008, 009, 010 | Finding Service Business Logic | 🟡🟡🔴 |
| [SOL-GROUP-E-scan-service-config.md](./SOL-GROUP-E-scan-service-config.md) | BUG-012 | Scan Service ZAP Config | 🔴 |
| [SOL-SHARED-config-helper.md](./SOL-SHARED-config-helper.md) | Tất cả | Shared Config Infrastructure | — |

---

## Thứ Tự Fix Được Khuyến Nghị

### Phase 1 — Unblock Production (Sprint hiện tại)

```
BUG-004 → BUG-001 → BUG-003 → BUG-010 → BUG-012
```

Lý do: Đây là những bugs ngăn service hoạt động đúng trong container/K8s.

### Phase 2 — Fix Before Release (Sprint tiếp theo)

```
BUG-009 → BUG-011 → BUG-007 → BUG-002 → BUG-008
```

### Phase 3 — Technical Debt (Backlog)

```
BUG-005 → BUG-006
```

---

## Chiến Lược Tổng Thể

### 1. Shared Config Package

Tạo `shared/pkg/config/` với các helper functions dùng chung cho toàn bộ services,
thay vì mỗi service tự implement env-reading logic riêng.

### 2. Environment Variable Convention

```
<SERVICE>_<DEPENDENCY>_<PROTOCOL>
Ví dụ:
  FINDING_SERVICE_GRPC     → grpc addr của finding-service
  SEARCH_SERVICE_HTTP      → http addr của search-service
  ZAP_BASE_URL             → URL của ZAP scanner
  MINIO_ENDPOINT           → MinIO endpoint cho report storage
```

### 3. Warning Log Pattern

Mọi fallback sang localhost đều phải có `log.Warn()` để dễ debug trong production.

### 4. Fail-Fast Pattern

Credentials và security-sensitive config không được có default — phải fail khi thiếu.
