# TASK-BE-P0-04 — Fix Dashboard SLA 500 + SLA Config Schema

**Phase:** Sprint 1 — P0 Unblock  
**Nguồn giải pháp:** [`solutions/SOL-003_fix-findings-sla-500.md`](../solutions/SOL-003_fix-findings-sla-500.md)  
**Ưu tiên:** 🔴 P0 — Blocking (Dashboard SLA không load)  
**Phụ thuộc:** TASK-BE-P0-03 (finding-service cần chạy)  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

Fix `GET /api/v1/dashboard/sla` trả 500 `"Failed to parse SLA response"`. Fix `GET /api/v1/sla/config` trả schema sai. Fix `GET /api/v1/sla/overview` nếu trả 404.

---

## Root Cause (đã xác định)

Từ [`apps/osv/internal/gateway/bff/dashboard.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/dashboard.go) line 98:

```go
// BFF đang gọi sai endpoint:
err := bff.client.get(ctx, "http://"+bff.findingSvcAddr+"/internal/sla-dashboard", &result)
// → finding-service:8085/internal/sla-dashboard — endpoint này KHÔNG TỒN TẠI
```

---

## Files cần sửa

### Fix 1 — [MODIFY] `apps/osv/internal/gateway/bff/dashboard.go`

**File**: [`apps/osv/internal/gateway/bff/dashboard.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/dashboard.go)

Thay đổi `HandleDashboardSLA` để gọi đúng endpoint:

```go
// Line 90-108 — THAY THẾ toàn bộ hàm HandleDashboardSLA:

// HandleDashboardSLA — GET /api/v1/dashboard/sla
func (bff *DashboardBFF) HandleDashboardSLA(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    // Option A: Gọi sla-service trực tiếp
    // Cần thêm slaSvcAddr vào DashboardBFF struct
    var result map[string]interface{}
    err := bff.client.get(ctx, "http://sla-service:8086/api/v2/sla-dashboard", &result)
    if err != nil {
        log.Error().Err(err).Msg("failed to fetch SLA dashboard from sla-service")
        // Graceful degradation: trả empty state thay vì 500/503
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "summary": map[string]interface{}{
                "total_active_findings": 0,
                "compliance_percent":    100.0,
                "breached":              0,
                "at_risk":               0,
                "ok":                    0,
            },
            "compliance_trend":  []interface{}{},
            "breached_findings": []interface{}{},
            "at_risk_findings":  []interface{}{},
            "by_product":        []interface{}{},
            "total_breached":    0,
            "total_at_risk":     0,
            "page":              1,
            "page_size":         20,
        })
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

**Lưu ý**: Sử dụng hardcode `sla-service:8086` (Docker network hostname) theo đúng service map trong `01-architecture.md §2.2`.

### Fix 2 — [FIND & MODIFY] `services/sla-service` — thêm `/api/v2/sla-dashboard` endpoint

```bash
# Tìm router của sla-service
grep -r "chi.NewRouter\|r\.Get\|sla-dashboard\|/api/v2/sla" \
  services/sla-service/ --include="*.go" -l
```

Thêm hoặc verify endpoint `/api/v2/sla-dashboard`:

```go
// services/sla-service/internal/delivery/http/sla_handler.go
// Thêm handler:

// GET /api/v2/sla-dashboard
func (h *SLAHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Lấy SLA stats từ repo (finding.sla_expiry, sla_breached fields)
    breached, _ := h.repo.CountBreached(ctx)
    atRisk, _   := h.repo.CountAtRisk(ctx)     // sla_expiry trong 7 ngày tới
    total, _    := h.repo.CountActive(ctx)
    ok := total - breached - atRisk
    if ok < 0 { ok = 0 }

    var compliance float64
    if total > 0 {
        compliance = float64(ok) / float64(total) * 100.0
    } else {
        compliance = 100.0
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "summary": map[string]interface{}{
            "total_active_findings": total,
            "compliance_percent":    compliance,
            "breached":              breached,
            "at_risk":               atRisk,
            "ok":                    ok,
        },
        "compliance_trend":  []interface{}{},
        "breached_findings": []interface{}{},
        "at_risk_findings":  []interface{}{},
        "by_product":        []interface{}{},
        "total_breached":    breached,
        "total_at_risk":     atRisk,
        "page":              1,
        "page_size":         20,
    })
}
```

Register trong router:
```go
r.Get("/api/v2/sla-dashboard", h.GetDashboard) // THÊM MỚI
```

### Fix 3 — [FIND & MODIFY] SLA Config schema

```bash
# Tìm sla config handler
grep -r "GetConfig\|/api/v1/sla/config\|SLAConfig" \
  services/sla-service/ --include="*.go" -l
```

Fix response format:
```go
// GET /api/v1/sla/config — phải trả { "global": {...}, "product_overrides": [] }
func (h *SLAHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    global, err := h.repo.GetGlobalConfig(r.Context())
    if err != nil {
        // Default values nếu chưa có config
        global = &SLAConfig{
            CriticalDays: 7,
            HighDays:     30,
            MediumDays:   90,
            LowDays:      180,
        }
    }

    overrides, _ := h.repo.GetProductOverrides(r.Context())
    if overrides == nil {
        overrides = []interface{}{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "global": map[string]interface{}{
            "critical_days": global.CriticalDays,
            "high_days":     global.HighDays,
            "medium_days":   global.MediumDays,
            "low_days":      global.LowDays,
        },
        "product_overrides": overrides,
    })
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/dashboard/sla` trả HTTP 200 (không còn 500)
- [ ] Response có `summary.compliance_percent`, `breached_findings`, `at_risk_findings`
- [ ] Khi sla-service down, dashboard/sla vẫn trả 200 với empty data (graceful degradation)
- [ ] `GET /api/v1/sla/config` trả HTTP 200 với `{ "global": {...}, "product_overrides": [] }`
- [ ] `GET /api/v1/sla/overview` trả HTTP 200 (đã có route, cần sla-service endpoint)

## Verification

```bash
# Dashboard SLA
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/dashboard/sla
# Expected: HTTP 200 { "summary": { "compliance_percent": 100.0, ... } }

# SLA Config
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/config
# Expected: HTTP 200 { "global": { "critical_days": 7, ... }, "product_overrides": [] }

# SLA Overview
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/overview
# Expected: HTTP 200
```
