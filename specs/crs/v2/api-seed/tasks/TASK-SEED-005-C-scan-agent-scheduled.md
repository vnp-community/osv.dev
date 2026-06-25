# TASK-SEED-005-C: Scan-Service — Agent Routes + Scheduled Scan Gateway Exposure

> **Solution:** [SOL-SEED-005](../solutions/SOL-SEED-005-assets-scan.md)  
> **Service:** `services/scan-service` + `apps/osv`  
> **Depends on:** TASK-SEED-001-D (gateway auth hoạt động)  
> **Blocking:** Không có  
> **Status:** ✅ COMPLETED — 2026-06-19  
> **Files tạo/sửa:**  
> - `services/scan-service/internal/delivery/http/agent_handler.go` (đã có `RegisterAgent`, `ListAgents`, `GetAgent`, `SubmitReport`)  
> - `services/scan-service/internal/delivery/http/schedule/schedule_handler.go` (đã có `CreateSchedule`, `ListSchedules`, `GetSchedule`, `UpdateSchedule`, `DeleteSchedule`)  
> - `services/scan-service/internal/delivery/http/router.go` (thêm `schedulehttp.ScheduleHandler` parameter, mount `/api/v1/scans/scheduled` CRUD)  
> - `apps/osv/internal/gateway/router.go` (thêm SEED-005-C routes: agents CRUD + scans/scheduled POST/PUT/DELETE)

## Mục tiêu

(1) Thêm agent registration handlers vào scan-service (nếu chưa có HTTP endpoints).  
(2) Expose scheduled scan routes qua gateway (hiện tại chưa được route).

## Bước 1: Khảo sát scan-service

```bash
# Xem toàn bộ delivery layer
find /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/delivery \
  -name "*.go" | head -20

# Kiểm tra agent handler hiện có
find /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service \
  -name "*.go" | xargs grep -l "RegisterAgent\|agent.*handler\|POST.*agents" 2>/dev/null | head -10

# Kiểm tra scheduled scan routes
find /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service \
  -name "*.go" | xargs grep -l "schedule\|Schedule\|cron" 2>/dev/null | head -10

# Xem router file
find /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service \
  -name "router.go" | head -5
```

## Bước 2: Thêm Agent handlers (nếu chưa có)

Nếu agent report endpoint `/api/v1/agents/report` đã có (theo architecture §3.6) nhưng thiếu `RegisterAgent`, `ListAgents`, `GetAgent`:

**File:** `internal/delivery/http/agent_handler.go` (NEW hoặc thêm vào existing handler)

```go
// AgentHandler xử lý agent registration và report submission
type AgentHandler struct {
    agentUC agentUseCase
    log     zerolog.Logger
}

// RegisterAgent handles POST /api/v1/agents (Admin only — enforce at gateway)
func (h *AgentHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name      string   `json:"name"`
        Hostname  string   `json:"hostname"`
        IPAddress string   `json:"ip_address"`
        OS        string   `json:"os"`
        Tags      []string `json:"tags"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, 400, errResp("invalid_body", err.Error()))
        return
    }
    
    agent, apiKeyPlaintext, err := h.agentUC.Register(r.Context(), req)
    if err != nil {
        writeJSON(w, 500, errResp("internal", err.Error()))
        return
    }
    
    // api_key là one-time plaintext — không lưu lại sau response này
    writeJSON(w, 201, map[string]any{
        "id":         agent.ID,
        "name":       agent.Name,
        "hostname":   agent.Hostname,
        "ip_address": agent.IPAddress,
        "os":         agent.OS,
        "tags":       agent.Tags,
        "api_key":    apiKeyPlaintext, // ONE-TIME ONLY
        "status":     "inactive",
        "created_at": agent.CreatedAt,
    })
}

// ListAgents handles GET /api/v1/agents
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
    agents, err := h.agentUC.List(r.Context())
    if err != nil {
        writeJSON(w, 500, errResp("internal", err.Error()))
        return
    }
    writeJSON(w, 200, map[string]any{"agents": agents, "count": len(agents)})
}

// GetAgent handles GET /api/v1/agents/{id}
func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
    // parse {id} → agentUC.GetByID → 200 | 404
}

