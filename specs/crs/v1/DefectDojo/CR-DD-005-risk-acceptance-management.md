# ✅ COMPLETED — CR-DD-005 — Risk Acceptance Management

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-005 |
| **Tiêu đề** | Risk Acceptance — Simple & Full Risk Acceptance với Expiry & Auto-Reactivation |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/03-product-management-service.md §risk_acceptance`, `SRS.md §FR-DM-03` |
| **Target Service** | `product-service` (phần của CR-DD-001) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | Feature Addition |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

Risk Acceptance cho phép security teams chấp nhận một số findings có rủi ro nhưng không fix được ngay (vd: phụ thuộc vendor fix, accepted business risk). DefectDojo hỗ trợ **2 modes**:

1. **Simple Risk Acceptance** — Click nút "Accept Risk" trực tiếp trên finding, không cần approval workflow
2. **Full Risk Acceptance** — Tạo Risk Acceptance document với ngày hết hạn, người chịu trách nhiệm, file đính kèm

OSV hiện tại **không có** Risk Acceptance concept.

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo |
|---------|-----|-----------|
| Simple risk acceptance | ❌ | ✅ Per-finding toggle |
| Full risk acceptance | ❌ | ✅ Document với expiry |
| Risk acceptance expiry | ❌ | ✅ Scheduled check + notification |
| Auto-reactivation on expiry | ❌ | ✅ `reactivate_expired=true` |
| File proof attachment | ❌ | ✅ PDF/document upload |
| Finding linking (M2M) | ❌ | ✅ Multiple findings per RA |
| Expiry notification | ❌ | ✅ NATS event → Notification |

---

## 3. Domain Design

### 3.1 Risk Acceptance Entity

```go
// product-service/internal/domain/risk_acceptance/entity.go
// Mirrors Python: dojo/models.py::Risk_Acceptance

type RiskAcceptance struct {
    ID          string
    Name        string   // human-readable name
    ProductID   string   // Which product this RA belongs to

    // Responsible parties
    AcceptedByID string  // User who accepted
    OwnerID      *string // Optional additional owner

    // Expiry
    ExpirationDate *time.Time

    // Documentation
    Notes           string
    ProofFileID     *string  // Minio/S3 key for proof document

    // Behavior on expiry
    ReactivateExpired     bool    // Auto-reactivate findings when expired
    ReactivateNoteText    string  // Note to add when reactivating
    RestartSLAOnReactivation bool // Restart SLA countdown when findings reactivated

    // State
    IsExpired bool

    // Associated findings (stored as array for simplicity)
    FindingIDs []string  // Findings covered by this risk acceptance

    CreatedAt time.Time
    UpdatedAt time.Time
}

// RiskAcceptanceStatus computed from IsExpired + ExpirationDate
func (ra *RiskAcceptance) Status() RiskAcceptanceStatus {
    if ra.IsExpired {
        return RiskAcceptanceStatusExpired
    }
    if ra.ExpirationDate != nil && time.Now().After(*ra.ExpirationDate) {
        return RiskAcceptanceStatusExpired
    }
    return RiskAcceptanceStatusActive
}

type RiskAcceptanceStatus string
const (
    RiskAcceptanceStatusActive  RiskAcceptanceStatus = "active"
    RiskAcceptanceStatusExpired RiskAcceptanceStatus = "expired"
)
```

### 3.2 Use Cases

```go
// usecase/risk_acceptance/create.go
// Mirrors Python: dojo/risk_acceptance/helper.py::add_findings_to_risk_acceptance()

type CreateRiskAcceptanceInput struct {
    Name            string
    ProductID       string
    FindingIDs      []string    // Findings to accept
    ExpirationDate  *time.Time
    Notes           string
    ReactivateExpired bool
    ReactivateNoteText string
    RestartSLAOnReactivation bool
    RequestorUserID string
}

