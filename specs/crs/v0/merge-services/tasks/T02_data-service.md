# T02 — data-service ✅ DONE

**Phase**: 2
**Depends on**: T01
**Status**: ✅ Completed — 2026-06-12
**Spec**: [02_data-service.md](../../../services/02_data-service.md)
**Estimated effort**: 4-5 hours

---

## Mục tiêu

Tạo `services/data-service/` bằng cách merge `vulnerability-service` (base) với `ingestion-service`. Đây là service phức tạp nhất vì gộp 2 bounded contexts lớn.

---

## Nguồn merge

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/vulnerability-service/` | Domain model chính (CVE, KEV, taxonomy, alias) |
| **MERGE** | `services/ingestion-service/` | Fetchers, converters, sync pipeline |

---

## Tác vụ chi tiết

### Bước 1: Copy vulnerability-service làm base

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/data-service"

cp -r "$SVC_ROOT/vulnerability-service/." "$SVC/"
echo "Copied vulnerability-service → data-service"
```

### Bước 2: Đổi module name

```bash
sed -i '' 's|module github.com/osv/vulnerability-service|module github.com/osv/data-service|g' "$SVC/go.mod"

find "$SVC" -name "*.go" -exec sed -i '' \
  's|github.com/osv/vulnerability-service|github.com/osv/data-service|g' {} \;
```

### Bước 3: Tổ chức lại domain/ theo spec

```
internal/domain/ (hiện tại trong vulnerability-service)
├── aggregate/
├── alias/
├── cve/
├── entity/
├── errors/
├── kev/
├── repository/
├── service/
├── taxonomy/
└── valueobject/
```

**Tổ chức lại** theo spec (đặt entity chung vào từng domain):

```bash
cd "$SVC/internal/domain"

# Các subdomain đã đúng tên: cve/, kev/, taxonomy/, alias/
# Đổi aggregate/ → merge vào cve/ nếu là CVE aggregate
# entity/ → chia vào từng subdomain tương ứng
# repository/ → chia vào từng subdomain
# valueobject/ → chia vào từng subdomain

# Tạo source/ domain mới (từ ingestion-service)
mkdir -p source
cat > source/entity.go << 'EOF'
package source

// DataSource defines upstream CVE data sources
type DataSource string

const (
    NVD    DataSource = "NVD"
    OSV    DataSource = "OSV"
    GHSA   DataSource = "GHSA"
    MITRE  DataSource = "MITRE"
    GitHub DataSource = "GITHUB"
)

// SyncJob represents an ingestion/sync job
type SyncJob struct {
    // fields per spec
}
EOF
```

### Bước 4: Copy fetcher/ và pipeline từ ingestion-service

```bash
ING="$SVC_ROOT/ingestion-service"

# Copy các thư mục đặc thù của ingestion-service
cp -r "$ING/internal/fetcher" "$SVC/internal/"
cp -r "$ING/internal/converter" "$SVC/internal/"
cp -r "$ING/internal/pipeline" "$SVC/internal/"
cp -r "$ING/internal/sync" "$SVC/internal/"

# Sửa import paths trong các file vừa copy
find "$SVC/internal/fetcher" "$SVC/internal/converter" \
     "$SVC/internal/pipeline" "$SVC/internal/sync" \
  -name "*.go" -exec sed -i '' \
  's|github.com/osv/ingestion-service|github.com/osv/data-service|g' {} \;

echo "Copied fetcher, converter, pipeline, sync from ingestion-service"
```

### Bước 5: Merge usecases từ ingestion-service

```bash
ING_UC="$ING/internal/usecase"
SVC_UC="$SVC/internal/usecase"

# Usecases từ ingestion-service cần thêm vào data-service:
for uc in ingest sync; do
  if [ -d "$ING_UC/$uc" ]; then
    cp -r "$ING_UC/$uc" "$SVC_UC/"
    find "$SVC_UC/$uc" -name "*.go" -exec sed -i '' \
      's|github.com/osv/ingestion-service|github.com/osv/data-service|g' {} \;
    echo "Copied usecase: $uc"
  fi
done
```