// SubmitReport handles POST /api/v1/agents/{id}/reports (admin tạo agent report để seed)
// Requires API key của agent hoặc scope scan:execute
func (h *AgentHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
    agentID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        writeJSON(w, 400, errResp("invalid_id", "invalid agent UUID"))
        return
    }
    
    var report AgentReport
    if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
        writeJSON(w, 400, errResp("invalid_body", err.Error()))
        return
    }
    
    result, err := h.agentUC.SubmitReport(r.Context(), agentID, report)
    if err != nil {
        writeJSON(w, 500, errResp("internal", err.Error()))
        return
    }
    
    // 202 Accepted — report queued for processing
    writeJSON(w, 202, map[string]any{
        "report_id":    result.ID,
        "agent_id":     agentID,
        "package_count": len(report.Packages),
        "status":       "queued_for_processing",
    })
}
```

## Bước 3: Đăng ký agent routes

**File:** `internal/delivery/http/router.go` (scan-service)

```go
// SEED-005: Agent routes
// QUAN TRỌNG: literal paths TRƯỚC wildcards
r.Post("/api/v1/agents",              agentH.RegisterAgent)
r.Get("/api/v1/agents",               agentH.ListAgents)
r.Get("/api/v1/agents/{id}",          agentH.GetAgent)
r.Post("/api/v1/agents/{id}/reports", agentH.SubmitReport)
```

## Bước 4: Kiểm tra Scheduled Scan handlers

```bash
# Xem scheduled scan endpoints trong scan-service
grep -rn "schedule\|Schedule\|cron_expr\|POST.*schedule" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/delivery \
  --include="*.go" | head -20
```

Nếu routes đã có ở dạng `/schedules/*`, cần biết path chính xác để gateway rewrite.

## Bước 5: Gateway routes — Agents + Scheduled Scans

**File:** `apps/osv/internal/gateway/router.go`

```bash
# Kiểm tra scan-service routes hiện tại trong gateway
grep -n "scan-service\|/api/v1/scans" \
  /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go | head -20

# Xem cách proxy.Forward được implement (để biết có hỗ trợ path rewrite không)
grep -rn "ForwardWithPath\|PathRewrite\|StripPrefix" \
  /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/ --include="*.go" | head -10
```

**Thêm routes:**

```go
// SEED-005: Agent management
mux.Handle("POST /api/v1/agents",
    adminOnly(proxy.Forward("scan-service:8084")))
mux.Handle("GET /api/v1/agents",
    protected(proxy.Forward("scan-service:8084")))
mux.Handle("GET /api/v1/agents/{id}",
    protected(proxy.Forward("scan-service:8084")))
mux.Handle("POST /api/v1/agents/{id}/reports",
    protected(proxy.Forward("scan-service:8084")))  // agent API key hoặc scan:execute scope

// SEED-005: Scheduled Scans
// Nếu scan-service dùng /schedules/* path, cần path rewrite:
mux.Handle("POST /api/v1/scans/scheduled",
    protected(proxy.ForwardWithRewrite("scan-service:8084", "/api/v1/scans/scheduled", "/schedules")))
mux.Handle("GET /api/v1/scans/scheduled",
    protected(proxy.ForwardWithRewrite("scan-service:8084", "/api/v1/scans/scheduled", "/schedules")))
mux.Handle("GET /api/v1/scans/scheduled/{id}",
    protected(proxy.ForwardWithRewrite("scan-service:8084", "/api/v1/scans/scheduled/", "/schedules/")))
mux.Handle("PUT /api/v1/scans/scheduled/{id}",
    protected(proxy.ForwardWithRewrite("scan-service:8084", "/api/v1/scans/scheduled/", "/schedules/")))
mux.Handle("DELETE /api/v1/scans/scheduled/{id}",
    protected(proxy.ForwardWithRewrite("scan-service:8084", "/api/v1/scans/scheduled/", "/schedules/")))
```

> **Lưu ý về path rewrite**: Nếu `proxy.ForwardWithRewrite` chưa tồn tại, implement hoặc dùng `http.StripPrefix` + reverse proxy. Đọc `apps/osv/internal/gateway/proxy.go` để biết cách hiện tại.

## Bước 6: Verify path rewrite

```bash
# Xem proxy.go để biết pattern
cat /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/proxy.go
```

Nếu scan-service đã serve scheduled scans ở `/api/v1/scans/scheduled/*` (không cần rewrite), chỉ cần forward thẳng:

```go
mux.Handle("POST /api/v1/scans/scheduled",
    protected(proxy.Forward("scan-service:8084")))
// v.v...
```

## Acceptance Criteria

- [x] `POST /api/v1/agents` (Admin) → `201` với `api_key` plaintext
- [x] `GET /api/v1/agents` → `200` với danh sách agents
- [x] `POST /api/v1/agents/{id}/reports` với 50 packages → `202`
- [x] `GET /api/v1/scans/scheduled` qua gateway → KHÔNG trả `404`
- [x] `POST /api/v1/scans/scheduled` → `201` với `cron_expr` được lưu
- [x] `go build ./...` scan-service thành công
