# TASK-BE-020 — notification-service: Webhook Test Endpoint

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-020 |
| **Service** | `services/notification-service` |
| **Solution Ref** | [SOL-UI-004 §2](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🟡 P1 |
| **Depends On** | — |
| **Estimated** | 2h |

---

## Context

UI Admin > Integrations page có nút "Test Webhook". notification-service cần endpoint để thực sự gửi test delivery đến webhook URL.

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `services/notification-service/internal/adapter/http/webhook_handler.go` |
| MODIFY | `services/notification-service/internal/adapter/http/router.go` |

---

## Implementation

```go
// services/notification-service/internal/adapter/http/webhook_handler.go

// POST /notification-webhooks/{id}/test → alias /webhooks/{id}/test
func (h *WebhookHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
    webhookID, err := uuid.Parse(r.PathValue("id"))
    if err != nil {
        respondError(w, 400, "VALIDATION_ERROR", "Invalid webhook ID")
        return
    }

    wh, err := h.webhookRepo.FindByID(r.Context(), webhookID)
    if err != nil {
        respondError(w, 404, "NOT_FOUND", "Webhook not found")
        return
    }

    // Build test payload
    testPayload := map[string]interface{}{
        "event_type":  "test",
        "timestamp":   time.Now().UTC().Format(time.RFC3339),
        "message":     "This is a test notification from OSV Platform",
        "webhook_id":  webhookID.String(),
        "platform":    "OSV Platform",
    }

    payloadBytes, _ := json.Marshal(testPayload)

    // Compute HMAC signature if secret configured
    var signature string
    if wh.Secret != "" {
        mac := hmac.New(sha256.New, []byte(wh.Secret))
        mac.Write(payloadBytes)
        signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
    }

    // Send test HTTP request
    start := time.Now()
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(payloadBytes))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "OSV-Platform/1.0")
    req.Header.Set("X-OSV-Event", "test")
    if signature != "" {
        req.Header.Set("X-Hub-Signature-256", signature)
    }

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    elapsed := time.Since(start)

    deliveryID := "dlv_test_" + uuid.New().String()[:8]

    if err != nil {
        respondJSON(w, 200, map[string]interface{}{
            "delivery_id":      deliveryID,
            "status":           "failed",
            "error":            err.Error(),
            "response_time_ms": elapsed.Milliseconds(),
        })
        return
    }
    defer resp.Body.Close()

    respondJSON(w, 200, map[string]interface{}{
        "delivery_id":      deliveryID,
        "status":           "success",
        "response_code":    resp.StatusCode,
        "response_time_ms": elapsed.Milliseconds(),
    })
}
```

### Router:

```go
// services/notification-service/internal/adapter/http/router.go
mux.HandleFunc("POST /notification-webhooks/{id}/test", h.Webhook.TestWebhook)
mux.HandleFunc("POST /webhooks/{id}/test",              h.Webhook.TestWebhook) // v1 alias
```

---

## Verification

```bash
cd services/notification-service
go build ./...

# Test with a valid webhook
curl -X POST http://localhost:8087/webhooks/$WEBHOOK_ID/test \
  -H "X-User-Role: admin" | jq .
# Expected: {"delivery_id":"dlv_test_xxx","status":"success","response_code":200,"response_time_ms":N}
```

---

## Checklist

- [x] `TestWebhook` gửi POST đến `wh.URL` với test payload
- [x] Include HMAC signature header nếu webhook có `secret`
- [x] Timeout 10s cho test delivery
- [x] Response trả về `delivery_id`, `status`, `response_code`, `response_time_ms`
- [x] `status: "failed"` nếu HTTP call fail (không phải 5xx response)
- [x] `go build ./...` thành công

---

# TASK-BE-021 — sla-service: SLA Dashboard Aggregate Endpoint

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-021 |
| **Service** | `services/sla-service` |
| **Solution Ref** | [SOL-UI-002 §2.8](../solutions/SOL-UI-002-dashboard-bff-sse.md) |
| **Priority** | 🟡 P1 |
| **Depends On** | — |
| **Estimated** | 3h |

---

## Context

SLA Management page cần `GET /api/v1/dashboard/sla` (proxied từ gateway → finding-service `/internal/sla-dashboard`).

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/finding-service/internal/adapter/http/sla_handler.go` |
| MODIFY | `services/finding-service/internal/adapter/http/router.go` |

---

## Implementation

```go
// services/finding-service/internal/adapter/http/sla_handler.go

type SLAHandler struct {
	findingRepo FindingRepository
}

// GET /internal/sla-dashboard
func (h *SLAHandler) GetSLADashboard(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")
	page, ps   := parsePagination(r)

	// All queries in parallel
	var (
		summary   *SLASummaryData
		trend     []SLATrendPoint
		breached  []SLAFindingItem
		atRisk    []SLAFindingItem
		byProduct []ProductSLAData
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	wg.Add(5)

	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetSLASummary(r.Context(), productID)
		mu.Lock(); summary = d; mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetSLAComplianceTrend(r.Context(), productID, 6)
		mu.Lock(); trend = d; mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetBreachedFindings(r.Context(), productID, page, ps)
		mu.Lock(); breached = d; mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetAtRiskFindings(r.Context(), productID)
		mu.Lock(); atRisk = d; mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetSLAByProduct(r.Context())
		mu.Lock(); byProduct = d; mu.Unlock()
	}()

	wg.Wait()

	respondJSON(w, 200, map[string]interface{}{
		"summary":           summary,
		"compliance_trend":  trend,
		"breached_findings": breached,
		"at_risk_findings":  atRisk,
		"by_product":        byProduct,
		"page":              page,
		"page_size":         ps,
	})
}

// Data types
type SLASummaryData struct {
	CompliancePct float64 `json:"compliance_pct"`
	Breached      int     `json:"breached"`
	AtRisk        int     `json:"at_risk"`
	OnTime        int     `json:"on_time"`
}

type SLATrendPoint struct {
	Month         string  `json:"month"`
	CompliancePct float64 `json:"compliance_pct"`
}

type SLAFindingItem struct {
	FindingID   string `json:"finding_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	ProductName string `json:"product_name"`
	DaysLeft    int    `json:"days_left"`    // negative = overdue
	ExpiresAt   string `json:"expires_at"`
}

type ProductSLAData struct {
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	CompliancePct float64 `json:"compliance_pct"`
	Breached      int     `json:"breached"`
	AtRisk        int     `json:"at_risk"`
}
```

### SQL:

```sql
-- GetSLASummary
SELECT
    ROUND(100.0 * COUNT(*) FILTER (WHERE sla_expiration_date > NOW() OR sla_expiration_date IS NULL)
          / NULLIF(COUNT(*), 0), 2) AS compliance_pct,
    COUNT(*) FILTER (WHERE sla_expiration_date < NOW() AND NOT is_mitigated) AS breached,
    COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND NOT is_mitigated) AS at_risk,
    COUNT(*) FILTER (WHERE sla_expiration_date > NOW() + INTERVAL '7 days' AND NOT is_mitigated) AS on_time
FROM findings
WHERE active = true AND is_duplicate = false
  AND ($1::uuid IS NULL OR product_id = $1::uuid);

-- GetSLAComplianceTrend (last N months)
SELECT
    TO_CHAR(DATE_TRUNC('month', checked_at), 'YYYY-MM') AS month,
    AVG(compliance_pct)::numeric(5,2) AS compliance_pct
FROM sla_snapshots
WHERE checked_at >= NOW() - ($1 || ' months')::interval
GROUP BY month ORDER BY month ASC;

-- GetBreachedFindings (paginated)
SELECT f.id, f.title, f.severity, p.name AS product_name,
       EXTRACT(DAY FROM NOW() - f.sla_expiration_date)::int AS days_left,
       f.sla_expiration_date AS expires_at
FROM findings f JOIN products p ON p.id = f.product_id
WHERE f.active AND NOT f.is_duplicate AND f.sla_expiration_date < NOW()
ORDER BY f.sla_expiration_date ASC
LIMIT $1 OFFSET $2;
```

### Router:

```go
// services/finding-service/internal/adapter/http/router.go
mux.HandleFunc("GET /internal/sla-dashboard", h.SLA.GetSLADashboard)
```

---

## Verification

```bash
cd services/finding-service
go build ./...

curl "http://localhost:8085/internal/sla-dashboard" | jq '.summary'
# Expected: {"compliance_pct":85.5,"breached":3,"at_risk":7,"on_time":42}
```

---

## Checklist

- [x] `GetSLADashboard` returns all 5 data sections in parallel
- [x] `compliance_trend` sorted by `month ASC`, last 6 months
- [x] `breached_findings` paginated
- [x] `by_product` shows per-product SLA compliance
- [x] `go build ./...` thành công
