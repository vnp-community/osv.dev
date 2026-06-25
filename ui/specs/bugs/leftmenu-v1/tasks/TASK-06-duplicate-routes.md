# TASK-06 — Fix Duplicate Routes bằng URL Query Params

**Priority:** 🟡 Medium  
**Effort:** ~90 phút  
**Loại thay đổi:** Config + nhiều Component updates  
**Trạng thái:** ✅ DONE — 2026-06-22

> **Ghi chú thực thi:**
> - `SECTION_TO_PATH` đã sửa từ session trước (query params cho engagements, scorecards, report types).
> - `ProductSecurity.tsx`: thêm `useSearchParams`, map `?tab=` → active tab nội bộ.
> - `ReportCenter.tsx`: thêm `useSearchParams`, lọc template theo `?type=`, highlight template được chọn.
> - Đã tạo `AIInsights.tsx` + route `/ai/insights` trong `router.tsx`.

## Mục tiêu

Nhiều menu items trong cùng nhóm được trỏ về cùng 1 URL → khi click từ item này sang item khác trong nhóm, URL không thay đổi, router không re-render, người dùng trải nghiệm như nút bị liệt.

**4 nhóm bị ảnh hưởng:**
- Findings: `Active` ≡ `All Findings` → cùng `/findings`
- Product Security: `Engagements`, `Security Scorecards` ≡ `Products` → cùng `/products`
- AI Center: `AI Insights` ≡ `AI Triage Queue` → cùng `/ai/triage`
- Reports: `Technical Reports`, `Compliance Reports` ≡ `Executive Reports` → cùng `/reports`

**Giải pháp:** Dùng URL query params (`?tab=...`) để phân biệt các views trong cùng một trang.

> [!NOTE]
> TASK-02 đã xử lý phần `active-findings` và `mitigated` trong Findings. Task này chỉ xử lý phần còn lại của Findings và 3 nhóm khác.

---

## Files cần sửa

