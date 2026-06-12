> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T08 — Finding Service Wiring (NATS Subscriber + Routes)

## Thông tin
| | |
|---|---|
| **Phase** | 3 — Findings & CVE |
| **Ước tính** | 4–5 giờ |
| **Depends on** | T05 (NATS scan.completed event) |
| **Blocks** | T09, T10, T12 |

## Mục tiêu
Wire-up `finding-service`: khởi tạo usecases, tạo NATS subscriber lắng nghe `scan.completed`, mount finding HTTP routes. Findings tự động được tạo sau mỗi scan.

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `finding-service/internal/usecase/finding/use_cases.go` | `BatchCreateFindingsUseCase`, `StatusTransitionUseCase`, `BatchUpdateSLADatesUseCase` |
| `finding-service/internal/usecase/audit/` | `AuditRecorder` |
| `finding-service/internal/usecase/sla/` | `SLAComputer` |
| `finding-service/internal/domain/finding/entity.go` | Finding entity |
| `finding-service/internal/domain/finding/state_machine.go` | State machine |
| `finding-service/internal/infra/postgres/` | Finding repository |
| `finding-service/internal/infra/messaging/` | NATS subscriber (nếu có) |

> ⚠️ **Module path**: Finding-service dùng `github.com/defectdojo/finding-service` — xác minh lại trong `finding-service/go.mod`

---

## Các bước thực hiện

### 8.1 Đọc finding-service APIs

```bash
# Xác minh module path
cat osv.dev/services/finding-service/go.mod | head -5

# Đọc repository interface
cat osv.dev/services/finding-service/internal/domain/finding/repository.go

# Đọc infra repository
cat osv.dev/services/finding-service/internal/infra/postgres/*.go

# Đọc NATS subscriber (nếu có)
cat osv.dev/services/finding-service/internal/infra/messaging/*.go
```

### 8.2 Khởi tạo finding repository

```go
import (
    findingrepo "github.com/defectdojo/finding-service/internal/infra/postgres"
)

findingRepo := findingrepo.New(a.db)
```

### 8.3 Khởi tạo finding usecases

```go
import (
    findinguc "github.com/defectdojo/finding-service/internal/usecase/finding"
    audituc   "github.com/defectdojo/finding-service/internal/usecase/audit"
    slauc     "github.com/defectdojo/finding-service/internal/usecase/sla"
)

// NATS publisher
natsPublisher := natsutil.NewPublisher(a.nc, a.log)

// Usecases
batchCreateUC   := findinguc.NewBatchCreate(findingRepo, natsPublisher)
statusTransUC   := findinguc.NewStatusTransition(findingRepo, natsPublisher)
batchSLAUpdateUC := findinguc.NewBatchUpdateSLADates(findingRepo, natsPublisher)
closeOldUC      := findinguc.NewCloseOldFindings(findingRepo, natsPublisher)
```

### 8.4 Tạo NATS subscriber cho `scan.completed`

```go
// internal/app/app.go — thêm vào Start()

// Finding NATS worker
go func() {
    a.nc.Subscribe("scan.completed", func(msg *nats.Msg) {
        var event struct {
            ScanID string `json:"scan_id"`
        }
        json.Unmarshal(msg.Data, &event)

        // Lấy raw findings từ scan
        scan, _ := a.ScanRepo.FindByID(ctx, uuid.MustParse(event.ScanID))

        // Convert scan findings → finding-service input
        inputs := convertScanFindingsToFindingInputs(scan)

        // Batch create findings
        batchCreateUC.Execute(ctx, findinguc.BatchCreateInput{
            ScanID:   uuid.MustParse(event.ScanID),
            Findings: inputs,
        })
    })
}()
```

> **Hàm convert**: Viết `convertScanFindingsToFindingInputs()` để map từ scan-service's `Finding` entity sang finding-service's `FindingInput`. Đây là phần cần viết thủ công (~30 LOC).

### 8.5 Finding HTTP routes

Tìm hiểu finding-service có HTTP delivery không:

```bash
ls osv.dev/services/finding-service/internal/delivery/
# grpc/ — có gRPC handler

# Nếu chỉ có gRPC, cần viết thin HTTP wrapper
```

