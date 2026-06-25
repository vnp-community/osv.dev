# F06 — Product & Engagement Hierarchy

> **Spec Folder:** `specs/features/f06-product-security/`  
> **Feature Doc:** [`docs/features/F06-product-security.md`](../../../docs/features/F06-product-security.md)  
> **SRS Refs:** FR-04-05  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Hierarchy rules, RBAC per product, grading, member management |
| [dataflow.md](./dataflow.md) | Hierarchy creation, member invite, grade calculation flows |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `finding-service` | 8085 | Product/Engagement/Test CRUD, member management, grading |
| `identity-service` | 8081 | User lookup for member assignment |
| `audit-service` | 8090 | Log hierarchy changes |

---

## Hierarchy

```
ProductType
    └── Product
            ├── ProductMember (user, role)
            └── Engagement
                    └── Test
                            └── Finding
```

Mỗi cấp có RBAC riêng. Findings luôn gắn với Test → Engagement → Product.

---

## Product Grade

| Grade | Điều kiện |
|-------|---------|
| **A** | 0 Critical, 0 High |
| **B** | 0 Critical, ≤ 5 High |
| **C** | 0 Critical, > 5 High |
| **D** | 1–2 Critical |
| **F** | ≥ 3 Critical hoặc > 20 active findings |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET/POST | `/api/v2/product-types` | Manage product types |
| GET/POST | `/api/v2/products` | List/create products |
| GET | `/api/v2/products/{id}` | Product detail + grade |
| POST | `/api/v2/products/{id}/members` | Add member |
| DELETE | `/api/v2/products/{id}/members/{userId}` | Remove member |
| GET/POST | `/api/v2/engagements` | Engagements |
| GET/POST | `/api/v2/tests` | Tests |

---

## Database Schema (`osv_findings`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `product_types` | id, name, description | Top-level grouping |
| `products` | id, product_type_id, name, description, grade | Product entity |
| `product_members` | product_id, user_id, role | RBAC per product |
| `engagements` | id, product_id, name, start_date, end_date, status | Engagement |
| `tests` | id, engagement_id, name, tool, import_type | Test/scan session |
