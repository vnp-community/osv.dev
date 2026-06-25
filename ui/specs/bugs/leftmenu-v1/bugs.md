# Left Menu — Báo cáo lỗi đầy đủ (v1)

Rà soát toàn bộ các item trong [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx) đối chiếu với [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx).

**Tổng số item kiểm tra:** 49 items (bao gồm mục cha, mục con và quick links)
**Tổng số item bị lỗi:** 18 items

---

## Nhóm BUG-01: Mục cha không có điều hướng

**Mô tả:** Các mục cha có `children` nhưng ID của chúng không tồn tại trong `SECTION_TO_PATH`. Khi click, hàm `handleNavigate(id)` nhận được `path = undefined` nên không gọi `navigate()`. Nút chỉ đóng/mở sub-menu, không load bất kỳ trang nào.

| Label | ID | Đường dẫn mong muốn |
|---|---|---|
| Vulnerability Intel | `vuln-intel` | Chưa xác định |
| Active Scanning | `scanning` | Chưa xác định |
| Findings | `findings` | `/findings` (trang tổng hợp) |
| Assets | `assets` | `/assets` |
| Product Security | `product-security` | `/products` |
| AI Center | `ai-center` | Chưa xác định |
| Reports | `reports` | `/reports` |
| Integrations | `integrations` | Chưa xác định |
| Administration | `admin` | Chưa xác định |

---

## Nhóm BUG-02: Item con bị thiếu hoàn toàn trong SECTION_TO_PATH

**Mô tả:** Item được khai báo trong `navItems` (có hiển thị trên giao diện) nhưng ID lại **không có** trong `SECTION_TO_PATH`. Khi click, `path = undefined` → `navigate()` không được gọi → nút hoàn toàn không phản hồi.

| Label | ID | Thuộc mục cha |
|---|---|---|
| Mitigated | `mitigated` | Findings |

---

## Nhóm BUG-03: Item con trỏ sai đường dẫn (Wrong Route)

**Mô tả:** Item có cấu hình URL trong `SECTION_TO_PATH` nhưng URL đó không khớp với route thực tế trong `router.tsx`, dẫn đến lỗi 404 hoặc tải sai trang.

| Label | ID | Path cấu hình | Route thực tế | Vấn đề |
|---|---|---|---|---|
| Nmap Results | `nmap-results` | `/scans/latest/results/nmap` | `/scans/:id/results/nmap` | Dùng literal `"latest"` thay vì ID thực. Route yêu cầu `:id` cụ thể → 404 |
| ZAP Results | `zap-results` | `/scans/latest/results/zap` | `/scans/:id/results/zap` | Dùng literal `"latest"` thay vì ID thực. Route yêu cầu `:id` cụ thể → 404 |
| Asset Detail | `asset-detail` | `/assets` | `/assets/:id` | Trỏ về trang danh sách thay vì trang chi tiết. Đây là bản sao của Asset Inventory |
| Jira | `jira` | `/integrations/webhooks` | ❌ Không có route Jira | Trỏ nhầm sang trang Webhooks. Không có route `/integrations/jira` trong router.tsx |

---

## Nhóm BUG-04: Trùng lặp đường dẫn (Duplicate Route — Giả liệt nút)

**Mô tả:** Nhiều items trong cùng một nhóm được trỏ về **cùng một URL** mà không có query/params phân biệt. Khi người dùng đã đứng ở URL đó và click item khác trong nhóm, URL không thay đổi → router không re-render → trải nghiệm như nút bị liệt.

### Nhóm Findings (`/findings`):
| Label | ID | Path |
|---|---|---|
| All Findings | `all-findings` | `/findings` ← URL gốc |
| **Active** | `active-findings` | `/findings` ← **TRÙNG**, nên có `?status=active` |

### Nhóm Product Security (`/products`):
| Label | ID | Path |
|---|---|---|
| Products | `products` | `/products` ← URL gốc |
| **Engagements** | `engagements` | `/products` ← **TRÙNG**, nên có route riêng `/engagements` |
| **Security Scorecards** | `scorecards` | `/products` ← **TRÙNG**, nên có route riêng `/scorecards` |

### Nhóm AI Center:
| Label | ID | Path |
|---|---|---|
| AI Triage Queue | `ai-triage` | `/ai/triage` ← URL gốc |
| **AI Insights** | `ai-insights` | `/ai/triage` ← **TRÙNG**, nên có route riêng `/ai/insights` |

### Nhóm Reports (`/reports`):
| Label | ID | Path |
|---|---|---|
| Executive Reports | `exec-reports` | `/reports` ← URL gốc |
| **Technical Reports** | `tech-reports` | `/reports` ← **TRÙNG**, nên có route riêng hoặc tab |
| **Compliance Reports** | `compliance-reports` | `/reports` ← **TRÙNG**, nên có route riêng hoặc tab |

---

## Nhóm BUG-05: Dữ liệu rác / Orphan IDs

**Mô tả:** Các ID tồn tại trong code nhưng không khớp giữa các phần (`SECTION_CHILDREN` vs `navItems` vs `SECTION_TO_PATH`), có thể gây bug về highlight active state.

| ID | Tồn tại trong | Vấn đề |
|---|---|---|
| `false-positive` | `SECTION_CHILDREN.findings` | Không có trong `navItems` và không có trong `SECTION_TO_PATH` |
| `running-scan-detail` | `SECTION_CHILDREN.scanning` | Không có trong `navItems` và không có trong `SECTION_TO_PATH` |
| `finding-detail` | `SECTION_TO_PATH` + `SECTION_CHILDREN` | Không có trong `navItems` (không hiển thị), nhưng lại được map sang `/findings` — sẽ gây lỗi highlight nhầm khi ở `/findings/:id` |
| `security-settings` | `SECTION_TO_PATH` + `SECTION_CHILDREN.admin` | Không có trong `navItems` (không hiển thị), nhưng trỏ về `/admin/settings` — trùng với `system-settings` |

---

## Tổng hợp số lượng lỗi

| Nhóm | Số lượng item bị ảnh hưởng |
|---|---|
| BUG-01: Mục cha thiếu điều hướng | 9 |
| BUG-02: Item con thiếu trong SECTION_TO_PATH | 1 |
| BUG-03: Trỏ sai đường dẫn | 4 |
| BUG-04: Trùng lặp đường dẫn | 8 |
| BUG-05: Orphan ID / Dữ liệu rác | 4 |
| **Tổng** | **26 sự cố** |
