# TASK-00: Prerequisites & Codebase Audit

**Phase**: 0 — Preparation  
**Ước tính**: 2 giờ  
**Phụ thuộc**: Không có  
**Output**: Báo cáo tình trạng codebase, môi trường sẵn sàng

---

## Mục tiêu

Kiểm tra toàn bộ codebase `services/` để xác định chính xác các internal packages có thể import, các interface cần implement, và các dependency conflict cần xử lý.

---

## T-00.1: Kiểm tra Go Version & Toolchain

```bash
go version
# Expected: go1.26.3+

# Verify go.work
cat /Users/binhnt/Lab/sec/cve/osv.dev/services/go.work
```

**Checklist**:
- [ ] Go 1.26.3+ đã cài đặt
- [ ] go.work tồn tại và hợp lệ
- [ ] `buf` CLI có sẵn (cho proto generation nếu cần)

---

## T-00.2: Audit từng Service — Public API scan

Chạy lệnh sau cho từng service để xác định các package có thể import:

```bash
# Liệt kê tất cả packages trong mỗi service
for svc in auth-service finding-service product-service scan-service \
           vulnerability-service notification-service report-service \
           ai-service impact-service integration-service search-service \
           ingestion-service unified-gateway; do
  echo "=== $svc ==="
  find /Users/binhnt/Lab/sec/cve/osv.dev/services/$svc -name "*.go" \
    | head -30
done
```

**Kiểm tra từng service**:

### auth-service
```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/auth-service/internal/
# Tìm: domain, infra, usecase, delivery
# Ghi chú: package names, exported types
```
- [ ] Xác định package `domain/entity/user.go` — exported User type
- [ ] Xác định `infra/` — DB repository implementations
- [ ] Xác định `usecase/` — use case interfaces & implementations
- [ ] Xác định `delivery/grpc/` — gRPC server handler
- [ ] Xác định `delivery/http/` hoặc `adapter/http/` — HTTP handlers

### finding-service
```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/
cat /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/domain/finding/repository.go
```
- [ ] ✅ Đã xác định: `Repository` interface (Create, BulkCreate, FindByID, ...)
- [ ] ✅ Đã xác định: `Finding` entity
- [ ] Xác định `delivery/grpc/` — FindingService gRPC handler
- [ ] Xác định `infra/` — PostgresRepository implementation
- [ ] Xác định `usecase/finding/` — FindingUseCase

### product-service
```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/product-service/internal/
cat /Users/binhnt/Lab/sec/cve/osv.dev/services/product-service/internal/domain/product/entity.go
```
- [ ] ✅ Đã xác định: `Product` entity
- [ ] Xác định `domain/engagement/` — Engagement entity
- [ ] Xác định `domain/orchestrator/` — Orchestrator pattern
- [ ] Xác định `delivery/grpc/` — ProductService gRPC handler

### scan-service
```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/parsers/
```
- [ ] Xác định parser package structure
- [ ] Xác định `domain/scan/` — Scan entity
- [ ] Xác định `domain/agent/` — Scanner agent
- [ ] Xác định `usecase/` — ScanUseCase

### notification-service
```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/domain/
```
- [ ] Xác định `domain/rule/` — Alert rule
- [ ] Xác định `domain/alert/` — Alert entity
- [ ] Xác định `adapter/` — email, slack, webhook adapters

### report-service
```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/report-service/internal/
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/report-service/internal/formatters/
```
- [ ] Xác định formatter packages (PDF, HTML, CSV, JSON)
- [ ] Xác định `adapter/` — finding-service, product-service clients

### Remaining services
- [ ] Xác định ai-service: NATS consumer pattern, LLM adapter
- [ ] Xác định impact-service: CVE impact assessment
- [ ] Xác định integration-service: JIRA adapter, GitHub adapter
- [ ] Xác định search-service: OpenSearch adapter
- [ ] Xác định ingestion-service: data pipeline, parsers
- [ ] Xác định unified-gateway: existing route patterns

---

## T-00.3: Kiểm tra Proto Generated Code

```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/proto/gen/
# Tìm: go/ directory với generated pb.go files

ls /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/proto/gen/go/
# Expected: auth/v1/, finding/dd/v1/, product/dd/v1/, scan/v1/, ...
```

- [ ] Xác định gen/go path structure
- [ ] Xác định package names cho import
- [ ] Kiểm tra `finding/dd/v1/` — FindingServiceClient đã generate
- [ ] Kiểm tra `auth/v1/` — AuthServiceClient đã generate
- [ ] Liệt kê proto packages còn thiếu (cần generate thêm)

---

## T-00.4: Kiểm tra Module Names

```bash
# Ghi lại module name của từng service
grep "^module" /Users/binhnt/Lab/sec/cve/osv.dev/services/*/go.mod
grep "^module" /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/*/go.mod
```

**Expected output** (để dùng trong go.mod của DefectDojo):
```
auth-service:         module github.com/osv/auth-service
finding-service:      module github.com/defectdojo/finding-service
product-service:      module github.com/defectdojo/product-service
scan-service:         module github.com/defectdojo/scan-service
...
shared/pkg:           module github.com/osv/shared/pkg
shared/proto:         module github.com/osv/shared/proto (check go.mod)
```

- [ ] Ghi chép đầy đủ module names vào file `TASK-00-audit-results.md`

---

## T-00.5: Dependency Conflict Check

```bash
# So sánh versions của shared deps
for svc in auth-service finding-service product-service scan-service; do
  echo "=== $svc ==="
  grep -E "google.golang.org/grpc|jackc/pgx|nats-io/nats" \
    /Users/binhnt/Lab/sec/cve/osv.dev/services/$svc/go.mod
done
```

- [ ] Xác nhận gRPC version nhất quán
- [ ] Xác nhận pgx version nhất quán
- [ ] Xác nhận NATS version nhất quán
- [ ] Ghi chép conflicts (nếu có)

---

## T-00.6: Kiểm tra Migration Files

```bash
for svc in auth-service finding-service product-service scan-service \
           notification-service report-service integration-service \
           vulnerability-service; do
  echo "=== $svc ==="
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/$svc/migrations/ 2>/dev/null || echo "NO MIGRATIONS"
done
```

- [ ] Xác nhận migration format (goose, atlas, golang-migrate, raw SQL)
- [ ] Xác nhận không có schema conflicts giữa các services
- [ ] Liệt kê migration tool cần cài đặt

---

## Output: TASK-00-audit-results.md

Tạo file kết quả audit:

```markdown
# Audit Results

## Module Names
| Service | Module Name |
|---|---|
| auth-service | github.com/osv/auth-service |
| finding-service | github.com/defectdojo/finding-service |
...

## Proto Generated Packages
- gen/go/auth/v1 → authv1
- gen/go/finding/dd/v1 → findingv1
...

## Missing Protos (cần generate)
- product/dd/v1/product.proto
- notification/v1/notification.proto
...

## Migration Tool
- Tool: [goose|atlas|golang-migrate]
- Format: [SQL|Go]

## Known Issues
- [Liệt kê các vấn đề phát hiện]
```

---

## Definition of Done

- [ ] Tất cả module names đã được ghi chép
- [ ] Tất cả exported packages đã được liệt kê
- [ ] Proto generated code đã được xác nhận
- [ ] Migration format đã được xác nhận
- [ ] File `TASK-00-audit-results.md` đã tạo
- [ ] Không có showstopper issues
