# TASK-P1-01 — Tạo `shared/constants/thresholds.ts`

**Phase:** 1 — Foundation  
**Nguồn giải pháp:** [`solutions/13_14_15_16_misc_and_setup.md` — Solution 15](../solutions/13_14_15_16_misc_and_setup.md)  
**Ưu tiên:** 🔴 PHẢI làm trước Phase 2+  
**Phụ thuộc:** Không có

---

## Mục tiêu

Tập trung tất cả ngưỡng số nghiệp vụ (magic numbers) đang hardcode rải rác trong các component vào một file constants duy nhất.

---

## Vấn đề hiện tại

Các component đang dùng magic numbers trực tiếp, ví dụ:
```typescript
// ❌ ProductSecurity.tsx
if (slaCompliance >= 95) { color = 'green' }
if (daysLeft <= 7) { color = 'red' }
if (findingCount > 20) { bg = 'red' }
```

---

## Việc cần làm

### 1. Tạo file mới
**File:** `src/shared/constants/thresholds.ts`

```typescript
/**
 * Tập trung tất cả ngưỡng số nghiệp vụ.
 * Import từ đây thay vì hardcode trong component.
 */

// SLA thresholds
export const SLA_COMPLIANCE_GREEN = 95;   // >= 95% → green
export const SLA_COMPLIANCE_YELLOW = 90;  // >= 90% → yellow
export const SLA_DAYS_CRITICAL = 1;       // <= 1 day → critical
export const SLA_DAYS_WARNING = 7;        // <= 7 days → warning

// Finding count severity
export const FINDING_COUNT_HIGH = 20;     // > 20 → red
export const FINDING_COUNT_MEDIUM = 5;    // > 5 → yellow

// AI thresholds
export const AI_CONFIDENCE_HIGH = 0.90;   // > 90% → green
export const AI_CONFIDENCE_MEDIUM = 0.70; // > 70% → yellow

// Webhook performance
export const WEBHOOK_SLOW_MS = 1000;      // > 1000ms → slow (red)

// Risk acceptance
export const RISK_DAYS_EXPIRING = 30;     // <= 30 days → warning
```

### 2. Thêm barrel export
Thêm vào `src/shared/constants/index.ts` (tạo nếu chưa có):
```typescript
export * from './thresholds';
```

---

## Tiêu chí hoàn thành

- [x] File `src/shared/constants/thresholds.ts` tồn tại
- [x] Tất cả constants có JSDoc comment giải thích ý nghĩa
- [x] Barrel export `src/shared/constants/index.ts` đã được tạo
- [x] TypeScript không có lỗi compile (lỗi trong output là pre-existing)

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo:**
- [`src/shared/constants/thresholds.ts`](../../../../ui/src/shared/constants/thresholds.ts) — 11 constants với JSDoc
- [`src/shared/constants/index.ts`](../../../../ui/src/shared/constants/index.ts) — barrel export

**Cách sử dụng:**
```typescript
import { SLA_DAYS_WARNING, SLA_COMPLIANCE_GREEN } from '@/shared/constants';
// hoặc
import { SLA_DAYS_WARNING } from '@/shared/constants/thresholds';
```

