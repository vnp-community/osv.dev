# Giải pháp Fix Lỗi Left Menu (leftmenu-v1)

**Trạng thái tổng thể:** ✅ **HOÀN THÀNH** — 2026-06-22  
**Tasks liên quan:** [`tasks/README.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/README.md) — 7/7 tasks ✅ DONE

**Nguồn tham chiếu:**
- Bugs: [`bugs.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/bugs.md)
- Kiến trúc: [`architecture.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/architecture.md) — Section 4.3 (Routing), 4.2 (State Strategy), 7.1 (Code Splitting)
- Files đã sửa: [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx), [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx)

---

## FIX-01: BUG-01 — Thêm điều hướng cho các mục cha

**Trạng thái:** ✅ DONE ([TASK-01](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-01-parent-nav.md))

### Vấn đề
9 mục cha (Parent Nodes) không được khai báo trong `SECTION_TO_PATH`. Click vào chỉ toggle expand/collapse, không điều hướng.

### Giải pháp đã thực thi
Thêm 9 entries vào `SECTION_TO_PATH` trong [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx):

```typescript
// ── Parent node navigation defaults (TASK-01) ──────────────────────────
"vuln-intel":       "/cve/search",
"scanning":         "/scans",
"findings":         "/findings",
"assets":           "/assets",
"product-security": "/products",
"ai-center":        "/ai/triage",
"reports":          "/reports",
"integrations":     "/integrations/api-keys",
"admin":            "/admin/users",
```

> [!NOTE]
> Theo architecture.md Section 15, `Route definitions` là static config và được phép dùng literal trong code. Không vi phạm API-First rule.

---

## FIX-02: BUG-02 — Thêm route cho "Mitigated"

**Trạng thái:** ✅ DONE ([TASK-02](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-02-mitigated-route.md))

### Vấn đề
Item `Mitigated` (`id: "mitigated"`) có mặt trong `navItems` nhưng không tồn tại trong `SECTION_TO_PATH` → click không phản hồi.

### Giải pháp đã thực thi
**Option A — URL Query Parameter** đã được áp dụng.

Sửa `SECTION_TO_PATH` trong [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx):
```typescript
"active-findings": "/findings?status=active",    // đã sửa từ "/findings"
"mitigated":       "/findings?status=mitigated", // mới thêm
```

[`FindingsList.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/findings/components/FindingsList.tsx) đã có sẵn `useSearchParams` đọc `?status=` tại dòng 89 — **không cần sửa thêm**.

> [!TIP]
> **Option A được áp dụng** vì đồng nhất với cơ chế filter đang dùng (URL state), dễ bookmark, dễ deep link, và không cần tạo thêm route mới.

---

## FIX-03: BUG-03 — Sửa các item trỏ sai đường dẫn

### 3a. Nmap Results & ZAP Results

**Trạng thái:** ✅ DONE ([TASK-03](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-03-nmap-zap-redirect.md))

**Option B — Redirect sang scan mới nhất** đã được áp dụng.

**Files đã tạo mới:**
- [`useLatestScan.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/scanning/hooks/useLatestScan.ts) — hook `GET /api/v1/scans?sort_by=-created_at&page_size=1`
- [`LatestNmapRedirect.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/scanning/components/LatestNmapRedirect.tsx) — resolve scanId → redirect `/scans/:id/results/nmap`
- [`LatestZAPRedirect.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/scanning/components/LatestZAPRedirect.tsx) — resolve scanId → redirect `/scans/:id/results/zap`

**Routes đã thêm vào [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx):**
```typescript
{ path: '/scans/latest/nmap', element: <P><LatestNmapRedirect /></P> },
{ path: '/scans/latest/zap',  element: <P><LatestZAPRedirect /></P> },
```

**`SECTION_TO_PATH` đã sửa:**
```typescript
"nmap-results": "/scans/latest/nmap",
"zap-results":  "/scans/latest/zap",
```

> [!TIP]
> **Option B được áp dụng** vì UX tốt hơn — người dùng click "Nmap Results" và được tự động redirect đến kết quả của scan mới nhất.

---

### 3b. Asset Detail

**Trạng thái:** ✅ DONE ([TASK-04](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-04-asset-detail.md))

**Option A — Xóa khỏi sidebar** đã được áp dụng.

Đã xóa `{ id: "asset-detail", label: "Asset Detail" }` khỏi `navItems.assets.children` trong [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx). Giữ lại `"asset-detail"` trong `SECTION_CHILDREN.assets` để `/assets/:id` vẫn highlight đúng mục cha "Assets".

---

### 3c. Jira

**Trạng thái:** ✅ DONE ([TASK-05](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-05-jira-route.md))

**Files đã tạo / sửa:**
- **Mới:** [`JiraConfig.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/integrations/components/JiraConfig.tsx) — form cấu hình Jira URL, Project Key, API Token, User Email
- **Sửa [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx):** thêm lazy import `JiraConfig` + route `/integrations/jira`
- **Sửa `SECTION_TO_PATH`:** `jira: "/integrations/jira"` (thay vì `/integrations/webhooks`)

> [!NOTE]
> MSW handler cho `GET/PUT /api/v1/integrations/jira` chưa tạo — cần thêm vào `src/mocks/handlers/` khi backend chưa sẵn sàng.

---

## FIX-04: BUG-04 — Sửa trùng lặp đường dẫn bằng URL query params

**Trạng thái:** ✅ DONE ([TASK-06](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-06-duplicate-routes.md))

### 4a. Nhóm Findings
Đã sửa trong cùng TASK-02. `active-findings` và `mitigated` đều dùng query param `?status=`.

