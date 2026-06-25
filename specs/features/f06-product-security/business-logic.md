# F06 — Product Hierarchy: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Hierarchy Model

### 1.1 Cấu trúc phân cấp

Toàn bộ hệ thống quản lý bảo mật được tổ chức theo cấu trúc phân cấp 5 lớp:

```
ProductType (phân loại: Web App, Mobile, API, Infrastructure...)
    └── Product (ứng dụng/hệ thống cụ thể, vd: "Payment Gateway")
            ├── ProductMember (người dùng + role trong product này)
            └── Engagement (đợt đánh giá bảo mật, vd: "Q2 2026 Pentest")
                    └── Test (một buổi scan/test cụ thể, vd: "ZAP Scan #1")
                            └── Finding (lỗ hổng được phát hiện)
```

**Quy tắc toàn vẹn:**
- Finding PHẢI có test_id → test PHẢI có engagement_id → engagement PHẢI có product_id
- Xóa product → cascade delete tất cả engagements, tests, findings liên quan

### 1.2 Engagement Types

| Type | Mô tả |
|------|-------|
| Pentest | Manual penetration testing |
| Internal | Internal security review |
| External | External security assessment |
| Web Application | Dedicated web app assessment |
| CI/CD | Automated pipeline scan |

---

## 2. RBAC Per Product

### 2.1 Product Member Roles

Ngoài global roles (`admin`, `user`, `readonly`), mỗi Product có RBAC riêng qua bảng `product_members`:

```
Khi user truy cập /api/v2/products/{id}/...:
    1. Lấy X-User-ID từ header (injected bởi gateway)
    2. Query: SELECT role FROM product_members WHERE product_id=$1 AND user_id=$2
    3. Nếu không tìm thấy AND user không phải admin toàn hệ thống → 403 Forbidden
    4. Product member role override global role (lấy effective_role = max(global, product))
```

### 2.2 Product Member Roles

| Product Role | Quyền trong product |
|-------------|---------------------|
| `owner` | Tất cả, kể cả xóa product và quản lý members |
| `maintainer` | CRUD findings, engagements, tests; invite members |
| `developer` | Tạo/update findings, xem reports |
| `viewer` | Chỉ xem |

---

## 3. Product Grading

### 3.1 Khi nào tính grade

Grade được tính **real-time** khi có request `GET /products/{id}/grade`, không lưu cố định. Tuy nhiên, có thể cache trong Redis với TTL ngắn (5 phút) để tránh query nặng.

### 3.2 Thuật toán

```
calculateGrade(product_id):
    active_findings = COUNT findings WHERE product_id=$1 AND state='Active' GROUP BY severity

    critical = active_findings['Critical'] or 0
    high     = active_findings['High'] or 0
    total    = SUM of all active findings

    if critical >= 3 OR total > 20: return 'F'
    if critical in [1,2]:           return 'D'
    if critical == 0, high > 5:     return 'C'
    if critical == 0, 1 <= high <= 5: return 'B'
    if critical == 0, high == 0:    return 'A'
```

### 3.3 Grade Impact

- Grade thay đổi ngay khi finding state thay đổi (vì tính real-time)
- Grade được hiển thị trên Dashboard và trong báo cáo
- Grade 'F' có thể trigger notification tới product owner (nếu configured)

---

## 4. Engagement Lifecycle

```
Engagement states: Draft → Active → Completed | Cancelled

Draft:     Mới tạo, chưa có scan nào
Active:    Đang trong quá trình đánh giá
Completed: Đánh giá kết thúc, report được generate
Cancelled: Hủy engagement

Transition rules:
    Draft → Active    (bắt đầu đánh giá)
    Active → Completed (kết thúc, phải có ít nhất 1 test)
    Active → Cancelled (hủy)
    Completed → Active (reopen, nếu cần re-test)
```

---

## 5. Member Management

### 5.1 Add Member

```
POST /api/v2/products/{id}/members {user_id, role}

Rules:
    1. Requester phải là product owner hoặc system admin
    2. Validate: user_id tồn tại trong identity-service
    3. Check: user chưa là member của product này
    4. INSERT product_members {product_id, user_id, role, invited_by, invited_at}
    5. Publish NATS: audit.product.member_added
```

### 5.2 Remove Member

```
DELETE /api/v2/products/{id}/members/{userId}

Rules:
    1. Không thể xóa người cuối cùng có role 'owner'
    2. Không thể tự xóa mình nếu là owner duy nhất
    3. DELETE product_members WHERE product_id=$1 AND user_id=$2
```
