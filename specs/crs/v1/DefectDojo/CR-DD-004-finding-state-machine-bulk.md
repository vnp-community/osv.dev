# ✅ COMPLETED — CR-DD-004 — Finding State Machine & Bulk Operations

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-004 |
| **Tiêu đề** | Finding State Machine (6 States), Bulk Operations, CVSS v3/v4 Scoring |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/05-finding-management-service.md`, `SRS.md §3.1 FR-DM-02` |
| **Target Service** | `finding-service` (extend) |
| **Ưu tiên** | 🔴 High |
| **Loại** | Feature Enhancement |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV `finding-service` có Finding CRUD cơ bản nhưng thiếu:
1. **State machine** đầy đủ (6 states với valid transitions)
2. **Bulk operations** (close/reopen/accept-risk nhiều findings cùng lúc)
3. **CVSS v3/v4 scoring** và validation
4. **Finding groups** (nhóm findings theo component/file path)
5. **Notes và file attachments**
6. **False positive management**
7. **Endpoint tracking** với status per finding

---

## 2. Finding State Machine

### 2.1 States

```
DefectDojo Finding States (từ models.py booleans):

active=True,  verified=False, false_p=False, duplicate=False, out_of_scope=False, is_mitigated=False, risk_accepted=False
→ StateActive (default)

active=False, is_mitigated=True
→ StateMitigated (Closed/Fixed)

active=False, false_p=True
→ StateFalsePositive

active=False, risk_accepted=True (via Risk Acceptance)
→ StateRiskAccepted

active=False, out_of_scope=True
→ StateOutOfScope

duplicate=True (secondary record)
→ StateDuplicate
```

### 2.2 Valid Transitions

```
StateActive      → StateMitigated     (close)
StateActive      → StateFalsePositive (mark_false_p)
StateActive      → StateRiskAccepted  (accept_risk)
StateActive      → StateOutOfScope    (mark_out_of_scope)
StateMitigated   → StateActive        (reopen)
StateFalsePositive → StateActive      (undo_false_p)
StateRiskAccepted → StateActive       (risk_acceptance_expired)
StateOutOfScope  → StateActive        (remove_out_of_scope)
```

---

## 3. Domain Changes

### 3.1 Finding Entity Extension

```go
// finding-service/internal/domain/finding/entity.go
// Extends existing Finding entity with DefectDojo fields

type Finding struct {
    ID string

    // Core identification
    Title       string
    Description string
    Mitigation  string
    Impact      string
    References  string

    // Severity (mirrors Django Finding)
    Severity          Severity  // Critical|High|Medium|Low|Info
    NumericalSeverity int       // 4|3|2|1|0 (for sorting)

    // CVE / CWE
    CVE            string  // CVE-2021-44228
    CWE            int     // 79 (XSS), 89 (SQLi), etc.
    VulnIDFromTool string  // Scanner-specific ID

    // CVSS scoring (NEW)
    CVSSv3      string   // CVSS:3.1/AV:N/AC:L/...
    CVSSv3Score *float64 // 9.8
    CVSSv4      string   // CVSS:4.0/AV:N/AC:L/...
    CVSSv4Score *float64 // 9.3

    // Status flags (mirrors Django boolean fields)
    Active        bool
    Verified      bool
    FalsePositive bool    // false_p in Django
    Duplicate     bool
    OutOfScope    bool
    IsMitigated   bool
    RiskAccepted  bool

    // Lifecycle timestamps
    Date              time.Time
    MitigatedAt       *time.Time
    MitigatedByID     *string
    LastReviewed      *time.Time
    LastStatusUpdate  *time.Time
    SLAExpirationDate *time.Time  // computed by SLA-service

    // Relations (denormalized for performance)
    TestID       string
    EngagementID string
    ProductID    string

    // Duplicate tracking
    DuplicateFindingID *string  // FK to self (original finding)
    FindingGroupID     *string

    // Component info (SCA)
    ComponentName    string
    ComponentVersion string
    Service          string

    // File location (SAST)
    FilePath   string
    LineNumber  int
    SourceCode string

    // Deduplication
    HashCode string

    // Tags
    Tags          []string
    InheritedTags []string

    CreatedAt time.Time
    UpdatedAt time.Time
}

