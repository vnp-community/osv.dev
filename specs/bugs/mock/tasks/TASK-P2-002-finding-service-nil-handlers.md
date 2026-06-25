# TASK-P2-002 — Wire 7 nil Handlers trong finding-service

**Bug:** MOCK-004  
**Priority:** 🟡 P2 — Features bị disabled hoàn toàn  
**Effort:** ~3 giờ  
**Service:** `finding-service`  
**Loại thay đổi:** Wire handlers trong embedded.go (repositories đã tồn tại)

---

## Mục tiêu

`finding-service/embedded.go` truyền `nil` cho 7 handlers khi gọi `NewRouter(...)`:
- `EngagementHandler` → `/api/v2/engagements` bị tắt
- `TestHandler` → `/api/v2/tests` bị tắt
- `MemberHandler` → `/api/v2/members` bị tắt
- `ToolHandler` → `/api/v2/tool-configurations` bị tắt
- `RiskAcceptanceHandler` → `/api/v1/risk-acceptances` bị tắt
- `InternalHandler` → `/internal/stats` bị tắt (Dashboard BFF bị ảnh hưởng)
- `SLAHandler` → `/internal/sla-dashboard` bị tắt

---

## Preconditions

- [ ] Đọc `services/finding-service/embedded.go` — xem full `NewRouter(...)` call hiện tại
- [ ] Liệt kê các repositories đã có:
  ```bash
  ls services/finding-service/internal/infra/postgres/
  ```
- [ ] Liệt kê các handlers đã có:
  ```bash
  ls services/finding-service/internal/delivery/http/
  ```
- [ ] Xác định constructor signatures:
  ```bash
  grep -n "func New.*Handler\|func New.*UseCase" \
    services/finding-service/internal/delivery/http/*.go \
    services/finding-service/internal/usecase/*.go
  ```

---

## Steps

### Step 1 — Map handlers đã có vs còn thiếu

Chạy lệnh sau để biết handlers nào đã có:
```bash
grep -n "func New.*Handler" services/finding-service/internal/delivery/http/*.go
```

Với mỗi handler trong danh sách 7 nil handlers, xác định:
- Handler struct đã tồn tại chưa?
- Repo tương ứng đã có chưa (`EngagementRepo`, `MemberRepo`...)?
- UseCase tương ứng đã có chưa?

### Step 2 — Tạo missing repositories (nếu cần)

Với mỗi repo chưa có, tạo theo pattern của các repo đã có.

Kiểm tra từng cái:
```bash
# Engagement repo
find services/finding-service -name "engagement_repo*" -o -name "*engagement*repo*"
# Member repo
find services/finding-service -name "member_repo*" -o -name "*member*repo*"
# Tool config repo
find services/finding-service -name "tool*repo*" -o -name "*tool_config*"
# Risk acceptance repo
find services/finding-service -name "risk*repo*"
# SLA config repo (cho SLAHandler)
find services/finding-service -name "sla*repo*"
```

### Step 3 — Wire tất cả trong embedded.go

Mở `services/finding-service/embedded.go`.

Tìm đoạn `NewRouter(...)` call.

Với mỗi nil slot, thêm code khởi tạo handler tương ứng:

```go
// === FIX MOCK-004: Wire các handlers còn thiếu ===

// Engagement
engagementRepo := postgres.NewEngagementRepo(pool)  // nếu chưa có
engagementHandler := httpdelivery.NewEngagementHandler(engagementRepo, logger)

// Test
testRepo := postgres.NewTestRepo(pool)  // nếu chưa có
testHandler := httpdelivery.NewTestHandler(testRepo, engagementRepo, logger)

// Member
memberRepo := postgres.NewMemberRepo(pool)  // nếu chưa có
memberHandler := httpdelivery.NewMemberHandler(memberRepo, productRepo, logger)

// Tool Config
toolConfigRepo := postgres.NewToolConfigRepo(pool)  // nếu chưa có
toolHandler := httpdelivery.NewToolConfigHandler(toolConfigRepo, logger)

// Risk Acceptance
riskAcceptRepo := postgres.NewRiskAcceptanceRepo(pool)  // nếu chưa có
riskAcceptHandler := httpdelivery.NewRiskAcceptanceHandler(
    riskAcceptRepo, findingRepo, pub, logger)

// Internal Stats (cho BFF dashboard)
// Xác định use case cần thiết bằng cách đọc InternalHandler constructor
internalHandler := httpdelivery.NewInternalHandler(/* args */)

// SLA Handler
slaConfigRepo := postgres.NewSLAConfigRepo(pool)  // nếu chưa có
slaHandler := httpdelivery.NewSLAHandler(/* args */)
```

Thay thế `nil` slots trong `NewRouter(...)`:

```go
router := httpdelivery.NewRouter(
    findingHandler,
    bulkHandler,
    noteHandler,
    engagementHandler,  // FIX: thay nil
    testHandler,         // FIX: thay nil
    memberHandler,       // FIX: thay nil
    toolHandler,         // FIX: thay nil
    reportHandler,
    riskAcceptHandler,   // FIX: thay nil
    internalHandler,     // FIX: thay nil
    slaHandler,          // FIX: thay nil
    productHandler,
    productSeed,
    findingSeed,
    findingGroup,
    logger,
)
```

> **Quan trọng**: Đọc `NewRouter` signature để biết đúng thứ tự tham số!

### Step 4 — Xử lý trường hợp handler/repo chưa được implement

Nếu một handler chưa có implementation thực, không tạo stub mới. Thay vào đó:
- Giữ `nil` cho handler đó nếu router đã có nil-check
- Hoặc tạo handler với nil-check internal (trả 501 Not Implemented)

```go
// Chỉ wire nếu handler đã có implementation thực
// KHÔNG tạo fake handler mới
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/engagements` → không còn 404 (nếu EngagementHandler đã implement)
- [ ] `GET /api/v2/tests` → không còn 404
- [ ] `GET /internal/stats` → response từ InternalHandler (BFF dashboard hoạt động)
- [ ] Tất cả handlers đã wire không có nil repo (trừ những cái intentionally nil)
- [ ] `go build ./services/finding-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/finding-service/...
go vet ./services/finding-service/...

# Verify nil slots reduced in router call
grep -A 20 "NewRouter(" services/finding-service/embedded.go | grep "nil,"
# Expect: fewer nil, items

# Test endpoints
curl -H "X-User-ID: user1" http://localhost:8085/api/v2/engagements
# Expect: 200 với list (có thể rỗng), không phải 404
```
