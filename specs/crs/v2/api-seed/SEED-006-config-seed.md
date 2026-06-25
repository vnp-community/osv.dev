# Change Request SEED-006: Seed Notification Rules, SLA Config & JIRA Config

**Cập nhật:** 2026-06-18  
**Status:** Proposed  
**Domain:** notification-service, sla-service, jira-service  
**Priority:** 🟡 MEDIUM — Configuration data cần thiết để demo notifications và workflow integrations  
**Depends on:** SEED-001 (users), SEED-002 (products)

---

## 1. Bối cảnh

Sau khi có users, products và findings, hệ thống cần được cấu hình để hoạt động đúng:
- **SLA configurations** cho từng product
- **Notification rules** cho từng user/system
- **Alert subscriptions** (vendor/product/KEV)
- **JIRA configurations** cho từng product
- **Webhooks** cho external system integration

Phân tích hiện trạng:

| Use-case | Endpoint hiện tại | Trạng thái |
|---------|------------------|-----------|
| Tạo SLA config | `POST /api/v2/sla-configurations` | ✅ Có |
| Gán SLA cho product | `POST /api/v2/sla-configurations/{id}/assign/{product_id}` | ✅ Có |
| Bulk create SLA configs | **THIẾU** | ❌ |
| Tạo notification rule | `POST /api/v2/notification-rules` | ✅ Có |
| Tạo system notification rule | `PUT /api/v2/system-notification-rules` | ✅ Có (replace all) |
| Bulk create notification rules | **THIẾU** | ❌ |
| Tạo alert subscription | `POST /api/v2/subscriptions` | ✅ Có |
| Bulk create subscriptions | **THIẾU** | ❌ |
| Tạo JIRA config | `POST /api/v2/jira-configurations` | ✅ Có |
| Bulk create JIRA configs | **THIẾU** | ❌ |
| Tạo webhook | `POST /api/v1/webhooks` hoặc `POST /api/v2/webhooks` | ✅ Có |
| Bulk create webhooks | **THIẾU** | ❌ |
| Seed ranking data | `POST /ranking` (single) | ⚠️ Không có bulk |
| Import notification rules từ file | **THIẾU** | ❌ |
| Seed global default SLA | **THIẾU** | ❌ Không có endpoint set global default |

---

## 2. Thay đổi Đề Xuất

### 2.1 [HIGH] `POST /api/v2/sla-configurations/bulk` — Bulk create SLA configurations

**Gateway**:
```
POST /api/v2/sla-configurations/bulk  →  sla-service:8086  (adminOnly)
```

**Request body**:
```json
{
  "configurations": [
    {
      "name": "Standard SLA",
      "description": "Default SLA for most products",
      "critical_days": 7,
      "high_days": 30,
      "medium_days": 90,
      "low_days": 365,
      "is_default": true
    },
    {
      "name": "Critical Assets SLA",
      "description": "Stricter SLA for business-critical systems",
      "critical_days": 3,
      "high_days": 14,
      "medium_days": 60,
      "low_days": 180,
      "is_default": false
    }
  ]
}
```

**Response** `207 Multi-Status`:
```json
{
  "created_count": 2,
  "results": [
    { "name": "Standard SLA",       "status": "created", "id": "sla-uuid-1" },
    { "name": "Critical Assets SLA", "status": "created", "id": "sla-uuid-2" }
  ]
}
```

---

### 2.2 [HIGH] `POST /api/v2/sla-configurations/assign-bulk` — Bulk assign SLA to products

Gán SLA configuration cho nhiều products trong một request.

**Gateway**:
```
POST /api/v2/sla-configurations/assign-bulk  →  sla-service:8086  (adminOnly)
```

**Request body**:
```json
{
  "assignments": [
    { "product_id": "product-uuid-1", "sla_configuration_id": "sla-uuid-1" },
    { "product_id": "product-uuid-2", "sla_configuration_id": "sla-uuid-2" },
    { "product_id": "product-uuid-3", "sla_configuration_id": "sla-uuid-1" }
  ]
}
```

**Response** `207 Multi-Status`.

---

### 2.3 [HIGH] `POST /api/v2/notification-rules/bulk` — Bulk create notification rules

**Gateway**:
```
POST /api/v2/notification-rules/bulk  →  notification-service:8087  (authenticated)
```

**Request body**:
```json
{
  "rules": [
    {
      "product_id": "product-uuid-1",
      "finding_added": ["email", "slack"],
      "sla_breach": ["email", "slack", "webhook"],
      "finding_status_changed": ["inapp"]
    },
    {
      "product_id": "product-uuid-2",
      "finding_added": ["email"],
      "sla_breach": ["email"]
    }
  ]
}
```

**Response** `207 Multi-Status`.

---