// State machine
func (f *Finding) CurrentState() FindingState {
    switch {
    case f.Duplicate:     return StateDuplicate
    case f.FalsePositive: return StateFalsePositive
    case f.OutOfScope:    return StateOutOfScope
    case f.RiskAccepted:  return StateRiskAccepted
    case f.IsMitigated:   return StateMitigated
    default:              return StateActive
    }
}

type FindingState string
const (
    StateActive        FindingState = "active"
    StateMitigated     FindingState = "mitigated"
    StateFalsePositive FindingState = "false_positive"
    StateRiskAccepted  FindingState = "risk_accepted"
    StateOutOfScope    FindingState = "out_of_scope"
    StateDuplicate     FindingState = "duplicate"
)

var ValidTransitions = map[FindingState]map[FindingState]bool{
    StateActive: {
        StateMitigated:     true,
        StateFalsePositive: true,
        StateRiskAccepted:  true,
        StateOutOfScope:    true,
    },
    StateMitigated:     {StateActive: true},
    StateFalsePositive: {StateActive: true},
    StateRiskAccepted:  {StateActive: true},
    StateOutOfScope:    {StateActive: true},
}

func (f *Finding) CanTransitionTo(newState FindingState) bool {
    return ValidTransitions[f.CurrentState()][newState]
}
```

### 3.2 State Transition Use Cases

```go
// usecase/finding/close.go
// Mirrors Python: dojo/views.py::close_finding()
func (uc *CloseFindingUseCase) Execute(ctx context.Context, in CloseFindingInput) error {
    finding, _ := uc.findingRepo.FindByID(ctx, in.FindingID)

    if !finding.CanTransitionTo(StateMitigated) {
        return ErrInvalidStateTransition
    }

    now := time.Now()
    finding.Active = false
    finding.IsMitigated = true
    finding.MitigatedAt = &now
    finding.MitigatedByID = &in.RequestorUserID
    finding.LastStatusUpdate = &now

    uc.findingRepo.Save(ctx, finding)
    uc.eventPub.Publish(ctx, &events.FindingStatusChanged{
        FindingID: finding.ID, OldState: "active", NewState: "mitigated",
    })
    return nil
}

// usecase/finding/reopen.go
// Mirrors Python: dojo/views.py::reopen_finding()
func (uc *ReopenFindingUseCase) Execute(ctx context.Context, in ReopenFindingInput) error {
    finding, _ := uc.findingRepo.FindByID(ctx, in.FindingID)

    if !finding.CanTransitionTo(StateActive) {
        return ErrInvalidStateTransition
    }

    now := time.Now()
    finding.Active = true
    finding.IsMitigated = false
    finding.MitigatedAt = nil
    finding.LastStatusUpdate = &now

    uc.findingRepo.Save(ctx, finding)
    uc.eventPub.Publish(ctx, &events.FindingStatusChanged{
        FindingID: finding.ID, OldState: "mitigated", NewState: "active",
    })
    return nil
}

// usecase/finding/mark_false_positive.go
func (uc *MarkFalsePositiveUseCase) Execute(ctx context.Context, in MarkFPInput) error {
    finding, _ := uc.findingRepo.FindByID(ctx, in.FindingID)
    finding.Active = false
    finding.FalsePositive = true
    uc.findingRepo.Save(ctx, finding)
    // ...
}

// usecase/finding/accept_risk.go
// Mirrors Python: dojo/views.py::accept_risk() — triggered by RiskAcceptance creation
func (uc *AcceptRiskUseCase) Execute(ctx context.Context, in AcceptRiskInput) error {
    finding, _ := uc.findingRepo.FindByID(ctx, in.FindingID)
    finding.Active = false
    finding.RiskAccepted = true
    uc.findingRepo.Save(ctx, finding)
    // ...
}
```

### 3.3 Bulk Operations

```go
// usecase/finding/bulk.go
// Mirrors Python: dojo/views.py::finding_bulk_update()

type BulkOperation string
const (
    BulkClose        BulkOperation = "close"
    BulkReopen       BulkOperation = "reopen"
    BulkFalsePositive BulkOperation = "false_positive"
    BulkUndoFP       BulkOperation = "undo_false_positive"
    BulkAcceptRisk   BulkOperation = "accept_risk"
    BulkApplyTags    BulkOperation = "apply_tags"
    BulkDelete       BulkOperation = "delete"
)

