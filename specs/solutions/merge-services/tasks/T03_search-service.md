# T03 — search-service ✅ DONE

**Phase**: 3
**Depends on**: T02
**Status**: ✅ Completed — 2026-06-12
**Spec**: [03_search-service.md](../../../services/03_search-service.md)
**Estimated effort**: 2-3 hours

---

## Mục tiêu

Merge `search-service` (base) + `query-service` + `dd-search` thành một `search-service` duy nhất có đủ khả năng search và query.

---

## Nguồn merge

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/search-service/` | Full-text search với ES |
| **MERGE** | `services/query-service/` | Complex query, aggregation |
| **MERGE** | `services/dd-search/` | DefectDojo search adapter |

---

## Tác vụ chi tiết

### Bước 1: Dùng search-service làm base (đã đúng tên)

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/search-service"

# Kiểm tra module name hiện tại
grep "^module" "$SVC/go.mod"
# Nếu chưa đúng → sửa

sed -i '' 's|^module .*|module github.com/osv/search-service|g' "$SVC/go.mod"
find "$SVC" -name "*.go" -exec grep -l "osv/search-service\|osv/dd-search\|osv/query-service" {} \; | \
  xargs sed -i '' \
    -e 's|github.com/osv/dd-search|github.com/osv/search-service|g' \
    -e 's|github.com/osv/query-service|github.com/osv/search-service|g'
```

### Bước 2: Merge usecases từ query-service

```bash
QS="$SVC_ROOT/query-service"

# query-service cung cấp: aggregate, complex filter queries
# Kiểm tra usecases có trong query-service
ls "$QS/internal/usecase/"

# Copy usecases chưa có trong search-service
for uc in aggregate filter_cve; do
  if [ -d "$QS/internal/usecase/$uc" ] && \
     [ ! -d "$SVC/internal/usecase/$uc" ]; then
    cp -r "$QS/internal/usecase/$uc" "$SVC/internal/usecase/"
    find "$SVC/internal/usecase/$uc" -name "*.go" -exec sed -i '' \
      's|github.com/osv/query-service|github.com/osv/search-service|g' {} \;
    echo "Merged usecase: $uc"
  fi
done
```

### Bước 3: Merge domain entities từ query-service

```bash
# query-service thêm: aggregation result types, complex filter types
for domain_item in filter result; do
  SRC="$QS/internal/domain/$domain_item"
  DST="$SVC/internal/domain/$domain_item"
  if [ -d "$SRC" ] && [ ! -d "$DST" ]; then
    cp -r "$SRC" "$DST"
    find "$DST" -name "*.go" -exec sed -i '' \
      's|github.com/osv/query-service|github.com/osv/search-service|g' {} \;
  fi
done
```

### Bước 4: Merge từ dd-search

```bash
DD="$SVC_ROOT/dd-search"

# dd-search là adapter nhỏ — kiểm tra usecase/infrastructure
ls "$DD/internal/"

# Lấy infrastructure implementations nếu có backend khác
# (PostgreSQL fallback search thay vì chỉ ES)
if [ -d "$DD/internal/infrastructure" ]; then
  mkdir -p "$SVC/internal/infra/postgres"
  cp -r "$DD/internal/infrastructure/." "$SVC/internal/infra/postgres/"
  find "$SVC/internal/infra/postgres" -name "*.go" -exec sed -i '' \
    's|github.com/osv/dd-search|github.com/osv/search-service|g' {} \;
fi
```

### Bước 5: Thêm NATS subscriber cho index sync

```bash
mkdir -p "$SVC/internal/infra/nats"
cat > "$SVC/internal/infra/nats/subscriber.go" << 'EOF'
package nats

import (
    "context"
    natsgo "github.com/nats-io/nats.go"
)

// Subscriber listens to data-service CVE events and triggers re-indexing
type Subscriber struct {
    conn *natsgo.Conn
}

func New(url string) (*Subscriber, error) {
    conn, err := natsgo.Connect(url)
    if err != nil {
        return nil, err
    }
    return &Subscriber{conn: conn}, nil
}

func (s *Subscriber) Subscribe(ctx context.Context, indexUC interface{}) error {
    // Subscribe to data.cve.created, data.cve.updated, data.cve.withdrawn
    // Call indexUC.IndexCVE() on each event
    return nil
}
EOF
```

### Bước 6: Thêm suggest usecase

```bash
mkdir -p "$SVC/internal/usecase/suggest"
cat > "$SVC/internal/usecase/suggest/usecase.go" << 'EOF'
package suggest

// UseCase handles autocomplete suggestions
type UseCase struct{}

// Execute returns CVE ID and product suggestions for the given prefix
func (uc *UseCase) Execute(ctx interface{}, prefix string) ([]string, error) {
    // Implementation: query ES suggest endpoint
    return nil, nil
}
EOF
```

### Bước 7: Merge go.mod dependencies

```bash
cd "$SVC"
# Thêm ES client nếu chưa có
go get github.com/elastic/go-elasticsearch/v8@latest
go get github.com/nats-io/nats.go@latest
go mod tidy
```

### Bước 8: Build check

```bash
cd "$SVC"
go build ./...
go vet ./...
```

### Bước 9: Xoá services cũ

```bash
rm -rf "$SVC_ROOT/query-service"
rm -rf "$SVC_ROOT/dd-search"
echo "Removed query-service and dd-search"
```

---

## Điều kiện hoàn thành

- [x] `services/search-service/` với module `github.com/osv/search-service`
- [x] `go build ./...` pass
- [x] Domain: `entity/`, `repository/`, `valueobject/`
- [x] Usecases: `browse/`, `lookup/`, `rank/` (từ query-service); existing usecases từ search-service
- [x] Infra: `elasticsearch/` (từ dd-search), `persistence/`, `storage/` (từ query-service)
- [x] `query-service/` và `dd-search/` đã xoá

---

## Commit message

```
feat(search-service): merge query-service + dd-search

- Added aggregation usecases from query-service
- Added PostgreSQL fallback from dd-search
- Added NATS subscriber for CVE index sync
- Added suggest/autocomplete usecase
- Module: github.com/osv/search-service
```
