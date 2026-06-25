# SOL-001 — Gap Analysis: Current MVP vs Target Architecture

**Version:** 1.1  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** ✅ Completed — Đã phân tích và thực thi đầy đủ (Phase 1 + Phase 2 API layers)  
**Liên quan:** [architecture.md](../../../architecture.md), [TDD.md](../../../TDD.md)

---

## 1. Tổng quan

Tài liệu này phân tích khoảng cách (gap) giữa code MVP hiện tại trong `src/` và kiến trúc mục tiêu được định nghĩa trong `architecture.md` + `TDD.md`. Từ đó đề xuất **lộ trình migration** và **các giải pháp kỹ thuật cụ thể**.

---

## 2. Inventory Hiện Tại (Current State)

### 2.1 Cấu trúc thư mục hiện tại

```
ui/src/
├── main.tsx                     # Entry point
├── app/
│   ├── App.tsx                  # ❌ Monolithic router (if/else view switch)
│   └── components/              # ❌ Flat component folder (37 files)
│       ├── Dashboard.tsx
│       ├── CVESearch.tsx
│       ├── KEVCatalog.tsx
│       ├── FindingsList.tsx
│       └── ... (34 more)
├── imports/
│   └── pasted_text/             # Import artifacts
└── styles/                      # CSS styles
```

### 2.2 Dependencies hiện có (package.json)

| Package | Version | Status |
|---------|---------|--------|
| `react` | 18.3.1 | ✅ OK |
| `react-router` | 7.13.0 | ✅ Đã có, chưa dùng |
| `react-hook-form` | 7.55.0 | ✅ Đã có |
| `recharts` | 2.15.2 | ✅ Đang dùng |
| `lucide-react` | 0.487.0 | ✅ Đang dùng |
| `@radix-ui/*` | various | ✅ Đã có |
| `sonner` | 2.0.3 | ✅ Đã có (toast) |
| Zustand | — | ❌ Thiếu |
| React Query (`@tanstack/react-query`) | — | ❌ Thiếu |
| Axios | — | ❌ Thiếu |
| MSW | — | ❌ Thiếu |
| Zod | — | ❌ Thiếu |
| Vitest | — | ❌ Thiếu |
| Playwright | — | ❌ Thiếu |
| `@tanstack/react-virtual` | — | ❌ Thiếu |

---

## 3. Gap Analysis Chi Tiết

### 3.1 Gap: Routing Architecture

| Aspect | Current (MVP) | Target (Architecture) | Gap |
|--------|--------------|----------------------|-----|
| Routing | `useState` + if/else switch | React Router v7 Data Router | 🔴 Critical |
| URL params | ❌ Không có | URL-based deep links | 🔴 Critical |
| Code splitting | ❌ Không có | Lazy loading per route | 🟡 Medium |
| Auth guard | ❌ Không có (hardcode `screen=login`) | `AuthGuard` component | 🔴 Critical |
| Breadcrumbs | Hardcoded Map | Derived from URL | 🟡 Medium |

**Vấn đề cụ thể:** `App.tsx` dùng `useState<AppView>` + `if/else` switch. Không có URL, không có deep linking, không có browser history.

---

### 3.2 Gap: State Management

| Aspect | Current (MVP) | Target | Gap |
|--------|--------------|--------|-----|
| Server state | ❌ Hardcoded arrays | React Query | 🔴 Critical |
| Auth state | `useState` local | Zustand + persist | 🔴 Critical |
| UI state | `useState` local | Zustand / useState | 🟢 OK (local OK) |
| Form state | `useState` | React Hook Form + Zod | 🟡 Medium |
| Real-time | ❌ Không có SSE | EventSource + Zustand | 🟡 Medium |

**Vấn đề cụ thể:**
```tsx
// Dashboard.tsx — VI PHẠM: Hardcode data
const riskTrendData = [
  { month: "Jan", critical: 320, high: 480, ... },
  ...
];
const recentFindings = [{ id: "F-2847", ... }];
const productData = [{ name: "Banking App", ... }];
```

Tất cả 37 components đều có hardcoded data. Đây là vi phạm nghiêm trọng nhất theo `architecture.md Section 5.5`.