type BulkUpdateInput struct {
    FindingIDs  []string       // IDs to operate on
    Operation   BulkOperation  // What to do
    Tags        []string       // For apply_tags
    RequestorUserID string
}

func (uc *BulkUpdateUseCase) Execute(ctx context.Context, in BulkUpdateInput) (*BulkUpdateResult, error) {
    results := &BulkUpdateResult{}

    // Process in batches of 100
    for i := 0; i < len(in.FindingIDs); i += 100 {
        batch := in.FindingIDs[i:min(i+100, len(in.FindingIDs))]
        findings, _ := uc.findingRepo.FindByIDs(ctx, batch)

        for _, finding := range findings {
            var err error
            switch in.Operation {
            case BulkClose:
                err = uc.closeUC.Execute(ctx, CloseFindingInput{FindingID: finding.ID, RequestorUserID: in.RequestorUserID})
            case BulkReopen:
                err = uc.reopenUC.Execute(ctx, ReopenFindingInput{FindingID: finding.ID, RequestorUserID: in.RequestorUserID})
            case BulkApplyTags:
                finding.Tags = append(finding.Tags, in.Tags...)
                err = uc.findingRepo.Save(ctx, finding)
            case BulkDelete:
                err = uc.findingRepo.Delete(ctx, finding.ID)
            }

            if err != nil {
                results.Failed++
            } else {
                results.Succeeded++
            }
        }
    }

    return results, nil
}
```

### 3.4 CVSS v3/v4 Scoring

```go
// finding-service/internal/infra/cvss/calculator.go
// Mirrors Python: cvss library usage in dojo/api_v2/serializers.py

import "github.com/goark/go-cvss/v3/metric"

type CVSSCalculator struct{}

// ComputeCVSSv3Score tính score từ CVSS v3 vector string
func (c *CVSSCalculator) ComputeCVSSv3Score(vector string) (*float64, error) {
    base, err := metric.NewBase().Decode(vector)
    if err != nil {
        return nil, fmt.Errorf("invalid CVSSv3 vector: %w", err)
    }
    score := base.Score()
    return &score, nil
}

// ComputeNumericalSeverity từ score
// Mirrors Python: dojo/models.py::Finding.get_numerical_severity()
func (c *CVSSCalculator) NumericalSeverityFromScore(score float64) Severity {
    switch {
    case score >= 9.0: return SeverityCritical
    case score >= 7.0: return SeverityHigh
    case score >= 4.0: return SeverityMedium
    case score >= 0.1: return SeverityLow
    default:           return SeverityInfo
    }
}
```

### 3.5 Finding Groups

```go
// domain/group/entity.go
// Mirrors Python: dojo/models.py::Finding_Group
// Dùng để nhóm findings theo component, file path, CWE...

