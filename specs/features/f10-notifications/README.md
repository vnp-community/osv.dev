# F10 — Notifications & Webhooks

> **Spec Folder:** `specs/features/f10-notifications/`  
> **Feature Doc:** [`docs/features/F10-notifications.md`](../../../docs/features/F10-notifications.md)  
> **SRS Refs:** FR-07-01, FR-07-02  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Channel routing, HMAC webhook, retry, SSRF protection |
| [dataflow.md](./dataflow.md) | NATS event → channel delivery flows |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `notification-service` | 8087 | Subscribe NATS events → deliver to 5 channels |

---

## Notification Channels

| Channel | Config Required | Delivery |
|---------|----------------|---------|
| **Email** | SMTP server, from address | Per user email |
| **Slack** | Webhook URL | Slack channel |
| **Microsoft Teams** | Webhook URL | Teams channel |
| **In-app** | None | Stored in DB, polled via API |
| **Webhook** | URL + HMAC secret | HTTP POST to external URL |

---

## Events Handled (14 types)

| Event | Mô tả |
|-------|-------|
| `finding.state.changed` | Finding chuyển trạng thái |
| `finding.sla.breached` | SLA deadline vượt quá |
| `finding.sla.approaching` | SLA sắp hết hạn (7 ngày) |
| `finding.duplicate.detected` | Trùng lặp tìm thấy |
| `kev.new` | CVE mới vào CISA KEV |
| `ingestion.cve.synced` | CVE sync batch hoàn thành |
| `risk.acceptance.expired` | Risk acceptance hết hạn |
| `report.generated` | Report ready to download |
| `scan.completed` | Scan job hoàn thành |
| `jira.sync.failed` | JIRA sync lỗi |
| `audit.user.login` | User đăng nhập |
| `audit.user.locked` | Account bị khóa |
| `finding.sla.critical_approaching` | Critical SLA 1 ngày còn lại |
| `product.grade.changed` | Product grade thay đổi |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v2/notifications` | In-app notifications list |
| POST | `/api/v2/notifications/{id}/read` | Mark as read |
| GET/POST | `/api/v2/notification-configs` | Configure channels per product |
| GET/POST | `/api/v2/webhooks` | Webhook endpoints management |
| POST | `/api/v2/webhooks/test` | Test webhook delivery |

---

## Webhook Security

- **Signature:** `X-OSV-Signature: sha256={HMAC-SHA256(secret, payload_bytes)}`
- **SSRF protection:** Block requests to private IP ranges (10.x, 192.168.x, 127.x, etc.)
- **Retry:** 3 attempts, exponential backoff (1s, 2s, 4s)
- **Timeout:** 10s per delivery attempt
