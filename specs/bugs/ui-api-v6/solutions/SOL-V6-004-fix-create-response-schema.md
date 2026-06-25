# SOL-V6-004: Fix Response Schema — Scan Create & Webhook Create Missing `id`

**Bugs:** BUG-V6-022, BUG-V6-023  
**Task:** TASK-V6-004  
**Services:** `scan-service` (:8084), `notification-service` (:8087)  
**Kiến trúc tham chiếu:** `01-architecture.md §3.6, §3.8`, `02-technical-design.md §6, §8`, `§14.2`

---

## Root Cause Analysis

Cả hai endpoints đều trả 201 (Created) nhưng response body thiếu field `id`. Điều này vi phạm REST convention và `02-technical-design.md §13.3`:

```go
// Contract test trong spec:
assert.NotEmpty(t, body["id"])  // PHẢI có id trong response
```

**Nguyên nhân phổ biến:**
1. Handler serialize struct không có `json:"id"` tag
2. Handler trả về partial object (chỉ có message, không có full entity)
3. Repository `Create()` không trả về created entity với generated UUID

---

## BUG-V6-022: Fix `POST /api/v1/scans` response schema

**Service:** `scan-service` (:8084)  
**Reference:** `02-technical-design.md §14.2` — Scan entity & state machine

### Domain Entity phải có ID

```go
// services/scan-service/internal/domain/entity/scan.go

type Scan struct {
    ID          uuid.UUID   `db:"id"   json:"id"`           // ← PHẢI có json tag
    Name        string      `db:"name" json:"name"`
    Type        string      `db:"type" json:"type"`
    Status      ScanStatus  `db:"status" json:"status"`
    Targets     []string    `db:"targets" json:"targets"`    // pq.StringArray hoặc JSONB
    CreatedBy   uuid.UUID   `db:"created_by" json:"created_by"`
    CreatedAt   time.Time   `db:"created_at" json:"created_at"`
    UpdatedAt   time.Time   `db:"updated_at" json:"updated_at"`
    // Optional fields
    StartedAt   *time.Time  `db:"started_at" json:"started_at,omitempty"`
    CompletedAt *time.Time  `db:"completed_at" json:"completed_at,omitempty"`
    Progress    int         `db:"progress" json:"progress"`   // 0-100
}
```

### Use Case phải trả về full entity

```go
// services/scan-service/internal/usecase/create_scan.go

type CreateScanInput struct {
    Name    string   `json:"name"    validate:"required,min=1,max=255"`
    Type    string   `json:"type"    validate:"required,oneof=nmap_discovery nmap_full zap_baseline zap_full"`
    Targets []string `json:"targets" validate:"required,min=1,dive,required"`
}

func (uc *CreateScanUseCase) Execute(ctx context.Context, in CreateScanInput, userID uuid.UUID) (*entity.Scan, error) {
    scan := &entity.Scan{
        ID:        uuid.New(),       // Generate UUID ở đây
        Name:      in.Name,
        Type:      in.Type,
        Status:    entity.ScanStatusPending,
        Targets:   in.Targets,
        CreatedBy: userID,
        CreatedAt: time.Now().UTC(),
        UpdatedAt: time.Now().UTC(),
        Progress:  0,
    }

    if err := uc.repo.Create(ctx, scan); err != nil {
        return nil, fmt.Errorf("create scan: %w", err)
    }

    // Publish event để trigger actual scan execution
    uc.nats.PublishJSON("scan.scan.created", map[string]interface{}{
        "scan_id":   scan.ID.String(),
        "type":      scan.Type,
        "targets":   scan.Targets,
        "created_by": scan.CreatedBy.String(),
    })

    return scan, nil  // Trả về FULL entity, không phải partial
}
```

### Handler phải serialize full entity

```go
// services/scan-service/internal/delivery/http/scan_handler.go

// POST /scans → 201 với full Scan object
func (h *ScanHandler) CreateScan(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)

    var input usecase.CreateScanInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := h.validator.Struct(input); err != nil {
        writeValidationError(w, err)
        return
    }

    scan, err := h.createScanUC.Execute(r.Context(), input, userID)
    if err != nil {
        if errors.Is(err, domain.ErrInvalidInput) {
            writeError(w, http.StatusBadRequest, err.Error())
            return
        }
        h.log.Error().Err(err).Msg("create scan failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    // 201 Created với Location header + full body
    w.Header().Set("Location", fmt.Sprintf("/api/v1/scans/%s", scan.ID))
    writeJSON(w, http.StatusCreated, scan)  // ← serialize FULL scan entity
}
```

