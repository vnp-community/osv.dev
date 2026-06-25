# TASK-INDEX — Index Tất Cả Tasks Init-Account

> **Nguồn**: [Solutions](../solutions/SOL-INIT-000-index.md)  
> **Mục tiêu**: Sau khi thực thi tất cả tasks, `cp .env.bootstrap .env && ./scripts/bootstrap.sh` → start binaries → đăng nhập ngay

---

## Danh sách Tasks

| Task | Trạng thái | Files đã thay đổi | Build |
|------|-----------|-------------------|-------|
| [TASK-001](./TASK-INIT-001-env-bootstrap.md) | ✅ DONE | `.env.bootstrap` (NEW), `apps/osv/.env.example` (MODIFY) | — |
| [TASK-002](./TASK-INIT-002-identity-service.md) | ✅ DONE | `scripts/init.sh` (NEW), `migrations/001_initial_schema.sql`, `cmd/server/main.go`, `adapter/handler/http/router.go` | ✅ OK |
| [TASK-003](./TASK-INIT-003-data-service.md) | ✅ DONE | `scripts/init.sh` (NEW), `internal/config/storage_config.go` (BUG FIX), `cmd/server/main.go` | ✅ OK |
| [TASK-004](./TASK-INIT-004-005-search-ranking.md) §004 | ✅ DONE | `scripts/init.sh` (NEW), `cmd/server/main.go` (SEARCH_ prefix, REDIS_PASSWORD) | ✅ OK |
| [TASK-005](./TASK-INIT-004-005-search-ranking.md) §005 | ✅ DONE | `scripts/init.sh` (NEW), `cmd/server/main.go` (RANKING_PORT) | ✅ OK |
| [TASK-006](./TASK-INIT-006-007-notification-ai.md) §006 | ✅ DONE | `scripts/init.sh` (NEW), `cmd/server/main.go` (NOTIFICATION_ prefix) | ✅ OK |
| [TASK-007](./TASK-INIT-006-007-notification-ai.md) §007 | ✅ DONE | `scripts/init.sh` (NEW), `cmd/server/main.go` (AI_GRPC_PORT, real gRPC listener) | ✅ OK |
| [TASK-008](./TASK-INIT-008-009-gateway-bootstrap.md) §008 | ✅ DONE | `embedded.go` (BUG FIX: coalesce + AIAddr/RankingAddr), `scripts/init.sh` (NEW×2) | ✅ OK |
| [TASK-009](./TASK-INIT-008-009-gateway-bootstrap.md) §009 | ✅ DONE | `scripts/bootstrap.sh` (NEW), `scripts/health-check.sh` (NEW), `scripts/start-all.sh` (NEW) | ✅ OK |

---

## Thứ tự thực thi đề xuất

```
TASK-001  →  TASK-002  →  TASK-003  →  TASK-004  →  TASK-005
              ↓ (phụ thuộc migrations/schema)
            TASK-006  →  TASK-007  →  TASK-008  →  TASK-009
```

**Lý do thứ tự**:
- TASK-001 trước vì `.env.bootstrap` cần có cho các script khác test
- TASK-002 trước TASK-009 vì `scripts/bootstrap.sh` gọi `identity-service/scripts/init.sh`
- TASK-003 ưu tiên cao do bug nghiêm trọng (data-service sẽ không đọc env)

---

## Bugs Quan Trọng Cần Fix (Tham chiếu từ đọc code thực tế)

### BUG-001 (CRITICAL) — data-service `envOr()` placeholder
- **File**: `services/data-service/internal/config/storage_config.go`
- **Vấn đề**: `return defaultVal` — không bao giờ đọc OS env
- **Status**: ✅ FIXED — TASK-003

### BUG-002 (HIGH) — identity-service migration schema missing
- **File**: `services/identity-service/migrations/001_initial_schema.sql`
- **Vấn đề**: `SET search_path TO auth` trước khi schema được tạo
- **Status**: ✅ FIXED — TASK-002

### BUG-003 (MEDIUM) — gateway-service hardcoded upstream addresses
- **File**: `services/gateway-service/embedded.go` dòng 49-54
- **Vấn đề**: Địa chỉ upstream hardcode, không dùng `EmbeddedConfig`
- **Status**: ⬜ TODO — TASK-008

---

## Files mới sẽ được tạo

```
/Users/binhnt/Lab/sec/cve/osv.dev/
├── .env.bootstrap                                        ← TASK-001
├── scripts/
│   ├── bootstrap.sh                                      ← TASK-009
│   ├── health-check.sh                                   ← TASK-009
│   └── start-all.sh                                      ← TASK-009
├── services/
│   ├── identity-service/scripts/init.sh                  ← TASK-002
│   ├── data-service/scripts/init.sh                      ← TASK-003
│   ├── search-service/scripts/init.sh                    ← TASK-004
│   ├── ranking-service/scripts/init.sh                   ← TASK-005
│   ├── notification-service/scripts/init.sh              ← TASK-006
│   ├── ai-service/scripts/init.sh                        ← TASK-007
│   └── gateway-service/scripts/init.sh                   ← TASK-008
└── apps/
    └── osv/scripts/init.sh                               ← TASK-008
```

---

## Files sẽ được sửa

```
apps/osv/.env.example                                     ← TASK-001 (append)
```
services/identity-service/
  ├── migrations/001_initial_schema.sql          ← TASK-002 ✅ (CREATE SCHEMA + extensions)
  ├── adapter/handler/http/router.go             ← TASK-002 ✅ (thêm /health route)
  └── cmd/server/main.go                        ← TASK-002 ✅ (IDENTITY_ prefix + getEnvFallback)
services/data-service/
  ├── internal/config/storage_config.go                   ← TASK-003 (FIX envOr bug)
  └── cmd/server/main.go                                  ← TASK-003 (DATA_ prefix)
services/search-service/cmd/server/main.go                ← TASK-004 (SEARCH_ prefix, REDIS_PASSWORD)
services/ranking-service/cmd/server/main.go               ← TASK-005 (RANKING_PORT)
services/notification-service/cmd/server/main.go          ← TASK-006 (NOTIFICATION_ prefix)
services/ai-service/cmd/server/main.go                    ← TASK-007 (AI_GRPC_PORT, gRPC listener)
services/gateway-service/embedded.go                      ← TASK-008 (fix hardcoded addrs)
```
