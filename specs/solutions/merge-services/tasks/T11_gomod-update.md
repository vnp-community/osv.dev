# T11 — Go Modules Final Update

**Phase**: 11
**Depends on**: T10
**Estimated effort**: 30 minutes

---

## Mục tiêu

Final pass để đảm bảo tất cả `go.mod` của 8 services đều đúng: module names, dependencies, replace directives.

---

## Tác vụ chi tiết

### Bước 1: Kiểm tra module names

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

EXPECTED=(
  "identity-service:github.com/osv/identity-service"
  "data-service:github.com/osv/data-service"
  "search-service:github.com/osv/search-service"
  "scan-service:github.com/osv/scan-service"
  "finding-service:github.com/osv/finding-service"
  "ai-service:github.com/osv/ai-service"
  "notification-service:github.com/osv/notification-service"
  "gateway-service:github.com/osv/gateway-service"
)

for entry in "${EXPECTED[@]}"; do
  SVC="${entry%%:*}"
  MOD="${entry##*:}"
  ACTUAL=$(grep "^module" "$SVC_ROOT/$SVC/go.mod" | awk '{print $2}')
  if [ "$ACTUAL" = "$MOD" ]; then
    echo "✓ $SVC: $MOD"
  else
    echo "✗ $SVC: expected $MOD, got $ACTUAL"
    # Fix:
    sed -i '' "s|^module .*|module $MOD|g" "$SVC_ROOT/$SVC/go.mod"
  fi
done
```

### Bước 2: Đảm bảo replace directives đúng trong tất cả services

```bash
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  GOMOD="$SVC_ROOT/$svc/go.mod"

  # Check shared/pkg replace
  if ! grep -q "github.com/osv/shared/pkg" "$GOMOD"; then
    echo "require github.com/osv/shared/pkg v0.0.0" >> "$GOMOD"
    echo "replace github.com/osv/shared/pkg => ../shared/pkg" >> "$GOMOD"
    echo "Added shared/pkg to $svc"
  fi

  # Check shared/proto replace (nếu service dùng proto)
  if grep -q "shared/proto" "$GOMOD" && ! grep -q "replace.*shared/proto" "$GOMOD"; then
    echo "replace github.com/osv/shared/proto => ../shared/proto" >> "$GOMOD"
    echo "Added shared/proto replace to $svc"
  fi
done
```

### Bước 3: Kiểm tra không còn reference đến services cũ

```bash
# Services đã bị xoá — không được còn trong go.mod của bất kỳ service nào
OLD_SERVICES=(
  "github.com/osv/auth-service"
  "github.com/osv/vulnerability-service"
  "github.com/osv/ingestion-service"
  "github.com/osv/query-service"
  "github.com/osv/dd-search"
  "github.com/osv/schedule-service"
  "github.com/osv/product-service"
  "github.com/osv/report-service"
  "github.com/osv/integration-service"
  "github.com/osv/unified-gateway"
  "github.com/osv/impact-service"
)

for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  for old in "${OLD_SERVICES[@]}"; do
    if grep -q "$old" "$SVC_ROOT/$svc/go.mod"; then
      echo "ERROR: $svc/go.mod still references $old"
    fi
  done
done

echo "Old service reference check complete"
```

### Bước 4: Chạy go mod tidy cho tất cả

```bash
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  echo "=== go mod tidy: $svc ==="
  cd "$SVC_ROOT/$svc"
  go mod tidy
  echo "✓ $svc tidy complete"
done
```

### Bước 5: Final build check

```bash
FAIL=0
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  echo "=== go build: $svc ==="
  cd "$SVC_ROOT/$svc"
  if go build ./...; then
    echo "✓ $svc build OK"
  else
    echo "✗ $svc build FAILED"
    FAIL=1
  fi
done

if [ $FAIL -eq 0 ]; then
  echo ""
  echo "=== ALL 8 SERVICES BUILD SUCCESSFULLY ==="
else
  echo ""
  echo "=== SOME BUILDS FAILED — FIX BEFORE PROCEEDING ==="
  exit 1
fi
```

### Bước 6: Run go vet

```bash
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  cd "$SVC_ROOT/$svc"
  go vet ./... && echo "✓ $svc vet OK" || echo "✗ $svc vet WARNINGS"
done
```

---

## Điều kiện hoàn thành

- [ ] Tất cả 8 `go.mod` có đúng module name
- [ ] Tất cả `go.mod` có `replace github.com/osv/shared/pkg => ../shared/pkg`
- [ ] Không còn reference đến 11 services cũ
- [ ] `go mod tidy` pass cho tất cả 8 services
- [ ] `go build ./...` pass cho tất cả 8 services
- [ ] `go vet ./...` pass

---

## Commit message

```
chore: finalize go.mod for all 8 core services

- Verified all module names correct
- Verified shared/pkg replace directives
- Removed all references to deprecated services
- All 8 services build successfully
```
