# Data Models — audit-service

> **Service**: `services/audit-service`  
> **Mô tả**: Ghi nhận immutable audit trail cho tất cả thao tác trong hệ thống. Records không bao giờ bị update hoặc delete sau khi tạo. Có HMAC-SHA256 signature để đảm bảo tính toàn vẹn.  
> **Storage**: PostgreSQL (append-only, protected by RLS)  
> **Go package**: `services/audit-service/internal/domain/event`

---

## 1. AuditEvent

Record bất biến ghi nhận hành động thực hiện trong hệ thống.  
Captures: **WHO** did **WHAT** to **WHICH** resource at **WHEN**, với cryptographic signature.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `event_id` | string | Yes | NATS message ID hoặc external correlation ID |
| `event_type` | string | No | NATS subject, e.g. `defectdojo.finding.status_changed` |
| `actor_id` | *UUID | Yes | FK → User (nil nếu system action) |
| `actor_email` | *string | Yes | Email actor tại thời điểm event |
| `actor_type` | string | No | `user` \| `system` \| `service` |
| `service_name` | string | Yes | Tên microservice phát sinh event |
| `resource_type` | string | No | `finding` \| `product` \| `engagement` \| etc. |
| `resource_id` | UUID | No | ID của resource bị tác động |
| `action` | string | No | `created` \| `updated` \| `deleted` \| `status_changed` \| etc. |
| `changes` | map[string]interface{} | Yes | Old/new values của các trường thay đổi |
| `metadata` | map[string]interface{} | Yes | Context bổ sung |
| `occurred_at` | timestamp | No | Thời điểm event xảy ra (từ source service) |
| `recorded_at` | timestamp | No | Thời điểm audit-service ghi nhận |
| `signature` | string | No | HMAC-SHA256 signature để verify tính toàn vẹn |

**Signature payload** (canonical string cho HMAC):
```
event_type|resource_id|occurred_at(RFC3339Nano)|actor_id
```

> **Immutability**: Chỉ có `Create` và `FindByID` và `List`. Không có `Update` hay `Delete`.  
> RLS policies tại DB level enforce append-only.

---

## 2. Query

Tham số lọc khi truy vấn audit events.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `event_types` | []string | Yes | Lọc theo danh sách event types |
| `resource_type` | *string | Yes | Lọc theo resource type |
| `resource_id` | *UUID | Yes | Lọc theo resource ID cụ thể |
| `actor_id` | *UUID | Yes | Lọc theo actor |
| `from` | *timestamp | Yes | Từ thời điểm này |
| `to` | *timestamp | Yes | Đến thời điểm này |
| `limit` | int | No | Số kết quả tối đa |
| `offset` | int | No | Offset cho phân trang |
| `order_by` | string | Yes | `occurred_at DESC` (default) \| `occurred_at ASC` |

---

## 3. Repository Interface

```go
type Repository interface {
    Create(ctx, *AuditEvent) error
    FindByID(ctx, id UUID) (*AuditEvent, error)
    List(ctx, Query) ([]*AuditEvent, int64, error)
    // ExportJSON streams NDJSON cho compliance export
    ExportJSON(ctx, Query, io.Writer) error
}
```

---

## 4. NATS Subjects (AuditableSubjects)

Danh sách đầy đủ NATS subjects mà audit-service subscribe để ghi nhận:

| Nhóm | Subjects |
|------|---------|
| **Findings** | `defectdojo.finding.created`, `.updated`, `.deleted`, `.status_changed`, `.bulk_updated`, `.risk_accepted`, `.false_positive_marked`, `.duplicate_detected` |
| **Products** | `defectdojo.product.created`, `.updated`, `.deleted`, `.member.added`, `.member.removed`, `.member.role_changed` |
| **Engagements** | `defectdojo.engagement.created`, `.updated`, `.closed`, `.reopened` |
| **Tests** | `defectdojo.test.created`, `.updated` |
| **Scan imports** | `scan.import.started`, `.completed`, `.failed` |
| **Risk acceptances** | `defectdojo.risk_acceptance.created`, `.updated`, `.expired` |
| **SLA** | `defectdojo.sla.config.created`, `.updated`, `.deleted`, `defectdojo.sla.breach` |
| **JIRA** | `defectdojo.jira.issue.created`, `.updated`, `defectdojo.jira.synced` |
| **Users** | `identity.user.login`, `.login_failed`, `.logout`, `.password_changed`, `.role_changed`, `.created`, `.deleted` |
| **Reports** | `defectdojo.report.generated`, `.deleted` |

---

## 5. Helper Functions

| Hàm | Mô tả |
|-----|-------|
| `SubjectToResourceType(subject)` | Extract resource type từ NATS subject. e.g. `defectdojo.finding.status_changed` → `finding` |
| `SubjectToAction(subject)` | Extract action từ NATS subject. e.g. `defectdojo.finding.status_changed` → `status_changed` |

---

## 6. Relationships

```
AuditEvent ──── Actor (User via actor_id, optional)
AuditEvent ──── Any resource (via resource_type + resource_id)
NATS Subject → AuditEvent (1:1, mỗi message tạo 1 AuditEvent)
```
