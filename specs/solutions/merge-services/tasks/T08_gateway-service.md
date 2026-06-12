# T08 — gateway-service

**Phase**: 8
**Depends on**: T07
**Spec**: [08_gateway-service.md](../../../services/08_gateway-service.md)
**Estimated effort**: 1-2 hours

---

## Mục tiêu

Rename `unified-gateway` → `gateway-service` và cập nhật routing config để trỏ đến đúng service names mới.

---

## Nguồn

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/unified-gateway/` | Toàn bộ gateway code |

---

## Tác vụ chi tiết

### Bước 1: Copy unified-gateway → gateway-service

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/gateway-service"
GW="$SVC_ROOT/unified-gateway"

cp -r "$GW/." "$SVC/"
echo "Copied unified-gateway → gateway-service"
```

### Bước 2: Đổi module name

```bash
sed -i '' 's|^module .*|module github.com/osv/gateway-service|g' "$SVC/go.mod"

find "$SVC" -name "*.go" -exec sed -i '' \
  's|github.com/osv/unified-gateway|github.com/osv/gateway-service|g' {} \;

echo "Module renamed to github.com/osv/gateway-service"
```

### Bước 3: Cập nhật upstream service names trong config

Cấu hình routing cần cập nhật để dùng tên service mới:

```bash
CONFIG="$SVC/config"
[ -d "$CONFIG" ] || mkdir -p "$CONFIG"

# Cập nhật bất kỳ config/env reference nào đến service names cũ
find "$SVC" \( -name "*.go" -o -name "*.yaml" -o -name "*.yml" \) -exec sed -i '' \
  -e 's|auth-service|identity-service|g' \
  -e 's|vulnerability-service|data-service|g' \
  -e 's|ingestion-service|data-service|g' \
  -e 's|unified-gateway|gateway-service|g' {} \;

echo "Updated service name references"
```

### Bước 4: Tạo route config chính thức

```bash
cat > "$SVC/config/routes.yaml" << 'EOF'
# Route configuration for gateway-service
# Maps URL prefixes to upstream gRPC services

routes:
  # Auth (public)
  - prefix: "/api/v1/auth/register"
    upstream: "identity-service:50051"
    auth_required: false

  - prefix: "/api/v1/auth/login"
    upstream: "identity-service:50051"
    auth_required: false

  - prefix: "/api/v1/auth/oauth"
    upstream: "identity-service:50051"
    auth_required: false

  # Auth (protected)
  - prefix: "/api/v1/auth"
    upstream: "identity-service:50051"
    auth_required: true

  # CVE Data
  - prefix: "/api/v1/cve"
    upstream: "data-service:50052"
    auth_required: true
    roles: ["viewer", "analyst", "admin"]

  # Search
  - prefix: "/api/v1/search"
    upstream: "search-service:50053"
    auth_required: true
    roles: ["viewer", "analyst", "admin"]

  # Scanning
  - prefix: "/api/v1/scan"
    upstream: "scan-service:50054"
    auth_required: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/assets"
    upstream: "scan-service:50054"
    auth_required: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/schedules"
    upstream: "scan-service:50054"
    auth_required: true
    roles: ["analyst", "admin"]

  # Findings & Products
  - prefix: "/api/v1/findings"
    upstream: "finding-service:50055"
    auth_required: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/products"
    upstream: "finding-service:50055"
    auth_required: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/reports"
    upstream: "finding-service:50055"
    auth_required: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/sla"
    upstream: "finding-service:50055"
    auth_required: true
    roles: ["analyst", "admin"]

  # AI
  - prefix: "/api/v1/ai"
    upstream: "ai-service:50056"
    auth_required: true
    roles: ["analyst", "admin"]

  # Notifications
  - prefix: "/api/v1/notifications"
    upstream: "notification-service:50057"
    auth_required: true

  - prefix: "/api/v1/rules"
    upstream: "notification-service:50057"
    auth_required: true

  - prefix: "/api/v1/webhooks"
    upstream: "notification-service:50057"
    auth_required: true

  - prefix: "/api/v1/integrations"
    upstream: "notification-service:50057"
    auth_required: true

  # BFF endpoints (handled by gateway itself)
  - prefix: "/api/v1/bff"
    upstream: "gateway-bff"
    auth_required: true

  # Health (public)
  - prefix: "/health"
    upstream: "gateway-health"
    auth_required: false
EOF
echo "Created routes.yaml"
```