```go
// internal/router/router.go — finding routes
r.Group(func(r chi.Router) {
    r.Use(authMW.RequireAuth)

    r.Get("/api/v1/findings", func(w http.ResponseWriter, r *http.Request) {
        // Filter params
        severity := r.URL.Query().Get("severity")
        status := r.URL.Query().Get("status")
        scanID := r.URL.Query().Get("scan_id")
        page, _ := strconv.Atoi(r.URL.Query().Get("page"))

        findings, total, err := a.FindingRepo.List(r.Context(), findingRepo.Filter{
            Severity: severity,
            Status: status,
            // ScanID: scanID,
        }, page, 20)
        writeJSON(w, 200, map[string]any{"findings": findings, "total": total})
    })

    r.Get("/api/v1/findings/{id}", func(w http.ResponseWriter, r *http.Request) {
        id, _ := uuid.Parse(chi.URLParam(r, "id"))
        finding, err := a.FindingRepo.FindByID(r.Context(), id)
        if err != nil {
            writeJSON(w, 404, map[string]string{"error": "not_found"})
            return
        }
        writeJSON(w, 200, finding)
    })

    r.Patch("/api/v1/findings/{id}/status", func(w http.ResponseWriter, r *http.Request) {
        id, _ := uuid.Parse(chi.URLParam(r, "id"))
        var req struct{ Status string `json:"status"` }
        json.NewDecoder(r.Body).Decode(&req)

        switch req.Status {
        case "mitigated":
            a.StatusTransUC.Close(r.Context(), id, "user")
        case "false_positive":
            a.StatusTransUC.MarkFalsePositive(r.Context(), id)
        case "risk_accepted":
            a.StatusTransUC.AcceptRisk(r.Context(), id)
        }
        writeJSON(w, 200, map[string]string{"message": "status updated"})
    })

    r.Post("/api/v1/findings/{id}/accept-risk", func(w http.ResponseWriter, r *http.Request) {
        id, _ := uuid.Parse(chi.URLParam(r, "id"))
        a.StatusTransUC.AcceptRisk(r.Context(), id)
        writeJSON(w, 200, map[string]string{"message": "risk accepted"})
    })

    r.Get("/api/v1/findings/summary", func(w http.ResponseWriter, r *http.Request) {
        // Aggregate query: count by severity
        summary, _ := a.FindingRepo.CountBySeverity(r.Context())
        writeJSON(w, 200, summary)
    })
})
```

### 8.6 Cập nhật App struct

```go
type App struct {
    // ... existing fields

    // Finding service
    FindingRepo     finding.Repository
    BatchCreateUC   *findinguc.BatchCreateFindingsUseCase
    StatusTransUC   *findinguc.StatusTransitionUseCase
    BatchSLAUpdateUC *findinguc.BatchUpdateSLADatesUseCase
}
```

---

## Output

- [x] Finding repository khởi tạo ✓ (findingBridge với pgxpool.Pool — direct SQL)
- [x] BatchCreate, StatusTransition, SLAUpdate usecases khởi tạo ✓ (findingBridge methods)
- [x] NATS subscriber `scan.completed` → tự động tạo findings ✓ (finding_runner.go handleScanCompleted)
- [x] Finding HTTP routes mounted ✓ (router.go: HandleListFindings, HandleGetFinding, etc.)
- [x] `convertScanFindingsToFindingInputs()` function ✓ (finding_runner.go)

## Acceptance Criteria

```bash
# Chạy discovery scan, đợi hoàn thành
TOKEN=<token>
SCAN_ID=$(curl -s -X POST .../api/v1/scans -d '{"targets":["127.0.0.1"],"scan_type":"discovery"}' | jq -r .scan_id)
sleep 10

# Findings phải được tạo tự động
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/findings?scan_id=$SCAN_ID"
# → {"findings":[...],"total":N}

# Update finding status
FINDING_ID=$(...)
curl -X PATCH .../api/v1/findings/$FINDING_ID/status \
  -d '{"status":"mitigated"}'
# → {"message":"status updated"}
```
