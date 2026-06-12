# T09 — Shared Layer Update

**Phase**: 9
**Depends on**: T08
**Estimated effort**: 30 minutes

---

## Mục tiêu

Cập nhật module name của `shared/pkg` từ `github.com/osv/pkg` → `github.com/osv/shared/pkg` để nhất quán với cách các services reference nó trong `go.mod`. Cập nhật tất cả `replace` directives.

---

## Tác vụ chi tiết

### Bước 1: Kiểm tra module name hiện tại

```bash
grep "^module" /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/pkg/go.mod
# Hiện tại: module github.com/osv/pkg
# Kỳ vọng: module github.com/osv/shared/pkg
```

### Bước 2: Cập nhật shared/pkg module name

```bash
SHARED_PKG="/Users/binhnt/Lab/sec/cve/osv.dev/services/shared/pkg"

sed -i '' 's|^module github.com/osv/pkg$|module github.com/osv/shared/pkg|g' "$SHARED_PKG/go.mod"

# Cập nhật tất cả internal imports trong shared/pkg nếu có self-reference
find "$SHARED_PKG" -name "*.go" -exec sed -i '' \
  's|"github.com/osv/pkg/|"github.com/osv/shared/pkg/|g' {} \;

echo "Updated shared/pkg module name"
```

### Bước 3: Cập nhật replace directives trong tất cả 8 services

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  GOMOD="$SVC_ROOT/$svc/go.mod"

  if [ -f "$GOMOD" ]; then
    # Đảm bảo require và replace đúng
    # Thay thế reference cũ (github.com/osv/pkg) bằng mới (github.com/osv/shared/pkg)
    sed -i '' \
      -e 's|github.com/osv/pkg |github.com/osv/shared/pkg |g' \
      -e 's|github.com/osv/pkg$|github.com/osv/shared/pkg|g' \
      "$GOMOD"

    # Cập nhật replace path (relative path không đổi nếu đúng cấu trúc)
    # replace github.com/osv/shared/pkg => ../shared/pkg
    if ! grep -q "github.com/osv/shared/pkg => ../shared/pkg" "$GOMOD"; then
      echo "" >> "$GOMOD"
      echo "replace github.com/osv/shared/pkg => ../shared/pkg" >> "$GOMOD"
    fi

    echo "Updated $svc/go.mod"
  fi
done
```

### Bước 4: Cập nhật imports trong tất cả Go files

```bash
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  find "$SVC_ROOT/$svc" -name "*.go" -exec sed -i '' \
    's|"github.com/osv/pkg/|"github.com/osv/shared/pkg/|g' {} \;
  echo "Updated imports in $svc"
done
```

### Bước 5: Chạy go mod tidy cho tất cả services

```bash
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  echo "=== go mod tidy: $svc ==="
  cd "$SVC_ROOT/$svc" && go mod tidy
done
```

### Bước 6: Build check toàn bộ

```bash
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  echo "=== go build: $svc ==="
  cd "$SVC_ROOT/$svc" && go build ./...
done
```

---

## Điều kiện hoàn thành

- [ ] `shared/pkg/go.mod`: `module github.com/osv/shared/pkg`
- [ ] Tất cả 8 services có `replace github.com/osv/shared/pkg => ../shared/pkg`
- [ ] Không còn reference đến `github.com/osv/pkg` trong bất kỳ file nào
- [ ] `go build ./...` pass cho tất cả 8 services

---

## Commit message

```
chore(shared): normalize module name to github.com/osv/shared/pkg

- Updated shared/pkg module name for consistency
- Updated replace directives in all 8 services
- Updated all import paths
```