### Bước 5: Tạo upstreams config

```bash
cat > "$SVC/config/upstreams.yaml" << 'EOF'
# Upstream service gRPC addresses
upstreams:
  identity-service:    "${IDENTITY_SERVICE_ADDR:-identity-service:50051}"
  data-service:        "${DATA_SERVICE_ADDR:-data-service:50052}"
  search-service:      "${SEARCH_SERVICE_ADDR:-search-service:50053}"
  scan-service:        "${SCAN_SERVICE_ADDR:-scan-service:50054}"
  finding-service:     "${FINDING_SERVICE_ADDR:-finding-service:50055}"
  ai-service:          "${AI_SERVICE_ADDR:-ai-service:50056}"
  notification-service: "${NOTIFICATION_SERVICE_ADDR:-notification-service:50057}"
EOF
echo "Created upstreams.yaml"
```

### Bước 6: Cập nhật gRPC client imports

```bash
# Các gRPC clients cần point đến đúng proto packages
# Check hiện tại
grep -r "grpc.Dial\|grpc.NewClient" "$SVC/internal/" --include="*.go" -l

# Cập nhật nếu dùng hardcoded service addresses
find "$SVC/internal" -name "*.go" -exec sed -i '' \
  -e 's|"auth-service:50051"|"identity-service:50051"|g' \
  -e 's|"vulnerability-service:50052"|"data-service:50052"|g' {} \;
```

### Bước 7: Thêm BFF handlers nếu chưa có

```bash
BFF_DIR="$SVC/internal/bff"
[ -d "$BFF_DIR" ] || mkdir -p "$BFF_DIR"

cat > "$BFF_DIR/dashboard.go" << 'EOF'
package bff

// DashboardAggregator aggregates data from multiple services for dashboard
type DashboardAggregator struct {
    // finding client
    // scan client
    // data client
}

// GetDashboard fetches and combines data for the main dashboard view
func (a *DashboardAggregator) GetDashboard(ctx interface{}) (*DashboardData, error) {
    // Parallel calls to finding-service, scan-service, data-service
    // Combine into DashboardData
    return nil, nil
}

type DashboardData struct {
    Findings FindingsSummary
    Scans    ScansSummary
    KEV      KEVSummary
}
EOF
echo "Created BFF dashboard aggregator"
```

### Bước 8: Build check

```bash
cd "$SVC"
go mod tidy
go build ./...
go vet ./...
```

### Bước 9: Xoá service cũ

```bash
rm -rf "$SVC_ROOT/unified-gateway"
echo "Removed unified-gateway"
```

---

## Điều kiện hoàn thành

- [ ] `services/gateway-service/` với module `github.com/osv/gateway-service`
- [ ] `go build ./...` pass
- [ ] `config/routes.yaml` với mapping đầy đủ tất cả 8 services
- [ ] `config/upstreams.yaml` với tất cả service addresses
- [ ] Domain: `auth/`, `policy/`, `entity/`
- [ ] BFF aggregator tồn tại
- [ ] `unified-gateway/` đã xoá

---

## Commit message

```
feat(gateway-service): rename unified-gateway + update routing

- Renamed from unified-gateway to gateway-service
- Updated route config to point to all 8 new service names
- Created config/routes.yaml with full route mapping
- Created config/upstreams.yaml
- Added BFF dashboard aggregator
- Module: github.com/osv/gateway-service
```