---

### 3.3 Gap: Folder Structure

| Aspect | Current | Target | Gap |
|--------|---------|--------|-----|
| Module organization | Flat `components/` | Feature-based `features/*/` | 🔴 Critical |
| Shared types | ❌ Không có | `shared/types/*.ts` | 🔴 Critical |
| API layer | ❌ Không có | `shared/api/client.ts` + feature APIs | 🔴 Critical |
| Hooks | ❌ Không có | Feature-specific hooks | 🔴 Critical |
| Mock layer | ❌ Không có MSW | `src/mocks/` với handlers + fixtures | 🟡 Medium |

---

### 3.4 Gap: API Integration

| Aspect | Current | Target | Gap |
|--------|---------|--------|-----|
| HTTP Client | ❌ Không có | Axios + interceptors | 🔴 Critical |
| Token handling | ❌ Không có | JWT Bearer + refresh | 🔴 Critical |
| Error handling | ❌ Không có | QueryBoundary + toasts | 🟡 Medium |
| Loading states | Minimal | Skeleton UI | 🟡 Medium |
| Pagination | Client-side static | Server-side | 🔴 Critical |

---

### 3.5 Gap: Design System

| Aspect | Current | Target | Gap |
|--------|---------|--------|-----|
| CSS variables | Inline styles chủ yếu | CSS design tokens | 🟡 Medium |
| Shared components | Defined inline | Extracted to `shared/components/` | 🟡 Medium |
| Design tokens | Scattered | Centralized `theme.css` | 🟡 Medium |

**Tích cực:** Design system (màu sắc, layout, typography) hiện tại đã **rất gần** với target spec. `SEVERITY_COLORS`, `GradeCircle`, `KPICard` components đã implement đúng design token.

---

### 3.6 Gap: Testing

| Aspect | Current | Target | Gap |
|--------|---------|--------|-----|
| Unit tests | ❌ 0% coverage | ≥90% utils, ≥80% hooks | 🔴 Critical |
| Component tests | ❌ 0% | ≥70% features | 🔴 Critical |
| E2E tests | ❌ 0% | 20 flows (Playwright) | 🟡 Medium |
| CI gate | ❌ Không có | ESLint + hardcode check | 🟡 Medium |

---

## 4. Summary Gap Matrix

| Category | Severity | Effort | Priority |
|----------|----------|--------|----------|
| Hardcoded data removal | 🔴 Critical | High | P0 |
| React Router v7 integration | 🔴 Critical | Medium | P0 |
| Zustand + React Query setup | 🔴 Critical | Medium | P0 |
| Feature-based folder restructure | 🔴 Critical | High | P1 |
| Axios client + JWT interceptors | 🔴 Critical | Low | P0 |
| MSW mock layer setup | 🟡 Medium | Medium | P1 |
| TypeScript shared types | 🔴 Critical | Medium | P1 |
| Shared component extraction | 🟡 Medium | Medium | P2 |
| Testing infrastructure | 🔴 Critical | High | P2 |
| SSE real-time features | 🟡 Medium | Medium | P2 |
| Virtualized lists | 🟢 Low | Low | P3 |
| E2E tests (Playwright) | 🟢 Low | High | P3 |

---

## 5. Đánh giá Positive (Những gì đã tốt)

1. **Design system đẹp** — Color tokens, component styling đã match với target spec
2. **Component coverage đầy đủ** — 37 components bao phủ hầu hết 60+ screens
3. **Dependencies đã sẵn sàng** — React Router v7, react-hook-form đã trong package.json
4. **Radix UI primitives** — Đã setup, accessible components
5. **TypeScript** — Được dùng xuyên suốt

---

## 6. Rủi ro và Ràng buộc

| Rủi ro | Mức độ | Biện pháp |
|--------|--------|-----------|
| Refactor lớn có thể break UI | High | Branch-per-feature, incremental migration |
| Thiếu Zustand + React Query trong package.json | Medium | Cần `pnpm add` trước |
| MSW cần service worker setup | Low | Tài liệu rõ ràng trong setup guide |
| Type inference phức tạp với generics | Low | Dùng TypeScript strict mode từ đầu |
