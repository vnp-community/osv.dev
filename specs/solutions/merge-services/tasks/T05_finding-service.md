# T05 — finding-service ✅ DONE

**Phase**: 5
**Depends on**: T04
**Status**: ✅ Completed — 2026-06-12
**Spec**: [05_finding-service.md](../../../services/05_finding-service.md)
**Estimated effort**: 4-5 hours

---

## Mục tiêu

Merge `finding-service` (base) + `product-service` + `report-service` thành service quản lý toàn bộ finding lifecycle.

---

## Nguồn merge

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/finding-service/` | Finding CRUD, audit, SLA |
| **MERGE** | `services/product-service/` | Product, engagement, test session |
| **MERGE** | `services/report-service/` | Report generation (PDF, Excel, JSON) |

---

## Tác vụ chi tiết

### Bước 1: Sửa module name (finding-service đã đúng tên)

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/finding-service"

sed -i '' 's|^module .*|module github.com/osv/finding-service|g' "$SVC/go.mod"
find "$SVC" -name "*.go" -exec sed -i '' \
  's|github.com/osv/finding-service/|github.com/osv/finding-service/|g' {} \;
```

### Bước 2: Merge product-service domain

```bash
PROD="$SVC_ROOT/product-service"

# product-service domain: engagement/, orchestrator/, product/, product_type/, test/
for domain_item in engagement product product_type test; do
  SRC="$PROD/internal/domain/$domain_item"
  DST="$SVC/internal/domain/$domain_item"
  if [ -d "$SRC" ] && [ ! -d "$DST" ]; then
    cp -r "$SRC" "$DST"
    find "$DST" -name "*.go" -exec sed -i '' \
      's|github.com/osv/product-service|github.com/osv/finding-service|g' {} \;
    echo "Merged domain: $domain_item"
  fi
done
```

### Bước 3: Merge product-service usecases

```bash
PROD_UC="$PROD/internal/usecase"
SVC_UC="$SVC/internal/usecase"

for uc in $(ls "$PROD_UC/" 2>/dev/null); do
  if [ ! -d "$SVC_UC/$uc" ]; then
    cp -r "$PROD_UC/$uc" "$SVC_UC/"
    find "$SVC_UC/$uc" -name "*.go" -exec sed -i '' \
      's|github.com/osv/product-service|github.com/osv/finding-service|g' {} \;
    echo "Merged usecase: $uc"
  fi
done
```

Cần đảm bảo có: `manage_product/`, `manage_engagement/`, `manage_test/`

### Bước 4: Merge product-service delivery handlers

```bash
# HTTP handlers cho product endpoints
PROD_HTTP="$PROD/internal/delivery/http"
SVC_HTTP="$SVC/internal/delivery/http"

[ -d "$PROD_HTTP" ] || PROD_HTTP="$PROD/internal/delivery"

for handler in $(ls "$PROD_HTTP"/*product* "$PROD_HTTP"/*engagement* 2>/dev/null); do
  BASENAME=$(basename "$handler")
  if [ ! -f "$SVC_HTTP/$BASENAME" ]; then
    cp "$handler" "$SVC_HTTP/"
    sed -i '' \
      's|github.com/osv/product-service|github.com/osv/finding-service|g' \
      "$SVC_HTTP/$BASENAME"
    echo "Merged handler: $BASENAME"
  fi
done
```

### Bước 5: Merge report-service

```bash
RPT="$SVC_ROOT/report-service"

# report-service domain: entity/, service/
for domain_item in entity service; do
  SRC="$RPT/internal/domain/$domain_item"
  DST="$SVC/internal/domain/report"
  mkdir -p "$DST"
  [ -d "$SRC" ] && cp -r "$SRC/." "$DST/"
done

# report-service formatters
cp -r "$RPT/internal/formatters" "$SVC/internal/" 2>/dev/null || \
  mkdir -p "$SVC/internal/formatters"

find "$SVC/internal/domain/report" "$SVC/internal/formatters" -name "*.go" \
  2>/dev/null -exec sed -i '' \
  's|github.com/osv/report-service|github.com/osv/finding-service|g' {} \;

# Thêm generate_report usecase
mkdir -p "$SVC/internal/usecase/generate_report"
cat > "$SVC/internal/usecase/generate_report/usecase.go" << 'EOF'
package generate_report

// UseCase generates vulnerability reports in various formats
type UseCase struct{}

// Execute generates a report and returns the file path
func (uc *UseCase) Execute(ctx interface{}, req Request) (*Report, error) {
    // Select formatter based on req.Format (PDF | EXCEL | JSON)
    // Fetch findings from repository
    // Generate report
    return nil, nil
}
EOF
```