### 2.4 [HIGH] `POST /api/v2/subscriptions/bulk` — Bulk create alert subscriptions

**Gateway**:
```
POST /api/v2/subscriptions/bulk  →  notification-service:8087  (authenticated)
```

**Request body**:
```json
{
  "subscriptions": [
    { "type": "vendor",  "value": "apache",       "min_severity": "HIGH" },
    { "type": "vendor",  "value": "microsoft",    "min_severity": "CRITICAL" },
    { "type": "product", "value": "log4j",        "min_severity": "HIGH",     "min_epss": 0.5 },
    { "type": "kev",     "value": "",             "min_severity": "MEDIUM" }
  ]
}
```

**Response** `207 Multi-Status`:
```json
{
  "created_count": 4,
  "results": [
    { "type": "vendor", "value": "apache",    "status": "created", "id": "sub-uuid-1" },
    { "type": "vendor", "value": "microsoft", "status": "created", "id": "sub-uuid-2" },
    { "type": "product","value": "log4j",     "status": "created", "id": "sub-uuid-3" },
    { "type": "kev",    "value": "",           "status": "created", "id": "sub-uuid-4" }
  ]
}
```

---

### 2.5 [HIGH] `POST /api/v2/jira-configurations/bulk` — Bulk create JIRA configs

**Gateway**:
```
POST /api/v2/jira-configurations/bulk  →  jira-service:8088  (adminOnly)
```

**Request body**:
```json
{
  "configurations": [
    {
      "product_id": "product-uuid-1",
      "url": "https://company.atlassian.net",
      "username": "jira-bot@company.com",
      "api_token": "JIRA_API_TOKEN_PLAIN",
      "project_key": "SEC",
      "issue_type_id": "10001",
      "push_notes": true,
      "push_all_issues": false,
      "enable_deduplication": true,
      "priority_mapping": {
        "Critical": "Highest",
        "High": "High",
        "Medium": "Medium",
        "Low": "Low"
      }
    }
  ]
}
```

> **Security**: `api_token` được mã hóa AES-256-GCM trước khi lưu; không bao giờ trả về plaintext sau khi tạo.

**Response** `207 Multi-Status`.

---

### 2.6 [MEDIUM] `POST /api/v2/webhooks/bulk` — Bulk create webhooks

**Gateway**:
```
POST /api/v2/webhooks/bulk  →  notification-service:8087  (authenticated)
```

**Request body**:
```json
{
  "webhooks": [
    {
      "url": "https://hooks.slack.com/services/T00/B00/xxx",
      "events": ["kev.new", "cve.new.critical"],
      "description": "Slack alerts for new KEV and critical CVEs"
    },
    {
      "url": "https://ci.company.com/hooks/security",
      "events": ["cve.epss.high", "cve.vendor"],
      "description": "CI pipeline trigger"
    }
  ]
}
```

**Response** `207 Multi-Status` — mỗi entry có `secret` (dùng để verify HMAC khi nhận webhook).

---

### 2.7 [MEDIUM] `PUT /api/v2/system-notification-rules` — Chuẩn hóa schema

Endpoint này dùng PUT (replace all). Cần document rõ schema để seed script có thể seed system-wide defaults.

**Gateway**:
```
PUT /api/v2/system-notification-rules  →  notification-service:8087  (adminOnly)
```

**Request body**:
```json
{
  "scan_added": ["email", "inapp"],
  "finding_added": ["inapp"],
  "sla_breach": ["email", "inapp"],
  "sla_expiring_soon": ["email", "inapp"],
  "risk_acceptance_expiration": ["email"],
  "product_added": ["inapp"]
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `POST /api/v2/sla-configurations/bulk` với 2 configs, 1 `is_default: true` → `207` với `created_count: 2`; sau đó `GET /api/v2/sla-configurations` trả về cả 2.
2. `POST /api/v2/sla-configurations/assign-bulk` với 3 products → `207` với `assigned_count: 3`; mỗi product khi tạo finding sẽ dùng đúng SLA config.
3. `POST /api/v2/notification-rules/bulk` với 5 rules → `207` với `created_count: 5`.
4. `POST /api/v2/subscriptions/bulk` với `type: "kev"` → `207`; user nhận notification khi có KEV mới.
5. `POST /api/v2/jira-configurations/bulk` với valid config → `207`; `api_token` không thể GET lại sau khi tạo.
6. `POST /api/v2/webhooks/bulk` với 2 webhooks → `207`; `GET /api/v2/webhooks` trả về 2 entries.
7. `PUT /api/v2/system-notification-rules` → `200`; `GET /api/v2/system-notification-rules` trả về đúng config đã set.
8. Tất cả bulk endpoints trả về `207` ngay cả khi một số items fail — không return `500` toàn bộ.