type FindingGroup struct {
    ID         string
    Name       string
    TestID     string
    FindingIDs []string
    JIRAIssueKey *string  // JIRA issue linked to group
    CreatedAt  time.Time
}
```

### 3.6 Notes & File Attachments

```go
// domain/note/entity.go
// Mirrors Python: dojo/models.py::Notes
type Note struct {
    ID         string
    FindingID  string
    AuthorID   string
    Content    string
    EditCount  int
    IsPrivate  bool  // Private notes (không show cho lower-role users)
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// domain/file/entity.go
// Mirrors Python: dojo/models.py::FileUpload
type FileAttachment struct {
    ID        string
    FindingID string
    Filename  string
    MimeType  string
    SizeBytes int64
    StorageKey string  // Minio/S3 key
    UploadedByID string
    CreatedAt  time.Time
}
```

---

## 4. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET` | `/api/v2/findings?severity=High&active=true&product_id=X` | JWT | List với filters |
| `POST` | `/api/v2/findings` | JWT/Writer | Create |
| `GET` | `/api/v2/findings/{id}` | JWT | Get |
| `PUT/PATCH` | `/api/v2/findings/{id}` | JWT/Writer | Update |
| `DELETE` | `/api/v2/findings/{id}` | JWT/Maintainer | Delete |
| `POST` | `/api/v2/findings/{id}/close` | JWT/Writer | Close |
| `POST` | `/api/v2/findings/{id}/reopen` | JWT/Writer | Reopen |
| `POST` | `/api/v2/findings/{id}/accept-risk` | JWT/Writer | Accept risk |
| `POST` | `/api/v2/findings/{id}/false-positive` | JWT/Writer | Mark FP |
| `GET` | `/api/v2/findings/{id}/duplicates` | JWT | List duplicates |
| `POST` | `/api/v2/findings/bulk` | JWT/Writer | Bulk operations |
| `GET` | `/api/v2/findings/{id}/notes` | JWT | List notes |
| `POST` | `/api/v2/findings/{id}/notes` | JWT | Add note |
| `GET` | `/api/v2/finding-groups` | JWT | List groups |
| `POST` | `/api/v2/finding-groups` | JWT | Create group |
| `GET` | `/api/v2/findings/severity_count` | JWT | Severity stats |

### Severity Count Endpoint

```json
GET /api/v2/findings/severity_count?product_id=X

{
  "critical": 3,
  "high": 12,
  "medium": 45,
  "low": 89,
  "info": 23,
  "total": 172
}
```

---

## 5. NATS Events

```
finding.created                 {finding_id, test_id, product_id, severity}
finding.status_changed          {finding_id, product_id, old_state, new_state, by_user_id}
finding.bulk_updated            {finding_ids, operation, product_id}
finding.risk_accepted           {finding_id, product_id, risk_acceptance_id}
finding.duplicate_detected      {finding_id, duplicate_of_id}
finding.false_positive_marked   {finding_id, product_id}
```

---

## 6. Database Schema Changes

```sql
-- Extend findings table with missing columns
ALTER TABLE findings ADD COLUMN IF NOT EXISTS
    cvss_v4 VARCHAR(255),
    cvss_v4_score DECIMAL(4,1),
    false_p BOOLEAN DEFAULT FALSE,
    out_of_scope BOOLEAN DEFAULT FALSE,
    risk_accepted BOOLEAN DEFAULT FALSE,
    finding_group_id UUID REFERENCES finding_groups(id),
    duplicate_finding_id UUID REFERENCES findings(id),
    source_code TEXT,
    vuln_id_from_tool VARCHAR(255);

-- finding_groups (NEW)
CREATE TABLE finding_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    test_id UUID NOT NULL,
    jira_issue_key VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- notes (NEW)
CREATE TABLE finding_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    author_id UUID NOT NULL,
    content TEXT NOT NULL,
    edit_count INTEGER DEFAULT 0,
    is_private BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- file_attachments (NEW)
CREATE TABLE finding_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100),
    size_bytes BIGINT,
    storage_key TEXT NOT NULL,  -- Minio key
    uploaded_by_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 7. Acceptance Criteria

- [x] `POST /api/v2/findings/{id}/close` thay đổi state: Active → Mitigated
- [x] `POST /api/v2/findings/{id}/reopen` thay đổi state: Mitigated → Active
- [x] Transition không hợp lệ (Duplicate → Active) trả về 409 Conflict
- [x] `POST /api/v2/findings/bulk` với `{"operation":"close","finding_ids":["id1","id2"]}` đóng nhiều findings
- [x] CVSS v3 vector string → cvssv3_score được tính tự động
- [x] Severity count endpoint phân loại đúng theo severity
- [x] Notes có thể thêm, edit, xóa per finding
- [x] File attachment upload/download qua Minio/S3
- [x] Finding group: gộp 3 findings → 1 group → 1 JIRA issue
- [x] NATS `finding.status_changed` được publish sau mỗi state transition

## Implementation Status: ✅ DONE

> `finding-service/internal/domain/finding/entity.go` — 6 states (Active/Mitigated/FalsePositive/RiskAccepted/OutOfScope/Duplicate) + ValidTransitions map + CanTransitionTo()
> `finding-service/internal/usecase/finding/{close,reopen,mark_fp,accept_risk,bulk}.go` — all state transitions + BulkUpdateUseCase (batches of 100)
> `finding-service/internal/infra/cvss/calculator.go` — CVSS v3 + v4 vector → score via go-cvss library
> `finding-service/internal/domain/{note,group}/entity.go` — FindingNote (private), FindingGroup (jira_issue_key)
> `finding-service/migrations/{010_finding_groups,011_finding_notes,012_finding_files}.sql` — new tables