func (uc *CreateRiskAcceptanceUseCase) Execute(ctx context.Context, in CreateRiskAcceptanceInput) (*RiskAcceptance, error) {
    // 1. Permission check: Writer+ role required
    // 2. Create risk acceptance record
    ra := &RiskAcceptance{
        ID:              uuid.New().String(),
        Name:            in.Name,
        ProductID:       in.ProductID,
        AcceptedByID:    in.RequestorUserID,
        ExpirationDate:  in.ExpirationDate,
        Notes:           in.Notes,
        ReactivateExpired: in.ReactivateExpired,
        FindingIDs:      in.FindingIDs,
        CreatedAt:       time.Now(),
    }
    uc.raRepo.Save(ctx, ra)

    // 3. Mark each finding as risk_accepted
    for _, findingID := range in.FindingIDs {
        uc.findingClient.MarkRiskAccepted(ctx, &findingv1.MarkRiskAcceptedRequest{
            FindingId:        findingID,
            RiskAcceptanceId: ra.ID,
        })
    }

    // 4. Publish event
    uc.eventPub.Publish(ctx, &events.RiskAcceptanceCreated{
        RiskAcceptanceID: ra.ID,
        ProductID:        in.ProductID,
        FindingIDs:       in.FindingIDs,
        ExpirationDate:   in.ExpirationDate,
    })

    return ra, nil
}

// usecase/risk_acceptance/expire.go
// Mirrors Python: dojo/tasks.py::risk_acceptance_expiration_handler()
// Chạy daily, kiểm tra RAs đã qua expiry date

func (uc *CheckExpiryUseCase) Execute(ctx context.Context) error {
    now := time.Now()

    // Find all active RAs with expiration_date <= today
    expiredRAs, _ := uc.raRepo.FindExpired(ctx, now)

    for _, ra := range expiredRAs {
        ra.IsExpired = true
        uc.raRepo.Save(ctx, ra)

        if ra.ReactivateExpired {
            // Reactivate all linked findings
            for _, findingID := range ra.FindingIDs {
                uc.findingClient.ReactivateRiskAccepted(ctx, &findingv1.ReactivateRiskAcceptedRequest{
                    FindingId:  findingID,
                    NoteText:   ra.ReactivateNoteText,
                    RestartSLA: ra.RestartSLAOnReactivation,
                })
            }
        }

        // Publish event → Notification Service will send email
        uc.eventPub.Publish(ctx, &events.RiskAcceptanceExpired{
            RiskAcceptanceID: ra.ID,
            ProductID:        ra.ProductID,
            FindingIDs:       ra.FindingIDs,
            ExpiredAt:        now,
        })
    }
    return nil
}

// usecase/risk_acceptance/remove_finding.go
// Remove a finding from risk acceptance (re-activates it)
func (uc *RemoveFindingFromRAUseCase) Execute(ctx context.Context, raID, findingID string) error {
    ra, _ := uc.raRepo.FindByID(ctx, raID)

    // Remove finding from RA
    ra.FindingIDs = removeFrom(ra.FindingIDs, findingID)
    uc.raRepo.Save(ctx, ra)

    // Reactivate finding
    uc.findingClient.ReactivateRiskAccepted(ctx, &findingv1.ReactivateRiskAcceptedRequest{
        FindingId: findingID,
    })

    return nil
}
```

### 3.3 Scheduler

```go
// infra/scheduler/cron.go

// Daily at 06:00 — check expired risk acceptances
scheduler.AddFunc("0 6 * * *", func() {
    uc.CheckExpiry.Execute(context.Background())
})

// Daily at 06:30 — notify expiring soon (7 days)
scheduler.AddFunc("30 6 * * *", func() {
    uc.NotifyExpiringSoon.Execute(context.Background())
})
```

---

## 4. gRPC Extensions

Extensions cần thêm vào `finding-service` proto (finding.proto):

```protobuf
// New RPCs for Risk Acceptance
service FindingService {
    // Called by product-service when RA is created
    rpc MarkRiskAccepted(MarkRiskAcceptedRequest) returns (MarkRiskAcceptedResponse);

    // Called by product-service when RA expires
    rpc ReactivateRiskAccepted(ReactivateRiskAcceptedRequest) returns (ReactivateRiskAcceptedResponse);

    // Called by report-service for RA statistics
    rpc GetRiskAcceptedCount(GetRiskAcceptedCountRequest) returns (GetRiskAcceptedCountResponse);
}

message MarkRiskAcceptedRequest {
    string finding_id         = 1;
    string risk_acceptance_id = 2;
}