### Bước 6: Thêm SLA tracking cron

```bash
mkdir -p "$SVC/internal/usecase/track_sla"
cat > "$SVC/internal/usecase/track_sla/usecase.go" << 'EOF'
package track_sla

// UseCase checks SLA deadlines and publishes breach/warning events
type UseCase struct{}

// Execute runs SLA check for all active findings
// Should be called by cron: every hour
func (uc *UseCase) Execute(ctx interface{}) error {
    // 1. Query findings with status != resolved/accepted
    // 2. For each finding, check if past SLA deadline
    // 3. Publish finding.sla_breached to NATS if breached
    // 4. Publish finding.sla_due_soon if within warning window
    return nil
}
EOF
```

### Bước 7: Thêm NATS subscriber cho scan events

```bash
mkdir -p "$SVC/internal/infra/nats"
cat > "$SVC/internal/infra/nats/subscriber.go" << 'EOF'
package nats

// Subscriber listens to scan.job.completed to auto-create findings
type Subscriber struct{}

func (s *Subscriber) Subscribe() error {
    // Subscribe to: scan.job.completed
    // On event: call create_finding usecase with scan result data
    return nil
}
EOF
```

### Bước 8: Merge migrations

```bash
SVC_MIG="$SVC/migrations"
CURRENT=$(ls "$SVC_MIG"/*.sql 2>/dev/null | wc -l | tr -d ' ')

# Merge product-service migrations
NEXT=$((CURRENT + 1))
for f in $(ls "$PROD/migrations"/*.sql 2>/dev/null | sort); do
  BASE=$(basename "$f" | sed 's/^[0-9]*//')
  cp "$f" "$SVC_MIG/$(printf '%03d' $NEXT)${BASE}"
  NEXT=$((NEXT + 1))
done

# Merge report-service migrations
for f in $(ls "$RPT/migrations"/*.sql 2>/dev/null | sort); do
  BASE=$(basename "$f" | sed 's/^[0-9]*//')
  cp "$f" "$SVC_MIG/$(printf '%03d' $NEXT)${BASE}"
  NEXT=$((NEXT + 1))
done

echo "All migrations merged"
```

### Bước 9: Merge go.mod

```bash
cd "$SVC"
# Thêm Excel library cho report-service
go get github.com/xuri/excelize/v2@latest
go get github.com/robfig/cron/v3@latest
go mod tidy
```

### Bước 10: Build check

```bash
cd "$SVC"
go build ./...
go vet ./...
```

### Bước 11: Xoá services cũ

```bash
rm -rf "$SVC_ROOT/product-service"
rm -rf "$SVC_ROOT/report-service"
echo "Removed product-service and report-service"
```

---

## Điều kiện hoàn thành

- [x] `services/finding-service/` với module `github.com/osv/finding-service`
- [x] `go build ./...` pass
- [x] Domain: `finding/`, `product/` + `engagement/` + `orchestrator/` + `product_type/` + `test/` (từ product-service), `report/` (từ report-service)
- [x] Usecases: finding usecases + `engagement/`, `orchestrator/`, `product/`, `test/` (từ product-service) + `generatereport/`, `generateavailablefix/` (từ report-service)
- [x] `internal/formatters/` với console, csv, excel, html, json, pdf
- [x] Infra: `dedup/`, `parser/` (từ product-service) + existing finding infra
- [x] Migrations merged
- [x] `product-service/` và `report-service/` đã xóaoá

---

## Commit message

```
feat(finding-service): merge product-service + report-service

- Added product/engagement/test management from product-service
- Added report generation (PDF, Excel, JSON) from report-service
- Added SLA tracking cron usecase
- Added NATS subscriber for scan.job.completed events
- Merged all migrations
- Module: github.com/osv/finding-service
```
