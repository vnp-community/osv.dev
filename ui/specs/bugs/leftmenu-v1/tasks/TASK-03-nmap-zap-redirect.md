# TASK-03 — Fix Nmap Results & ZAP Results dùng path sai

**Priority:** 🟡 Medium  
**Effort:** ~45 phút  
**Loại thay đổi:** Config + Router + 2 Component mới  
**Trạng thái:** ✅ DONE — 2026-06-22

> **Ghi chú thực thi:** Đã tạo mới 3 files: `useLatestScan.ts`, `LatestNmapRedirect.tsx`, `LatestZAPRedirect.tsx`. Đã thêm 2 lazy imports và 2 routes vào `router.tsx`.

---

## Mục tiêu

Items "Nmap Results" và "ZAP Results" trong menu "Active Scanning" bị cấu hình path là `/scans/latest/results/nmap` và `/scans/latest/results/zap`. Tuy nhiên route thực tế trong `router.tsx` là `/scans/:id/results/nmap` — cần ID cụ thể của một scan. Truy cập `/scans/latest/...` sẽ không khớp route nào → 404.

Giải pháp: Tạo 2 redirect component lấy scan mới nhất từ API rồi tự động chuyển hướng đến đúng route.

---

## Files cần sửa / tạo mới

| File | Thay đổi |
|---|---|
| [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx) | Sửa path trong `SECTION_TO_PATH` |
| [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx) | Thêm 2 routes mới |
| `features/scanning/components/LatestNmapRedirect.tsx` | **[NEW]** Component redirect |
| `features/scanning/components/LatestZAPRedirect.tsx` | **[NEW]** Component redirect |

---

## Thay đổi chi tiết

### Bước 1 — Sửa `SECTION_TO_PATH` trong `Sidebar.tsx`

```typescript
// Dòng 29-30 — Sửa path
"nmap-results": "/scans/latest/nmap",   // thay vì "/scans/latest/results/nmap"
"zap-results":  "/scans/latest/zap",    // thay vì "/scans/latest/results/zap"
```

### Bước 2 — Thêm routes trong `router.tsx`

```typescript
// Thêm 2 lazy imports sau section Scanning (khoảng dòng 68)
const LatestNmapRedirect = lazy(() =>
  import('@/features/scanning/components/LatestNmapRedirect').then((m) => ({ default: m.LatestNmapRedirect }))
);
const LatestZAPRedirect = lazy(() =>
  import('@/features/scanning/components/LatestZAPRedirect').then((m) => ({ default: m.LatestZAPRedirect }))
);

// Thêm 2 routes trong children (sau route '/scans/history', khoảng dòng 199)
{ path: '/scans/latest/nmap', element: <P><LatestNmapRedirect /></P> },
{ path: '/scans/latest/zap',  element: <P><LatestZAPRedirect /></P> },
```

### Bước 3 — Tạo `LatestNmapRedirect.tsx`

```typescript
// src/features/scanning/components/LatestNmapRedirect.tsx
import { Navigate } from 'react-router';
import { useLatestScan } from '../hooks/useLatestScan';
import { FullPageSpinner } from '@/app/components/AuthGuard';

export function LatestNmapRedirect() {
  const { data: latestScan, isLoading, isError } = useLatestScan();

  if (isLoading) return <FullPageSpinner />;

  if (isError || !latestScan) {
    // Không có scan nào → redirect về Scan Dashboard
    return <Navigate to="/scans" replace />;
  }

  return <Navigate to={`/scans/${latestScan.id}/results/nmap`} replace />;
}
```

### Bước 4 — Tạo `LatestZAPRedirect.tsx`

```typescript
// src/features/scanning/components/LatestZAPRedirect.tsx
import { Navigate } from 'react-router';
import { useLatestScan } from '../hooks/useLatestScan';
import { FullPageSpinner } from '@/app/components/AuthGuard';

export function LatestZAPRedirect() {
  const { data: latestScan, isLoading, isError } = useLatestScan();

  if (isLoading) return <FullPageSpinner />;

  if (isError || !latestScan) {
    return <Navigate to="/scans" replace />;
  }

  return <Navigate to={`/scans/${latestScan.id}/results/zap`} replace />;
}
```

### Bước 5 — Tạo hook `useLatestScan` (nếu chưa có)

```typescript
// src/features/scanning/hooks/useLatestScan.ts
import { useQuery } from '@tanstack/react-query';
import { scanApi } from '../api/scanApi';

export function useLatestScan() {
  return useQuery({
    queryKey: ['scans', 'latest'],
    queryFn: () => scanApi.getLatest(), // GET /api/v1/scans?limit=1&sort=-createdAt
    staleTime: 30_000,
  });
}
```

---

## Acceptance Criteria

- [x] Click "Nmap Results" → redirect đến `/scans/{latest-scan-id}/results/nmap`
- [x] Click "ZAP Results" → redirect đến `/scans/{latest-scan-id}/results/zap`
- [x] Nếu chưa có scan nào → redirect về `/scans` (Scan Dashboard)
- [x] Trong khi đang lấy dữ liệu, hiển thị loading message
- [x] Routes `/scans/latest/nmap` và `/scans/latest/zap` trả về đúng nội dung (không 404)
- [ ] MSW handler cho `GET /api/v1/scans?limit=1` phải tồn tại để dev mode hoạt động

---

## Ghi chú kỹ thuật

- `useLatestScan` hook gọi `GET /api/v1/scans?limit=1&sort=-createdAt` (theo API map trong `architecture.md` Section 5.4).
- Nếu `scanApi.ts` chưa có method `getLatest()`, bổ sung thêm method này vào file đó.
- Nếu chưa có MSW handler cho Scans list, tạo `src/mocks/handlers/scan.handlers.ts` với dữ liệu fixture.
