# F10 — Notifications & Webhooks

**Status:** ✅ v2.1–v2.2 Implemented  
**CR References:** CR-DD-007, CR-GCV-006  
**Services:** `notification-service` (port 8087)  
**UI Routes:** `/notifications`, `/integrations/webhooks`  
**UI Components:** `NotificationCenter`, `WebhookEvents`

---

## 1. Mô tả

Hệ thống thông báo đa kênh hỗ trợ 14 loại events, routing đến Email/Slack/Teams/Webhook/In-app theo rules cấu hình. Webhook delivery bảo mật bằng HMAC-SHA256, có retry/backoff và SSRF protection.

---

## 2. Notification Channels

| Channel | Config | Mô tả |
|---------|--------|-------|
| **Email** | SMTP server + from address | Rich HTML email |
| **Slack** | Webhook URL | Slack block kit messages |
| **Microsoft Teams** | Webhook URL | Teams adaptive cards |
| **In-app** | NATS → stored alerts | Real-time trong UI |
| **Webhook** | URL + HMAC secret | HTTP POST với signature |

---

## 3. 14 Alert Event Types

| Event | Channels | Mô tả |
|-------|----------|-------|
| `kev.new` | Webhook + Slack + Email | CVE mới vào CISA KEV catalog |
| `cve.critical.new` | Webhook + Email | CVE Critical mới cho subscribed vendor |
| `epss.spike` | Webhook | EPSS score vượt ngưỡng (> 0.9) |
| `finding.created` | In-app | Finding mới được tạo |
| `finding.sla.breached` | Email + Slack | SLA deadline vượt quá |
| `finding.status.changed` | In-app | Finding status thay đổi |
| `risk_acceptance.expired` | In-app + Email | Risk acceptance hết hạn |
| `jira.issue.created` | In-app | JIRA ticket được tạo |
| `jira.issue.resolved` | In-app | JIRA ticket được đóng |
| `scan.completed` | In-app + Email | Scan hoàn thành |
| `scan.failed` | Email + Slack | Scan thất bại |
| `user.invited` | Email | User được mời vào platform |
| `password.reset` | Email | Password reset request |
| `finding.batch_created` | In-app | Batch scan import completed |

---

## 4. Notification Rules

Users/Admins có thể config rules: "khi event X xảy ra với filter Y thì gửi đến channel Z"

**Rule Config:**
```json
{
  "name": "Critical CVE for Apache",
  "event_type": "cve.critical.new",
  "filters": {
    "vendor": "apache",
    "min_cvss": 9.0
  },
  "channels": ["email", "slack"],
  "enabled": true
}
```

---

## 5. Webhook Service

### 5.1 Registration
```
POST /api/v1/webhooks
{
  "url": "https://api.company.com/osv-events",
  "secret": "my-hmac-secret",
  "event_types": ["kev.new", "finding.sla.breached"],
  "filters": {
    "vendors": ["apache", "microsoft"],
    "min_severity": "HIGH"
  }
}
```

### 5.2 Delivery
**Headers:**
```
Content-Type: application/json
X-OSV-Signature: sha256={hex_hmac}
X-OSV-Event: kev.new
X-OSV-Delivery: {unique_delivery_id}
```

**HMAC Signature:**
```
HMAC-SHA256(secret, raw_body)
```

### 5.3 Retry Policy
| Attempt | Delay |
|---------|-------|
| 1st retry | 1 giây |
| 2nd retry | 5 giây |
| 3rd retry | 30 giây |
| After 3 failures | Webhook marked as `failed`, alert to admin |

**Timeout:** 10 giây per attempt

### 5.4 SSRF Protection
Block private IP ranges:
- `10.0.0.0/8`
- `172.16.0.0/12`
- `192.168.0.0/16`
- `127.0.0.0/8`
- `::1` (IPv6 loopback)

### 5.5 Deduplication
- Window: 1 giờ
- Key: `{alert_type, cve_id}`
- Tránh gửi duplicate alerts trong burst scenarios

### 5.6 Webhook Test
```
POST /api/v1/webhooks/{id}/test   → Gửi test payload đến webhook URL
```

### 5.7 APIs
```
GET /api/v1/webhooks              → List registered webhooks
POST /api/v1/webhooks             → Register new webhook
GET /api/v1/webhooks/{id}         → Webhook detail
PUT /api/v1/webhooks/{id}         → Update webhook
DELETE /api/v1/webhooks/{id}      → Deregister webhook
GET /api/v1/webhooks/{id}/events  → Delivery history (last 50)
POST /api/v1/webhooks/{id}/test   → Send test event
```

---

## 6. In-App Notification Center

**UI Component:** `NotificationCenter`  
**Route:** `/notifications`

**Features:**
- Danh sách notifications với unread count badge
- Mark as read / Mark all as read
- Filter by event type
- Real-time updates qua SSE stream (v3.1)
- Notification detail với link đến entity

**APIs:**
```
GET /api/v1/notifications                   → List notifications (paginated)
PATCH /api/v1/notifications/{id}/read       → Mark as read
POST /api/v1/notifications/mark-all-read    → Mark all as read
GET /api/v1/notifications/stream            → SSE real-time stream (v3.1)
```

---

## 7. Subscription Filters

Người dùng có thể subscribe events theo:
- **Vendor/Product:** Chỉ nhận alerts về Apache, Microsoft...
- **Severity:** Min severity threshold
- **Product:** Chỉ alerts cho products mình phụ trách
- **Event type:** Chọn loại events quan tâm

---

## 8. NATS Integration

Notification-service subscribe các NATS subjects sau:

| Subject | Action |
|---------|--------|
| `kev.new` | Gửi KEV alert qua Webhook + Slack + Email |
| `finding.created` | Gửi in-app notification |
| `finding.status.changed` | Gửi in-app notification |
| `finding.sla.breached` | Gửi Email + Slack |
| `risk_acceptance.expired` | Gửi In-app + Email |
| `jira.issue.created` | Gửi In-app |
| `finding.batch_created` | Gửi In-app |

---

## 9. Database Schema (`osv_notif`)

| Table | Mô tả |
|-------|-------|
| `webhooks` | Registered webhook endpoints |
| `notification_rules` | User notification preferences |
| `alerts` | In-app notification store |
| `notification_log` | Delivery history + status |

---

## 10. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Webhook delivery timeout | 10 giây per attempt |
| Max webhook retries | 3 attempts |
| Deduplication window | 1 giờ |
| In-app notification | Near real-time (< 5 giây) |
| SSE latency (v3.1) | < 2 giây từ NATS event đến browser |
| SSRF protection | Block all private IP ranges |
