# F06 — Product Hierarchy: Data Flow

---

## 1. Tạo Product và Hierarchy

```
Client → POST /api/v2/products {product_type_id, name, description}
    │
    ▼
finding-service:
    1. Validate product_type_id tồn tại
    2. INSERT products record
    3. Auto-add creator as product member với role='owner'
    4. Publish NATS: audit.product.created
    │
    ▼
Client ← 201 {product_id, name, grade: 'A' (mặc định, chưa có findings)}

Client → POST /api/v2/engagements {product_id, name, type, start_date, end_date}
    │
    ▼
finding-service:
    1. Validate: user có quyền trong product (maintainer/owner/admin)
    2. INSERT engagements, state='Draft'
    │
    ▼
Client ← 201 {engagement_id}

Client → POST /api/v2/tests {engagement_id, name, tool}
    │
    ▼
finding-service:
    1. Validate: user có quyền trong engagement's product
    2. INSERT tests
    │
    ▼
Client ← 201 {test_id}
```

---

## 2. Product Grade Calculation Flow

```
Client → GET /api/v2/products/{id}/grade
    │
    ▼
finding-service:
    Check Redis cache: osv:grade:{product_id}
    [HIT] → return cached grade
    [MISS] →
        SELECT severity, COUNT(*) FROM findings
        WHERE product_id=$1 AND state='Active'
        GROUP BY severity
        │
        ▼
        Apply grading algorithm → Grade A/B/C/D/F
        SET Redis cache TTL 5min
    │
    ▼
Client ← 200 {grade, breakdown: {critical, high, medium, low, total}}
```

---

## 3. Product Dashboard Aggregation

```
Client → GET /api/v2/products/{id}
    │
    ▼
finding-service (parallel queries):
    Query 1: Product details + member count
    Query 2: Active findings count by severity
    Query 3: Engagements list (last 5, sorted by date)
    Query 4: Grade calculation
    Query 5: SLA breaches count
    │
    ▼
Merge results
Client ← 200 {
    product,
    grade: "B",
    findings_summary: {critical: 0, high: 3, medium: 12, ...},
    sla_breached: 2,
    recent_engagements: [...]
}
```

---

## 4. Member Management Flow

```
Client → POST /api/v2/products/{id}/members {user_id, role}
    │
    ▼
finding-service:
    1. Verify requester is owner or admin
    2. GET /api/v1/users/{user_id} → identity-service (validate user exists)
    3. INSERT product_members
    4. Publish NATS: audit.product.member_added
    │
    ▼
Client ← 201 {product_id, user_id, role}
```

---

## 5. NATS Events

| Event | Trigger | Payload |
|-------|---------|---------|
| `audit.product.created` | Product tạo mới | {product_id, name, created_by} |
| `audit.product.member_added` | Member được thêm | {product_id, user_id, role} |
| `audit.product.member_removed` | Member bị xóa | {product_id, user_id} |
| `audit.engagement.created` | Engagement tạo mới | {engagement_id, product_id} |
| `audit.engagement.completed` | Engagement hoàn thành | {engagement_id, finding_count} |

Tất cả events trên đều được **audit-service** subscribe để ghi vào append-only audit log.
