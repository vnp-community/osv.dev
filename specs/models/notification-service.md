# Data Models — notification-service

> **Service**: `services/notification-service`  
> **Mô tả**: Gửi notifications qua nhiều channels (email, Slack, MS Teams, webhook, in-app). Quản lý notification rules theo event type và product, alert subscriptions cho CVE/KEV alerts, và webhook endpoints cho external integrations.  
> **Storage**: PostgreSQL (rules, delivery records, subscriptions), Redis (webhook dedup), MongoDB (alerts)  
> **Go packages**: `domain/alert`, `domain/rule`, `domain/webhook`, `domain/delivery`, `domain/subscription`

---

## 1. NotificationRule

Định nghĩa channels nhận notification cho từng loại event.  
Package: `domain/rule`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `user_id` | *UUID | Yes | nil = system-wide rule |
| `product_id` | *UUID | Yes | nil = áp dụng cho tất cả products |
| `scan_added` | []Channel | No | |
| `test_added` | []Channel | No | |
| `finding_added` | []Channel | No | |
| `finding_status_changed` | []Channel | No | |
| `jira_update` | []Channel | No | |
| `engagement_added` | []Channel | No | |
| `engagement_closed` | []Channel | No | |
| `risk_acceptance_expiration` | []Channel | No | |
| `sla_breach` | []Channel | No | |
| `sla_expiring_soon` | []Channel | No | |
| `product_added` | []Channel | No | |
| `user_mentioned` | []Channel | No | |
| `closed_finding_removed` | []Channel | No | |
| `review_requested` | []Channel | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — EventType**:

| Giá trị | Mô tả |
|---------|-------|
| `scan_added` | Scan mới được thêm |
| `test_added` | Test mới được thêm |
| `finding_added` | Finding mới được phát hiện |
| `finding_status_changed` | Trạng thái finding thay đổi |
| `jira_update` | JIRA issue được cập nhật |
| `engagement_added` | Engagement mới |
| `engagement_closed` | Engagement đóng |
| `risk_acceptance_expiration` | Risk acceptance sắp hết hạn |
| `sla_breach` | SLA bị vi phạm |
| `sla_expiring_soon` | SLA sắp hết hạn |
| `product_added` | Product mới |
| `user_mentioned` | User được mention |
| `closed_finding_removed` | Finding đã đóng bị xóa |
| `review_requested` | Yêu cầu review |

**Enums — Channel**:

| Giá trị | Mô tả |
|---------|-------|
| `email` | Email notification |
| `slack` | Slack message |
| `msteams` | Microsoft Teams |
| `webhook` | HTTP webhook delivery |
| `inapp` | In-app alert (UI notification) |

---

## 2. Alert (In-app)

In-app notification hiển thị trong UI.  
Package: `domain/alert`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `user_id` | UUID | No | FK → User |
| `event_type` | string | No | Loại event |
| `title` | string | No | Tiêu đề notification |
| `description` | string | Yes | Mô tả chi tiết |
| `url` | string | Yes | Deep link URL |
| `is_read` | bool | No | Đã đọc chưa |
| `created_at` | timestamp | No | |

---

## 3. Webhook

Registered webhook endpoint nhận CVE/KEV event notifications.  
Package: `domain/webhook`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | Khóa chính |
| `url` | string | No | Endpoint URL |
| `secret` | string | No | HMAC-SHA256 signing key |
| `events` | []EventType | No | Danh sách event types được subscribe |
| `is_active` | bool | No | Có đang kích hoạt không |
| `owner_id` | string | No | FK → User |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Webhook EventTypes**:

| Giá trị | Mô tả |
|---------|-------|
| `kev.new` | KEV entry mới từ CISA |
| `cve.new.critical` | CVE Critical mới |
| `cve.new.high` | CVE High mới |
| `cve.epss.high` | CVE có EPSS cao |
| `cve.vendor` | CVE ảnh hưởng vendor đang subscribe |
| `cve.product` | CVE ảnh hưởng product đang subscribe |

---

## 4. WebhookDelivery

Record mỗi lần giao hàng webhook attempt.  
Package: `domain/webhook`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | |
| `webhook_id` | string | No | FK → Webhook |
| `event_type` | EventType | No | Loại event được deliver |
| `payload` | string | No | JSON payload |
| `status_code` | *int | Yes | HTTP response code |
| `attempt` | int | No | Lần thử thứ mấy |
| `status` | DeliveryStatus | No | Trạng thái delivery |
| `delivered_at` | *timestamp | Yes | Thời điểm deliver thành công |
| `next_retry_at` | *timestamp | Yes | Thời điểm retry tiếp theo |
| `created_at` | timestamp | No | |

**Enums — DeliveryStatus**:

| Giá trị | Mô tả |
|---------|-------|
| `pending` | Chờ gửi |
| `delivered` | Đã gửi thành công |
| `failed` | Gửi thất bại |
| `retrying` | Đang retry |

---

## 5. DeliveryRecord

Tracking notification delivery attempt cho tất cả channels.  
Package: `domain/delivery`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `rule_id` | *UUID | Yes | FK → NotificationRule |
| `event_type` | string | No | |
| `channel` | string | No | |
| `recipient` | string | No | Email, Slack channel, webhook URL, v.v. |
| `status` | Status | No | Trạng thái delivery |
| `attempts` | int | No | Số lần thử |
| `last_attempt_at` | *timestamp | Yes | |
| `next_retry_at` | *timestamp | Yes | |
| `error_message` | string | Yes | |
| `payload` | json.RawMessage | Yes | Payload gửi đi |
| `created_at` | timestamp | No | |

**Enums — Status**: `pending` \| `sent` \| `failed` \| `retrying`

---

## 6. AlertSubscription

User subscribe nhận notifications về vendor/product/KEV.  
Package: `domain/subscription`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | |
| `owner_id` | string | No | FK → User |
| `type` | SubscriptionType | No | Loại subscription |
| `value` | string | No | e.g. `apache` cho vendor subscription |
| `min_severity` | string | No | `CRITICAL` \| `HIGH` \| `MEDIUM` \| `LOW` |
| `min_epss` | *float64 | Yes | EPSS threshold tối thiểu |
| `is_active` | bool | No | |
| `created_at` | timestamp | No | |

**Enums — SubscriptionType**:

| Giá trị | Mô tả |
|---------|-------|
| `vendor` | Subscribe CVE theo vendor name |
| `product` | Subscribe CVE theo product name |
| `kev` | Subscribe tất cả KEV updates từ CISA |

---

## 7. Relationships

```
NotificationRule ─── User (N:1, user_id nullable = system rule)
NotificationRule ─── Product (N:1, product_id nullable = all products)
NotificationRule ──→ DeliveryRecord (1:N khi event xảy ra)
Alert ─────────────── User (N:1, user_id)
Webhook ────────────── WebhookDelivery (1:N)
AlertSubscription ─── User (N:1, owner_id)
```