### 4b. Nhóm Product Security

**`SECTION_TO_PATH` đã sửa:**
```typescript
engagements: "/products?tab=engagements",
scorecards:  "/products?tab=scorecards",
```

**[`ProductSecurity.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/product-security/components/ProductSecurity.tsx) đã cập nhật:**
```typescript
const [searchParams] = useSearchParams();
const urlTab = searchParams.get("tab");
const activeTab = urlTab === "engagements" ? "Engagements"
                : urlTab === "scorecards"  ? "Risk Acceptance"
                : "Engagements"; // default
```

### 4c. Nhóm AI Center

**Option B — Route riêng** đã được áp dụng.

- **Mới:** [`AIInsights.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/ai-center/components/AIInsights.tsx) — dashboard KPIs, weekly triage trend chart, top CWE bar chart, AI recommendations
- **Route mới:** `{ path: '/ai/insights', element: <P><AIInsights /></P> }`
- **`SECTION_TO_PATH`:** `"ai-insights": "/ai/insights"`

### 4d. Nhóm Reports

**`SECTION_TO_PATH` đã sửa:**
```typescript
"exec-reports":       "/reports?type=executive",
"tech-reports":       "/reports?type=technical",
"compliance-reports": "/reports?type=compliance",
```

**[`ReportCenter.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/reports/components/ReportCenter.tsx) đã cập nhật:**
```typescript
const [searchParams] = useSearchParams();
const urlType = searchParams.get("type");
const typeFilter = urlType === "executive"  ? "Executive"
                 : urlType === "technical"  ? "Technical"
                 : urlType === "compliance" ? "Compliance" : null;
// typeFilter được dùng để filter + highlight template cards
```

---

## FIX-05: BUG-05 — Dọn dẹp Orphan IDs

**Trạng thái:** ✅ DONE ([TASK-07](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/tasks/TASK-07-cleanup-orphan-ids.md))

Tất cả orphan IDs đã được dọn dẹp trong [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx):

| Item | Thay đổi | Kết quả |
|---|---|---|
| `false-positive` | Xóa khỏi `SECTION_CHILDREN.findings` | ✅ |
| `running-scan-detail` | Xóa khỏi `SECTION_CHILDREN.scanning` | ✅ |
| `finding-detail` | Xóa khỏi `SECTION_TO_PATH` (giữ trong `SECTION_CHILDREN` để highlight `/findings/:id`) | ✅ |
| `security-settings` | Xóa khỏi cả `SECTION_TO_PATH` và `SECTION_CHILDREN.admin` | ✅ |
| `asset-detail` | Xóa khỏi `SECTION_TO_PATH` (giữ trong `SECTION_CHILDREN` để highlight `/assets/:id`) | ✅ |

---

## Tóm tắt thực thi

### Files đã sửa

| File | Thay đổi |
|---|---|
| [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx) | `SECTION_TO_PATH` (+9 parent paths, fix 10 wrong paths, remove duplicates), `navItems` (xóa asset-detail), `SECTION_CHILDREN` (cleanup 4 orphan IDs) |
| [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx) | +5 lazy imports + 5 routes mới: `/scans/latest/nmap`, `/scans/latest/zap`, `/integrations/jira`, `/ai/insights` |
| [`ProductSecurity.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/product-security/components/ProductSecurity.tsx) | Thêm `useSearchParams`, sync tab với `?tab=` URL param |
| [`ReportCenter.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/reports/components/ReportCenter.tsx) | Thêm `useSearchParams`, filter + highlight template theo `?type=` URL param |

### Files mới tạo

| File | Mô tả |
|---|---|
| [`JiraConfig.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/integrations/components/JiraConfig.tsx) | Trang cấu hình tích hợp Jira — form + API hook |
| [`AIInsights.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/ai-center/components/AIInsights.tsx) | Dashboard AI Insights — KPIs, charts, recommendations |
| [`LatestNmapRedirect.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/scanning/components/LatestNmapRedirect.tsx) | Redirect sang Nmap results của scan mới nhất |
| [`LatestZAPRedirect.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/scanning/components/LatestZAPRedirect.tsx) | Redirect sang ZAP results của scan mới nhất |
| [`useLatestScan.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/scanning/hooks/useLatestScan.ts) | Hook lấy scan mới nhất từ API |

### Trạng thái từng FIX

| Fix | Mô tả | Trạng thái |
|---|---|---|
| FIX-01 | 9 mục cha thiếu path điều hướng | ✅ DONE |
| FIX-02 | `Mitigated` không có path + `Active` trỏ sai | ✅ DONE |
| FIX-03a | Nmap/ZAP path sai → 404 | ✅ DONE |
| FIX-03b | Asset Detail trong sidebar → xóa | ✅ DONE |
| FIX-03c | Jira trỏ nhầm Webhooks | ✅ DONE |
| FIX-04a | Findings: Active ≡ All Findings | ✅ DONE |
| FIX-04b | Product Security: Engagements/Scorecards trùng Products | ✅ DONE |
| FIX-04c | AI Center: AI Insights trùng AI Triage | ✅ DONE |
| FIX-04d | Reports: 3 loại report trùng nhau | ✅ DONE |
| FIX-05 | Orphan IDs: false-positive, running-scan-detail, security-settings | ✅ DONE |

> [!NOTE]
> **Việc còn lại:** MSW mock handlers cho `GET/PUT /api/v1/integrations/jira` và `GET /api/v1/ai/insights` cần tạo riêng khi backend chưa triển khai. Tham khảo `architecture.md` Section 5.6 về cách tạo MSW handler.
