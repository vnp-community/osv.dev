# TASK-P1-02 — Tạo `shared/styles/tokens.css`

**Phase:** 1 — Foundation  
**Nguồn giải pháp:** [`solutions/13_14_15_16_misc_and_setup.md` — Solution 15](../solutions/13_14_15_16_misc_and_setup.md)  
**Ưu tiên:** 🔴 PHẢI làm trước Phase 5-03  
**Phụ thuộc:** Không có

---

## Mục tiêu

Tập trung tất cả màu sắc lặp lại (`#0B1020`, `#151B2F`, `#E5E7EB`, ...) vào CSS custom properties (design tokens) để tránh magic values.

---

## Vấn đề hiện tại

Cùng một màu được lặp lại ở hàng chục component:
```typescript
// ❌ Lặp lại trong 40+ component
style={{ background: '#0B1020' }}
style={{ color: '#E5E7EB' }}
style={{ border: '1px solid rgba(255,255,255,0.07)' }}
```

---

## Việc cần làm

### 1. Tạo file CSS tokens
**File:** `src/shared/styles/tokens.css`

```css
/* Design tokens — tập trung tất cả màu sắc lặp lại */
:root {
  /* Background */
  --color-bg-page: #0B1020;
  --color-bg-card: #151B2F;
  --color-bg-sidebar: #0F1629;
  --color-bg-hover: rgba(255, 255, 255, 0.02);
  --color-bg-input: rgba(255, 255, 255, 0.05);

  /* Borders */
  --color-border-subtle: rgba(255, 255, 255, 0.07);
  --color-border-input: rgba(255, 255, 255, 0.1);
  --color-border-section: rgba(255, 255, 255, 0.06);

  /* Text */
  --color-text-primary: #E5E7EB;
  --color-text-secondary: #9CA3AF;
  --color-text-muted: #6B7280;
  --color-text-faint: #4B5563;
  --color-text-disabled: #374151;

  /* Brand */
  --color-primary: #4F8CFF;
  --color-primary-dark: #3B6FCC;
  --color-primary-bg: rgba(79, 140, 255, 0.1);
  --color-primary-border: rgba(79, 140, 255, 0.3);

  /* Severity */
  --color-severity-critical: #EF4444;
  --color-severity-critical-bg: rgba(239, 68, 68, 0.1);
  --color-severity-high: #F97316;
  --color-severity-high-bg: rgba(249, 115, 22, 0.15);
  --color-severity-medium: #EAB308;
  --color-severity-medium-bg: rgba(234, 179, 8, 0.15);
  --color-severity-low: #3B82F6;
  --color-severity-low-bg: rgba(59, 130, 246, 0.15);

  /* Status */
  --color-status-success: #10B981;
  --color-status-success-bg: rgba(16, 185, 129, 0.1);
  --color-status-warning: #F59E0B;
  --color-status-warning-bg: rgba(245, 158, 11, 0.1);
  --color-status-error: #EF4444;
  --color-status-error-bg: rgba(239, 68, 68, 0.1);
  --color-status-info: #4F8CFF;
  --color-status-info-bg: rgba(79, 140, 255, 0.1);

  /* AI */
  --color-ai: #A78BFA;
  --color-ai-bg: rgba(167, 139, 250, 0.1);

  /* Charts */
  --color-chart-grid: rgba(255, 255, 255, 0.05);
  --color-chart-tooltip-bg: #1E2A45;
  --color-chart-tooltip-border: rgba(255, 255, 255, 0.1);
}
```

### 2. Import vào global styles
Thêm import vào `src/index.css` hoặc `src/App.css`:
```css
@import './shared/styles/tokens.css';
```

---

## Tiêu chí hoàn thành

- [x] File `src/shared/styles/tokens.css` tồn tại với đầy đủ tokens (70+ variables)
- [x] File được import trong `src/styles/index.css` (dòng đầu tiên)
- [x] Tokens hiển thị đúng trong browser DevTools (`:root` có các variables)

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`src/styles/tokens.css`](../../../../ui/src/styles/tokens.css) — 70+ CSS custom properties
- [`src/styles/index.css`](../../../../ui/src/styles/index.css) — thêm `@import './tokens.css';`

**Tokens đã có:**
- Background (6): `--color-bg-page`, `--color-bg-card`, `--color-bg-sidebar`, ...
- Borders (4): `--color-border-subtle`, `--color-border-input`, ...
- Text (5): `--color-text-primary`, `--color-text-secondary`, ...
- Brand (5): `--color-primary`, `--color-primary-dark`, ...
- Severity (8): `--color-severity-critical`, `--color-severity-high`, ...
- Status (10): `--color-status-success`, `--color-status-warning`, ...
- AI, Charts, Roles, Misc


---

## Ghi chú

> Task TASK-P5-03 sẽ dùng file này để migrate inline hex sang CSS variables trên toàn codebase.  
> Các task Phase 2–4 có thể vừa tạo code mới vừa dùng inline style (chấp nhận được cho task đó), migration sẽ được làm trong Phase 5.