### Bước 6: Merge infra/ từ ingestion-service

```bash
# ingestion-service có thêm: gcs, firestore, nats publisher
# vulnerability-service đã có: postgres, mongo

# Copy các infra adapters chưa có
for infra_dir in firestore gcs nats; do
  if [ -d "$ING/internal/infra/$infra_dir" ] && \
     [ ! -d "$SVC/internal/infra/$infra_dir" ]; then
    cp -r "$ING/internal/infra/$infra_dir" "$SVC/internal/infra/"
    find "$SVC/internal/infra/$infra_dir" -name "*.go" -exec sed -i '' \
      's|github.com/osv/ingestion-service|github.com/osv/data-service|g' {} \;
    echo "Copied infra: $infra_dir"
  fi
done
```

### Bước 7: Merge go.mod dependencies

```bash
# Merge dependencies từ ingestion-service vào data-service go.mod
# Ingestion-service có thêm: cloud.google.com/go/storage, ossf/osv-schema, mongo-driver

# Thêm các deps còn thiếu:
cd "$SVC"
go get cloud.google.com/go/storage@latest
go get cloud.google.com/go/firestore@latest
go get github.com/ossf/osv-schema/bindings/go@latest
go get github.com/robfig/cron/v3@latest
go mod tidy
```

### Bước 8: Cập nhật cmd/server/main.go

Đảm bảo `main.go` khởi động cả:
- HTTP/gRPC server (từ vulnerability-service)
- Cron jobs cho sync (từ ingestion-service)
- NATS publisher

### Bước 9: Merge migrations

```bash
MIG="$SVC/migrations"
ING_MIG="$ING/migrations"

# Đánh số lại migrations từ ingestion-service
# Ví dụ: nếu vuln-service có 001-004, ingestion có 001-003
# → đổi ingestion thành 005, 006, 007

NEXT_NUM=5
for f in $(ls "$ING_MIG"/*.sql 2>/dev/null | sort); do
  BASENAME=$(basename "$f" | sed 's/^[0-9]*//')
  cp "$f" "$MIG/$(printf '%03d' $NEXT_NUM)${BASENAME}"
  NEXT_NUM=$((NEXT_NUM + 1))
done

echo "Merged migrations"
```

### Bước 10: Build check

```bash
cd "$SVC"
go mod tidy
go build ./...
go vet ./...
```

### Bước 11: Xoá services cũ

```bash
rm -rf "$SVC_ROOT/vulnerability-service"
rm -rf "$SVC_ROOT/ingestion-service"
echo "Removed vulnerability-service and ingestion-service"
```

---

## Điều kiện hoàn thành

- [x] `services/data-service/` tồn tại
- [x] `go.mod` module: `github.com/osv/data-service`
- [x] `go build ./...` pass
- [x] Domain: `cve/`, `kev/`, `taxonomy/`, `alias/`, `aggregate/`, `entity/`, `repository/`, `service/`
- [x] Usecases: `cve/`, `kev/`, `sync/`, `syncall/`, `syncsource/`, `alias/`, `query/`, `searchbycpe/`
- [x] Fetchers: từ ingestion-service `fetcher/`, `sync/` (nvd, pypi, circl, ids)
- [x] Converters: `converter/cve5/`, `converter/nvd/`
- [x] Infra: `persistence/`, `mongo/`, `external/`, `messaging/`, `storage/`
- [x] Migrations merged đánh số liên tục
- [x] `services/vulnerability-service/` đã xóa
- [x] `services/ingestion-service/` đã xóa

---

## Commit message

```
feat(data-service): merge vulnerability-service + ingestion-service

- Combined CVE store (vulnerability-service) with data ingestion pipeline
- Added fetchers: NVD, OSV, GHSA, MITRE
- Added sync pipeline (incremental + full)
- Added domain: source/, expanded kev/, taxonomy/, alias/
- Merged 7 migrations (4 from vuln + 3 from ingestion)
- Module: github.com/osv/data-service
```
