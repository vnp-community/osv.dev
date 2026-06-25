# TASK-P3-003 — Wire FindingClient gRPC + NATS Publisher cho asset-service

**Bug:** MOCK-015  
**Priority:** 🟡 P3 — Business Logic bị vô hiệu hoá  
**Effort:** ~2 giờ  
**Service:** `asset-service`  
**Loại thay đổi:** New file (AssetEventPublisher) + Sửa embedded.go

---

## Mục tiêu

1. `NoopFindingClient` → risk score luôn = 0 (không phản ánh finding severity)
2. `noopEventPublisher` → asset CRUD events bị drop → audit log thiếu

---

## Preconditions

- [ ] Đọc `services/asset-service/embedded.go`
- [ ] Xác định `FindingClient` interface:
  ```bash
  grep -rn "type FindingClient interface\|FindingClient" services/asset-service/internal/
  ```
- [ ] Kiểm tra FindingClient gRPC implementation đã có chưa:
  ```bash
  find services/asset-service -name "*finding*client*" -o -name "*grpc*finding*"
  ```
- [ ] Xác định `EventPublisher` interface:
  ```bash
  grep -rn "type EventPublisher interface" services/asset-service/internal/
  ```
- [ ] Xác định module name: `grep "^module" services/asset-service/go.mod`

---

## Steps

### Step 1 — Phần 1: Wire FindingClient gRPC thực sự

Kiểm tra xem `FindingClient` gRPC implementation đã có chưa:
```bash
find services/asset-service -name "*.go" | xargs grep -l "NewFindingClient\|FindingGRPC" 2>/dev/null
```

**Nếu đã có** `mygrpc.NewFindingClient(target)`:

Mở `services/asset-service/embedded.go`.

Tìm:
```go
var fc ucasset.FindingClient = &mygrpc.NoopFindingClient{}
```

Thay bằng:
```go
// FIX MOCK-015: Wire real gRPC FindingClient
var fc ucasset.FindingClient = &mygrpc.NoopFindingClient{} // graceful fallback

grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
if grpcTarget == "" {
    grpcTarget = "localhost:50060"
}

realFC, err := mygrpc.NewFindingClient(grpcTarget)
if err != nil {
    logger.Warn().Err(err).Str("target", grpcTarget).
        Msg("asset-service: Finding gRPC unavailable, risk scoring disabled (score=0)")
} else {
    fc = realFC
    logger.Info().Str("target", grpcTarget).
        Msg("asset-service: Finding gRPC connected, risk scoring enabled")
}
```

**Nếu chưa có** FindingClient implementation, bỏ qua phần này và chỉ fix NATS.

### Step 2 — Phần 2: Tạo NATS AssetEventPublisher

Kiểm tra xem đã có NATS publisher chưa:
```bash
find services/asset-service -name "*.go" | xargs grep -l "nats.Conn\|natsgo" 2>/dev/null
```

**File mới**: `services/asset-service/internal/infra/nats/asset_publisher.go`

```go
package nats

import (
    "encoding/json"
    "fmt"
    natsgo "github.com/nats-io/nats.go"
)

// AssetEventPublisher publishes asset domain events to NATS.
// Implements the ucasset.EventPublisher interface.
type AssetEventPublisher struct {
    nc *natsgo.Conn
}

func NewAssetEventPublisher(nc *natsgo.Conn) *AssetEventPublisher {
    return &AssetEventPublisher{nc: nc}
}

// Publish publishes an event with subject "asset.<subject>".
func (p *AssetEventPublisher) Publish(subject string, payload map[string]any) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("asset event marshal: %w", err)
    }
    return p.nc.Publish("asset."+subject, data)
}
```

### Step 3 — Wire NATS EventPublisher trong embedded.go

Mở `services/asset-service/embedded.go`.

Tìm:
```go
crudUC := ucasset.NewAssetCRUDUseCase(repo, &noopEventPublisher{})
```

Thêm NATS init trước dòng này:

```go
// FIX MOCK-015: Wire NATS EventPublisher thay vì noop
var eventPub ucasset.EventPublisher = &noopEventPublisher{} // fallback

natsURL := os.Getenv("NATS_URL")
if natsURL == "" {
    natsURL = "nats://localhost:4222"
}
nc, natsErr := natsgo.Connect(natsURL,
    natsgo.RetryOnFailedConnect(true),
    natsgo.MaxReconnects(5),
)
if natsErr != nil {
    logger.Warn().Err(natsErr).Msg("asset-service: NATS unavailable, asset events disabled")
} else {
    eventPub = assetinfra.NewAssetEventPublisher(nc)
    logger.Info().Msg("asset-service: NATS event publisher connected")
}

crudUC := ucasset.NewAssetCRUDUseCase(repo, eventPub)
```

### Step 4 — Cập nhật docker-compose

```yaml
services:
  osv-monolith:
    environment:
      - FINDING_SERVICE_GRPC=localhost:50060
      - NATS_URL=nats://nats:4222
```

---

## Acceptance Criteria

- [ ] Asset risk score phản ánh finding severity thực tế (khi FindingClient connected)
- [ ] `asset.created`, `asset.updated`, `asset.deleted` events được publish tới NATS
- [ ] Khi NATS unavailable → `noopEventPublisher` fallback (không crash)
- [ ] `go build ./services/asset-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/asset-service/...
go vet ./services/asset-service/...

# Verify noop removed
grep -n "noopEventPublisher\|NoopFindingClient" services/asset-service/embedded.go
# Expected: chỉ còn trong fallback declaration, không phải final assignment

go test ./services/asset-service/internal/... -v
```
