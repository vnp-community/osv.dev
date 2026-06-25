# F10 — Notifications: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Notification Routing

### 1.1 Event → Channel Mapping

Khi notification-service nhận NATS event, nó phải quyết định gửi cho ai qua channel nào:

```
handleEvent(event):
    1. Identify event_type từ event payload
    2. Find notification_configs WHERE event_type matches AND product_id matches
    3. For each matching config:
        recipients = getRecipients(config, event)
        for channel in config.channels:
            deliver(channel, recipients, event)
```

### 1.2 Recipient Resolution

```
getRecipients(config, event):
    if config.recipient_type == "product_members":
        return getUserEmails(product_id, roles=[owner, maintainer])
    elif config.recipient_type == "specific_users":
        return config.user_emails
    elif config.recipient_type == "channel_only":
        return []  // Chỉ Slack/Teams, không cần email list
```

---

## 2. Channel Delivery Logic

### 2.1 Email Delivery

```
sendEmail(recipients[], template, data):
    subject = renderTemplate(template.subject, data)
    body    = renderTemplate(template.body_html, data)
    for email in recipients:
        smtp.Send(from=config.from_addr, to=email, subject, body)
        if error: log warning, continue
```

### 2.2 Slack / Teams Delivery

```
sendSlack(webhook_url, event_data):
    payload = buildSlackMessage(event_data):
        {
            text: "🚨 SLA Breached: CVE-2021-44228 in Payment Gateway",
            attachments: [{
                color: severityColor(finding.severity),
                fields: [{title: "Severity", value: "Critical"}, ...]
            }]
        }
    HTTP POST webhook_url, payload
    if error: log, không retry cho Slack (fire-and-forget)
```

### 2.3 In-app Notification

```
createInAppNotification(user_id, event):
    INSERT notifications {
        user_id, event_type, title, body,
        reference_type, reference_id,
        is_read=false, created_at=NOW()
    }
    
// User poll qua:
GET /api/v2/notifications?unread=true
→ list unread notifications

POST /api/v2/notifications/{id}/read
→ UPDATE notifications SET is_read=true, read_at=NOW()
```

### 2.4 Webhook Delivery với Retry

```
deliverWebhook(webhook_config, payload_bytes):
    signature = HMAC-SHA256(webhook_config.secret, payload_bytes)
    
    for attempt in [1, 2, 3]:
        try:
            HTTP POST webhook_config.url
                Headers: {
                    Content-Type: application/json,
                    X-OSV-Signature: sha256={signature},
                    X-OSV-Event: {event_type},
                    X-OSV-Delivery: {delivery_id}
                }
                Body: payload_bytes
                Timeout: 10s
            
            if response.status in [200..299]:
                log success
                return
            else:
                log warning, wait exponential_backoff(attempt), retry
        except timeout:
            log warning, wait, retry
    
    log error("webhook delivery failed after 3 attempts")
    INSERT webhook_delivery_failures record
```

---

## 3. SSRF Protection

Trước khi gửi webhook, kiểm tra URL:

```
validateWebhookURL(url):
    parsed = parseURL(url)
    
    // Chỉ cho phép HTTPS (production)
    if parsed.scheme != "https":
        return error("only HTTPS webhooks allowed")
    
    // Resolve DNS
    ip = DNS.resolve(parsed.host)
    
    // Block private ranges
    privateRanges = [
        "10.0.0.0/8",
        "172.16.0.0/12",
        "192.168.0.0/16",
        "127.0.0.0/8",
        "169.254.0.0/16",  // link-local
        "::1"              // IPv6 loopback
    ]
    
    for range in privateRanges:
        if ip IN range:
            return error("SSRF protection: private IP blocked")
    
    return ok
```

---

## 4. Notification Config Rules

### 4.1 Config structure

```
notification_config {
    product_id:      string (nullable = global config)
    event_types:     [kev.new, finding.sla.breached, ...]
    channels:        [email, slack, webhook]
    slack_url:       string (nếu channel=slack)
    webhook_id:      string (nếu channel=webhook)
    recipient_type:  product_members | specific_users
    min_severity:    Critical | High | null (filter events by severity)
}
```

### 4.2 Severity filter

```
Nếu config.min_severity = "High":
    Chỉ deliver nếu event.finding.severity IN [High, Critical]
    Bỏ qua Medium, Low, Info findings
```

---

## 5. Event Template System

Mỗi event_type có template riêng:

```
templates = {
    "finding.sla.breached": {
        subject: "⚠️ SLA Breached: {finding.title}",
        body: """
            Finding {finding.id} in product {product.name} has breached its SLA.
            Severity: {finding.severity}
            Deadline was: {finding.sla_expiration_date}
            Days overdue: {days_overdue}
            Link: {platform_url}/findings/{finding.id}
        """
    },
    "kev.new": {
        subject: "🚨 New CISA KEV: {event.cve_id}",
        body: """
            CVE {event.cve_id} has been added to the CISA Known Exploited Vulnerabilities catalog.
            Product: {event.product}
            Vendor: {event.vendor}
            Date Added: {event.date_added}
            Ransomware: {event.is_ransomware}
        """
    }
}
```