message ReactivateRiskAcceptedRequest {
    string finding_id = 1;
    string note_text  = 2;  // Note to add when reactivating
    bool restart_sla  = 3;  // Restart SLA expiration date
}
```

---

## 5. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET` | `/api/v2/risk-acceptances` | JWT | List RAs (filtered by product) |
| `POST` | `/api/v2/risk-acceptances` | JWT/Writer | Create RA |
| `GET` | `/api/v2/risk-acceptances/{id}` | JWT | Get RA |
| `PUT` | `/api/v2/risk-acceptances/{id}` | JWT/Writer | Update RA |
| `DELETE` | `/api/v2/risk-acceptances/{id}` | JWT/Maintainer | Delete RA (re-activates findings) |
| `POST` | `/api/v2/risk-acceptances/{id}/findings/{fid}/remove` | JWT/Writer | Remove finding from RA |

### Create Risk Acceptance Request

```json
POST /api/v2/risk-acceptances
{
  "name": "Accept Log4Shell risk for Legacy Service",
  "product_id": "uuid",
  "finding_ids": ["finding-id-1", "finding-id-2"],
  "expiration_date": "2026-12-31",
  "notes": "Vendor patch expected Q4 2026. Mitigating controls in place.",
  "reactivate_expired": true,
  "reactivate_note_text": "Risk acceptance expired, please review and fix.",
  "restart_sla_on_reactivation": false
}
```

---

## 6. Product Configuration

```go
// Product entity fields for risk acceptance (already in CR-DD-001)
type Product struct {
    // ...
    EnableSimpleRiskAcceptance bool  // true: Writer can click "Accept Risk" directly
    EnableFullRiskAcceptance   bool  // true: Full RA workflow required
}
```

**Business Rule:**
- Nếu `enable_simple_risk_acceptance=true`: User với Writer role có thể mark finding là `risk_accepted` trực tiếp mà không cần tạo Risk Acceptance document
- Nếu `enable_full_risk_acceptance=true`: Phải tạo Risk Acceptance document với ít nhất 1 finding

---

## 7. NATS Events

```
risk_acceptance.created    {id, product_id, finding_ids, expiration_date, accepted_by}
risk_acceptance.updated    {id, product_id, changes}
risk_acceptance.expired    {id, product_id, finding_ids, expired_at}
risk_acceptance.expiring_soon {id, product_id, expiration_date, days_remaining}
```

---

## 8. Database Schema

```sql
-- risk_acceptances (in product-service DB)
CREATE TABLE risk_acceptances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(300) NOT NULL,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    accepted_by_id UUID NOT NULL,
    expiration_date DATE,
    notes TEXT,
    proof_file_id TEXT,  -- Minio/S3 key
    reactivate_expired BOOLEAN DEFAULT FALSE,
    reactivate_note_text TEXT,
    restart_sla_on_reactivation BOOLEAN DEFAULT FALSE,
    is_expired BOOLEAN DEFAULT FALSE,
    finding_ids UUID[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ra_product ON risk_acceptances(product_id);
CREATE INDEX idx_ra_expiry ON risk_acceptances(expiration_date)
    WHERE is_expired = FALSE AND expiration_date IS NOT NULL;
```

---

## 9. Acceptance Criteria

- [x] `POST /api/v2/risk-acceptances` tạo RA và mark tất cả findings là `risk_accepted`
- [x] `DELETE /api/v2/risk-acceptances/{id}` xóa RA và reactivate findings
- [x] Scheduled job daily check expired RAs và mark `is_expired=true`
- [x] `reactivate_expired=true`: khi RA expired, findings tự động reactivated
- [x] NATS `risk_acceptance.expired` được publish khi RA expired
- [x] `GET /api/v2/findings?risk_accepted=true` trả về findings đã accept risk
- [x] `enable_simple_risk_acceptance=true`: Writer có thể `POST /api/v2/findings/{id}/accept-risk` không cần tạo RA document
- [x] Risk acceptance expiring in 7 days → NATS `risk_acceptance.expiring_soon` event

## Implementation Status: ✅ DONE

> Implemented in `finding-service` (consolidated from `product-service` per architecture)
> `finding-service/internal/domain/riskacceptance/entity.go` — RiskAcceptance: ExpirationDate, ReactivateExpired, RestartSLAOnReactivation, FindingIDs[]
> `finding-service/internal/usecase/riskacceptance/risk_acceptance.go` — Create (mark findings), Delete (reactivate), CheckExpiry (daily), NotifyExpiringSoon
> `finding-service/migrations/009_risk_acceptances.sql` — risk_acceptances table + 2 indexes
> gRPC extensions in finding-service: MarkRiskAccepted, ReactivateRiskAccepted, GetRiskAcceptedCount