### Expected Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Network Discovery Scan",
  "type": "nmap_discovery",
  "status": "pending",
  "targets": ["192.168.1.0/24"],
  "created_by": "user-uuid",
  "created_at": "2026-06-24T09:00:00Z",
  "updated_at": "2026-06-24T09:00:00Z",
  "progress": 0
}
```

---

## BUG-V6-023: Fix `POST /api/v1/webhooks` response schema

**Service:** `notification-service` (:8087)  
**Reference:** `01-architecture.md §3.8`, `02-technical-design.md §8`

### Domain Entity

```go
// services/notification-service/internal/domain/entity/webhook.go

type Webhook struct {
    ID        uuid.UUID  `db:"id"         json:"id"`       // ← PHẢI có
    Name      string     `db:"name"       json:"name"`
    URL       string     `db:"url"        json:"url"`
    Events    []string   `db:"events"     json:"events"`   // pq.StringArray
    Secret    string     `db:"secret"     json:"secret"`   // raw secret — chỉ trả về khi tạo
    Active    bool       `db:"active"     json:"active"`
    CreatedBy uuid.UUID  `db:"created_by" json:"created_by"`
    CreatedAt time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// WebhookCreateResponse — chỉ dùng cho CREATE response
// Secret hiển thị 1 lần duy nhất
type WebhookCreateResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    URL       string    `json:"url"`
    Events    []string  `json:"events"`
    Secret    string    `json:"secret"`     // Raw secret — sau này không xem lại được
    Active    bool      `json:"active"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Use Case

```go
// services/notification-service/internal/usecase/create_webhook.go

type CreateWebhookInput struct {
    Name   string   `json:"name"   validate:"required,min=1,max=255"`
    URL    string   `json:"url"    validate:"required,url"`
    Events []string `json:"events" validate:"required,min=1,dive,required"`
    Secret string   `json:"secret,omitempty"`  // User-provided hoặc auto-generate
}

func (uc *CreateWebhookUseCase) Execute(ctx context.Context, in CreateWebhookInput, userID uuid.UUID) (*entity.Webhook, error) {
    // SSRF protection (kiến trúc yêu cầu: 02-technical-design.md §8.2)
    if err := validateWebhookURL(in.URL); err != nil {
        return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err)
    }

    // Auto-generate secret nếu không được cung cấp
    secret := in.Secret
    if secret == "" {
        rawBytes := make([]byte, 32)
        rand.Read(rawBytes)
        secret = "whsec_" + hex.EncodeToString(rawBytes)
    }

    webhook := &entity.Webhook{
        ID:        uuid.New(),      // ← Generate UUID
        Name:      in.Name,
        URL:       in.URL,
        Events:    in.Events,
        Secret:    secret,          // Lưu raw secret (hoặc hash tùy thiết kế)
        Active:    true,
        CreatedBy: userID,
        CreatedAt: time.Now().UTC(),
        UpdatedAt: time.Now().UTC(),
    }

    if err := uc.repo.Create(ctx, webhook); err != nil {
        return nil, fmt.Errorf("create webhook: %w", err)
    }

    return webhook, nil  // Full entity
}
```

### Handler

```go
// POST /webhooks → 201 với full Webhook response
func (h *WebhookHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)

    var input usecase.CreateWebhookInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    webhook, err := h.createWebhookUC.Execute(r.Context(), input, userID)
    if err != nil {
        switch {
        case errors.Is(err, domain.ErrInvalidInput):
            writeError(w, http.StatusBadRequest, err.Error())
        default:
            h.log.Error().Err(err).Msg("create webhook failed")
            writeError(w, http.StatusInternalServerError, "internal error")
        }
        return
    }

    w.Header().Set("Location", fmt.Sprintf("/api/v1/webhooks/%s", webhook.ID))
    writeJSON(w, http.StatusCreated, entity.WebhookCreateResponse{
        ID:        webhook.ID.String(),
        Name:      webhook.Name,
        URL:       webhook.URL,
        Events:    webhook.Events,
        Secret:    webhook.Secret,   // Trả về raw secret 1 lần duy nhất
        Active:    webhook.Active,
        CreatedAt: webhook.CreatedAt,
    })
}
```

### Expected Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "name": "Security Alerts Webhook",
  "url": "https://example.com/webhook",
  "events": ["finding.created", "scan.completed"],
  "secret": "whsec_a3b4c5d6...",
  "active": true,
  "created_at": "2026-06-24T09:00:00Z"
}
```

---

## Verification

```bash
# BUG-V6-022: POST /scans → check id field
RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Scan","type":"nmap_discovery","targets":["10.0.0.1"]}' \
  https://c12.openledger.vn/api/v1/scans)
echo $RESP | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'id' in d, 'FAIL: no id'; print('PASS:', d['id'])"

# BUG-V6-023: POST /webhooks → check id field
RESP=$(curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Hook","url":"https://example.com/hook","events":["finding.created"]}' \
  https://c12.openledger.vn/api/v1/webhooks)
echo $RESP | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'id' in d, 'FAIL: no id'; print('PASS:', d['id'])"
```
