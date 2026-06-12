> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T07 — Agent Routes (Download + Report Submission)

## Thông tin
| | |
|---|---|
| **Phase** | 2 — Scan Core |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T05 |
| **Blocks** | T11 |

## Mục tiêu
Triển khai 2 endpoints agent: `GET /agent/download` (trả về Python script) và `POST /agent/report` (nhận package report từ agent). Sử dụng `scan-service/usecase/agent/submit_report`.

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `scan-service/internal/usecase/agent/submit_report/` | SubmitReportUseCase |
| `scan-service/internal/adapters/repository/postgres/` | AgentReport repo (nếu có) |

---

## Các bước thực hiện

### 7.1 Đọc agent usecase API

```bash
cat osv.dev/services/scan-service/internal/usecase/agent/submit_report/*.go
```

Ghi lại:
- Input struct (hostname, os_info, packages list)
- Output struct
- Dependencies cần inject

### 7.2 Khởi tạo agent usecase

```go
import (
    agentuc "github.com/osv/scan-service/internal/usecase/agent/submit_report"
)

agentUC := agentuc.New(
    agentRepo,  // agent report repository
    a.nc,       // NATS để publish "agent.report.submitted"
    a.log,
)
```

### 7.3 `GET /agent/download` — Render Python agent script

Đây là route render Python script với server URL được inject:

```go
// router.go
r.Get("/agent/download", func(w http.ResponseWriter, r *http.Request) {
    // Lấy server base URL
    baseURL := r.URL.Scheme + "://" + r.Host
    if baseURL == "://" {
        baseURL = "http://localhost:8080" // fallback
    }

    // Template Python script (tương tự Python gốc trong app.py)
    agentScript := generateAgentScript(baseURL + "/agent/report")

    w.Header().Set("Content-Type", "text/x-python")
    w.Header().Set("Content-Disposition", "attachment; filename=agent.py")
    w.Write([]byte(agentScript))
})

func generateAgentScript(reportURL string) string {
    return fmt.Sprintf(`#!/usr/bin/env python3
"""OpenVulnScan Agent - collects installed packages and reports vulnerabilities"""
import subprocess, json, requests, socket, platform

OPENVULNSCAN_API = %q
OSV_API = "https://api.osv.dev/v1/query"

# ... (copy Python script từ OpenVulnScan/app.py lines 293-429)
`, reportURL)
}
```

> **Lưu ý**: Copy nội dung Python script từ `OpenVulnScan/app.py` (lines 293–429), chỉ thay phần URL interpolation.

### 7.4 `POST /agent/report` — Nhận report

```go
r.Post("/agent/report", func(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Hostname string `json:"hostname"`
        OSInfo   string `json:"os_info"`
        Packages []struct {
            Name    string `json:"name"`
            Version string `json:"version"`
        } `json:"packages"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, 400, map[string]string{"error": "invalid_request"})
        return
    }

    // Gọi agent usecase để lưu report và enqueue enrichment
    result, err := a.AgentUC.Execute(r.Context(), agentuc.Input{
        Hostname: req.Hostname,
        OSInfo:   req.OSInfo,
        Packages: req.Packages, // Adjust to match Input struct
    })
    if err != nil {
        writeJSON(w, 500, map[string]string{"error": err.Error()})
        return
    }

    writeJSON(w, 200, map[string]interface{}{
        "report_id": result.ReportID,
        "message":   "report received, processing",
    })
})
```

### 7.5 Agent report list endpoints

```go
// Protected routes
r.Get("/api/v1/agent/reports", func(w http.ResponseWriter, r *http.Request) {
    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    reports, total, err := a.AgentRepo.List(r.Context(), page, 20)
    writeJSON(w, 200, map[string]any{"reports": reports, "total": total})
})

r.Get("/api/v1/agent/reports/{id}", func(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.Atoi(chi.URLParam(r, "id"))
    report, err := a.AgentRepo.GetByID(r.Context(), id)
    if err != nil {
        writeJSON(w, 404, map[string]string{"error": "not_found"})
        return
    }
    writeJSON(w, 200, report)
})
```

### 7.6 NATS publish agent event

Agent report usecase cần publish NATS event để ingestion pipeline xử lý:

```go
// Trong agentuc.Execute() — hoặc wrap trong route handler:
a.nc.Publish(ctx, "agent.report.submitted", AgentReportEvent{
    ReportID: result.ReportID,
    Hostname: req.Hostname,
})
```

---

## Output

- [x] `GET /agent/download` → Python script với URL đúng ✓ (HandleAgentDownload)
- [x] `POST /agent/report` → lưu report, trả về 200 ✓ (HandleAgentReport → NATS publish)
- [x] NATS event `agent.report.submitted` được publish ✓ (nc.Publish in HandleAgentReport)
- [x] `GET /api/v1/agent/reports` → list reports ✓ (HandleListAgentReports)
- [x] `GET /api/v1/agent/reports/{id}` → detail ✓ (HandleGetAgentReport)

## Acceptance Criteria

```bash
# Download agent script
curl -O http://localhost:8080/agent/download
# → file agent.py được download (~100 lines Python)

# Submit agent report thủ công
curl -X POST http://localhost:8080/agent/report \
  -H "Content-Type: application/json" \
  -d '{
    "hostname":"test-host",
    "os_info":"Ubuntu 22.04",
    "packages":[
      {"name":"openssl","version":"1.1.1"},
      {"name":"curl","version":"7.68.0"}
    ]
  }'
# → {"report_id":1,"message":"report received, processing"}

# Run actual agent (test end-to-end)
python3 agent.py
# → Agent gửi packages từ host thực
```
