# F10 — Notifications: Data Flow

---

## 1. Tổng quan flow từ NATS Event đến Channel

```
[Bất kỳ service nào publish NATS event]
    │ ví dụ: sla-service publish finding.sla.breached
    │
    ▼
NATS JetStream (durable consumer)
    │
    ▼
notification-service subscriber
    │
    ▼
1. Lookup notification_configs matching event_type + product_id
    │
    ├── Email config?  → sendEmail() → SMTP server
    ├── Slack config?  → HTTP POST → Slack webhook URL
    ├── Teams config?  → HTTP POST → Teams webhook URL
    ├── In-app?        → INSERT notifications table
    └── Webhook?       → HTTP POST → external URL (HMAC signed, retry 3x)
```

---

## 2. KEV Alert Flow

```
data-service publish NATS: kev.new
    {cve_id, product, vendor, date_added, is_ransomware}
    │
    ▼
notification-service:
    1. event_type = "kev.new"
    2. Tìm configs với event_type=kev.new (có thể nhiều configs)
    3. For each config:
        render template "kev.new" với event data
        send to configured channels
    │
    ├── Email: "🚨 New CISA KEV: CVE-2021-44228"
    ├── Slack: POST block message tới channel webhook
    └── Webhook: POST JSON + HMAC signature tới external URLs
```

---

## 3. SLA Breach Alert Flow

```
sla-service (daily cron) publish NATS: finding.sla.breached
    {finding_id, product_id, severity, sla_deadline, days_overdue}
    │
    ▼
notification-service:
    1. Fetch finding details từ finding-service
    2. Fetch product members từ finding-service/identity-service
    3. Filter: chỉ gửi cho owners + maintainers
    4. Render email template với finding details
    5. sendEmail(members_emails, rendered_template)
    6. sendSlack nếu có config
```

---

## 4. Webhook Delivery Flow

```
notification-service (webhook channel):
    │
    ▼
validateWebhookURL(url):
    DNS resolve → check IP not private
    → [blocked] → skip + log SSRF protection
    → [ok] → continue
    │
    ▼
Build payload: {event_type, timestamp, data: {...}}
Sign: X-OSV-Signature: sha256=HMAC-SHA256(secret, payload)
    │
    ▼
Attempt 1: HTTP POST (timeout 10s)
    [Success 2xx] → done
    [Fail] → wait 1s → Attempt 2
              [Fail] → wait 2s → Attempt 3
                        [Fail] → INSERT webhook_delivery_failures
                                 log error
```

---

## 5. In-app Notification Flow

```
notification-service:
    INSERT notifications {
        user_id, event_type, title, body,
        reference_type, reference_id, is_read=false
    }

User Frontend:
    Poll GET /api/v2/notifications?unread=true (mỗi 30s)
    ← [{id, title, body, created_at, reference_url}]
    
    POST /api/v2/notifications/{id}/read
    ← 204 No Content
```

---

## 6. NATS Events Consumed

| Event | Source | Action |
|-------|--------|--------|
| `kev.new` | data-service | Alert all subscribers of KEV channel |
| `finding.sla.breached` | sla-service | Alert product owners + SLA channel |
| `finding.sla.approaching` | sla-service | Warning email to product team |
| `finding.state.changed` | finding-service | (optional) notify on close/reopen |
| `report.generated` | finding-service | Notify requester: "report ready" |
| `risk.acceptance.expired` | sla-service | Alert owners: findings reopened |
| `scan.completed` | scan-service | (optional) notify product team |