| File | Thay đổi |
|---|---|
| [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx) | Sửa paths trong `SECTION_TO_PATH` |
| [`ProductSecurity.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/product-security/components/ProductSecurity.tsx) | Đọc `?tab=` query param |
| [`AITriage.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/ai-center/components/AITriage.tsx) | Đọc `?tab=` query param (hoặc tạo route riêng) |
| [`ReportCenter.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/reports/components/ReportCenter.tsx) | Đọc `?type=` query param |
| [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx) | Thêm route `/ai/insights` (nếu chọn Option B cho AI) |

---

## Thay đổi chi tiết

---

### 6a. Nhóm Product Security

#### Sửa `SECTION_TO_PATH` trong `Sidebar.tsx`:
```typescript
// Trước:
products:    "/products",
engagements: "/products",
scorecards:  "/products",

// Sau:
products:    "/products",
engagements: "/products?tab=engagements",
scorecards:  "/products?tab=scorecards",
```

#### Sửa `ProductSecurity.tsx`:
```typescript
import { useSearchParams } from 'react-router';

export function ProductSecurity() {
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get('tab') ?? 'products'; // 'products' | 'engagements' | 'scorecards'

  return (
    <div>
      {/* Tab navigation */}
      <div className="tabs">
        <button onClick={() => setSearchParams({ tab: 'products' })}
          data-active={activeTab === 'products'}>Products</button>
        <button onClick={() => setSearchParams({ tab: 'engagements' })}
          data-active={activeTab === 'engagements'}>Engagements</button>
        <button onClick={() => setSearchParams({ tab: 'scorecards' })}
          data-active={activeTab === 'scorecards'}>Security Scorecards</button>
      </div>

      {/* Conditional content */}
      {activeTab === 'products'     && <ProductsTab />}
      {activeTab === 'engagements'  && <EngagementsTab />}
      {activeTab === 'scorecards'   && <ScorecardsTab />}
    </div>
  );
}
```

---

### 6b. Nhóm AI Center

**Lựa chọn được khuyến nghị: Tạo route riêng `/ai/insights`**

#### Sửa `SECTION_TO_PATH` trong `Sidebar.tsx`:
```typescript
// Trước:
"ai-triage":   "/ai/triage",
"ai-enrichment": "/ai/enrichment",
"ai-insights": "/ai/triage",

// Sau:
"ai-triage":     "/ai/triage",
"ai-enrichment": "/ai/enrichment",
"ai-insights":   "/ai/insights",     // Route riêng biệt
```

#### Thêm vào `router.tsx`:
```typescript
// Lazy import
const AIInsights = lazy(() =>
  import('@/features/ai-center/components/AIInsights').then((m) => ({ default: m.AIInsights }))
);

// Route
{ path: '/ai/insights', element: <P><AIInsights /></P> },
```

#### Tạo `features/ai-center/components/AIInsights.tsx`:
```typescript
export function AIInsights() {
  // Hiển thị AI Insights dashboard — metrics, trends, recommendations
  // Dùng useAIInsights() hook để fetch data từ API
  return (
    <div>
      <h1>AI Insights</h1>
      {/* AI metrics, vulnerability trend analysis, recommendations */}
    </div>
  );
}
```

> **Cập nhật `SECTION_CHILDREN`** trong `Sidebar.tsx` để include `ai-insights`:
> ```typescript
> "ai-center": ["ai-triage", "ai-enrichment", "ai-insights"],
> // đã đúng — giữ nguyên
> ```

---

### 6c. Nhóm Reports

#### Sửa `SECTION_TO_PATH` trong `Sidebar.tsx`:
```typescript
// Trước:
"exec-reports":       "/reports",
"tech-reports":       "/reports",
"compliance-reports": "/reports",

// Sau:
"exec-reports":       "/reports?type=executive",
"tech-reports":       "/reports?type=technical",
"compliance-reports": "/reports?type=compliance",
```

#### Sửa `ReportCenter.tsx`:
```typescript
import { useSearchParams } from 'react-router';

export function ReportCenter() {
  const [searchParams, setSearchParams] = useSearchParams();
  const reportType = searchParams.get('type') ?? 'executive'; // 'executive' | 'technical' | 'compliance'

  return (
    <div>
      {/* Type navigation */}
      <div className="report-tabs">
        <button onClick={() => setSearchParams({ type: 'executive' })}
          data-active={reportType === 'executive'}>Executive Reports</button>
        <button onClick={() => setSearchParams({ type: 'technical' })}
          data-active={reportType === 'technical'}>Technical Reports</button>
        <button onClick={() => setSearchParams({ type: 'compliance' })}
          data-active={reportType === 'compliance'}>Compliance Reports</button>
      </div>

      {/* Report list filtered by type */}
      <ReportList type={reportType} />
    </div>
  );
}
```

---

## Acceptance Criteria

### Product Security
- [x] Click "Products" → `/products` (tab Products mặc định)
- [x] Click "Engagements" → `/products?tab=engagements` (tab Engagements active)
- [x] Click "Security Scorecards" → `/products?tab=scorecards` (tab Scorecards active)
- [x] Browser back/forward hoạt động đúng giữa các tabs
- [x] Sidebar highlight đúng item khi đang ở từng URL

### AI Center
- [x] Click "AI Triage Queue" → `/ai/triage`
- [x] Click "AI Enrichment" → `/ai/enrichment`
- [x] Click "AI Insights" → `/ai/insights` (không phải `/ai/triage`)
- [x] Route `/ai/insights` render đúng `AIInsights` component (không 404)

### Reports
- [x] Click "Executive Reports" → `/reports?type=executive`
- [x] Click "Technical Reports" → `/reports?type=technical`
- [x] Click "Compliance Reports" → `/reports?type=compliance`
- [x] `ReportCenter` hiển thị đúng nội dung theo `?type=` param
- [x] Default type là `executive` khi không có query param

---

## Ghi chú kỹ thuật

- `useSearchParams` từ `react-router` (v7) là hook reactive — component sẽ re-render khi URL query thay đổi mà không cần thêm logic.
- Khi implement `ReportList`, truyền `type` như một filter param vào hook: `useReports({ type: reportType })` → gọi `GET /api/v1/reports?type=executive`.
- Không hardcode data trong component. Các `Tab` component và `ReportList` phải dùng React Query hook.
