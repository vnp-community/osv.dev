# TASK-P1-03 — Cấu trúc thư mục MSW

**Phase:** 1 — Foundation  
**Nguồn giải pháp:** [`solutions/13_14_15_16_misc_and_setup.md` — Solution 16](../solutions/13_14_15_16_misc_and_setup.md)  
**Ưu tiên:** 🔴 PHẢI làm trước mọi task Phase 2+  
**Phụ thuộc:** Không có

---

## Mục tiêu

Thiết lập (hoặc chuẩn hóa) cấu trúc thư mục `src/mocks/` và file `handlers/index.ts` tổng hợp để sẵn sàng đăng ký các handler mới từ Phase 2+.

---

## Cấu trúc thư mục mục tiêu

```
src/mocks/
├── browser.ts                   # MSW worker setup (có thể đã có)
├── handlers/
│   ├── index.ts                 # Export tổng hợp — CẦN CẬP NHẬT
│   ├── admin.handlers.ts        # [NEW] Users, RBAC, Settings — tạo ở P2
│   ├── ai.handlers.ts           # [NEW] AI Triage — tạo ở P3
│   ├── audit.handlers.ts        # [NEW] Audit logs — tạo ở P2
│   ├── auth.handlers.ts         # [EXISTS] Giữ nguyên
│   ├── cve.handlers.ts          # [EXISTS] Giữ nguyên
│   ├── findings.handlers.ts     # [NEW/UPDATE] Risk Acceptances — tạo ở P3
│   ├── integrations.handlers.ts # [NEW] API Keys, Webhooks — tạo ở P3/P4
│   ├── notifications.handlers.ts# [NEW] Notifications — tạo ở P3
│   ├── products.handlers.ts     # [EXISTS/UPDATE] Products — tạo ở P5
│   ├── reports.handlers.ts      # [NEW] Reports — tạo ở P4
│   └── scan.handlers.ts         # [EXISTS/UPDATE] Scans, Stats — update ở P4
└── fixtures/                    # [NEW] Tách fixtures ra file riêng
    ├── users.fixture.ts
    ├── audit.fixture.ts
    ├── rbac.fixture.ts
    ├── ai-triage.fixture.ts
    ├── api-keys.fixture.ts
    ├── notifications.fixture.ts
    └── reports.fixture.ts
```

---

## Việc cần làm

### 1. Kiểm tra trạng thái hiện tại
```bash
ls src/mocks/
ls src/mocks/handlers/
```

### 2. Tạo `src/mocks/handlers/index.ts` (cập nhật nếu có)

```typescript
// src/mocks/handlers/index.ts
// Import handlers đã có
// (Bỏ comment khi file tương ứng được tạo trong các task tiếp theo)

// import { adminUserHandlers } from './admin.handlers';
// import { auditHandlers } from './audit.handlers';
// import { aiHandlers } from './ai.handlers';
// import { notificationHandlers } from './notifications.handlers';
// import { integrationHandlers } from './integrations.handlers';
// import { riskAcceptanceHandlers } from './findings.handlers';
// import { reportHandlers } from './reports.handlers';

// Handlers đã có (giữ nguyên)
// import { authHandlers } from './auth.handlers';
// import { cveHandlers } from './cve.handlers';
// import { scanHandlers } from './scan.handlers';

export const handlers = [
  // Bổ sung dần theo từng task:
  // ...adminUserHandlers,
  // ...auditHandlers,
  // ...aiHandlers,
  // ...notificationHandlers,
  // ...integrationHandlers,
  // ...riskAcceptanceHandlers,
  // ...reportHandlers,

  // Handlers hiện có
  // ...authHandlers,
  // ...cveHandlers,
  // ...scanHandlers,
];
```

### 3. Tạo thư mục `src/mocks/fixtures/` (nếu chưa có)

### 4. Cập nhật `src/main.tsx` — MSW init (nếu chưa có)

```typescript
async function enableMocking() {
  if (import.meta.env.VITE_ENABLE_MSW !== 'true') return;
  const { worker } = await import('./mocks/browser');
  return worker.start({ onUnhandledRequest: 'warn' });
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(<App />);
});
```

### 5. Kiểm tra `.env.development`

```env
# .env.development
VITE_ENABLE_MSW=true
```

---

## Tiêu chí hoàn thành

- [x] `src/mocks/handlers/index.ts` tồn tại và đã có đủ 12 handlers đăng ký
- [x] `src/mocks/fixtures/` directory tồn tại (auth, cves, dashboard, findings, scans đã có)
- [x] `src/main.tsx` có MSW init đúng pattern (đã có sẵn)
- [x] `.env.development` có `VITE_ENABLE_MSW=true` (đã có sẵn)
- [x] App vẫn chạy được (không thay đổi cấu trúc core)

---

## ✅ Đã hoàn thành — 2026-06-19 (cấu trúc đã đầy đủ)

**Phát hiện:** Cấu trúc MSW đã được setup tốt hơn task spec — không cần thay đổi gì thêm:

**Handlers hiện có (14 files):**
- `admin.handlers.ts`, `ai.handlers.ts`, `asset.handlers.ts`, `auth.handlers.ts`
- `cve.handlers.ts`, `dashboard.handlers.ts`, `finding.handlers.ts`
- `index.ts`, `integration.handlers.ts`, `kev.handlers.ts`
- `notification.handlers.ts`, `product.handlers.ts`, `report.handlers.ts`, `scan.handlers.ts`

**Fixtures hiện có (5 files):**
- `auth.fixture.ts`, `cves.fixture.ts`, `dashboard.fixture.ts`, `findings.fixture.ts`, `scans.fixture.ts`

**main.tsx** đã có MSW enableMocking() đúng pattern.  
**.env.development** đã có `VITE_ENABLE_MSW=true`.

> Fixtures bổ sung được tạo trong TASK-P1-04.

