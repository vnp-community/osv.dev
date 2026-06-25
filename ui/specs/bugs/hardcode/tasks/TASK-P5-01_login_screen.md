# TASK-P5-01 — Fix `LoginScreen.tsx` → Public Stats API + Vite Build Version

**Phase:** 5 — Minor Fixes  
**Nguồn giải pháp:** [`solutions/13_14_15_16_misc_and_setup.md` — Solution 13](../solutions/13_14_15_16_misc_and_setup.md)  
**Ưu tiên:** 🟢 Minor  
**Phụ thuộc:** TASK-P1-03

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/auth/components/LoginScreen.tsx
const stats = [
  { label: 'CVEs Tracked', value: '240K+' },   // hardcode
  { label: 'Scans Today', value: '1,847' },       // hardcode
  { label: 'Uptime SLA', value: '99.99%' },       // hardcode
];
const threatIndicators = [
  { label: 'Critical Threats', count: 14 },  // hardcode
  { label: 'KEV Active', count: 7 },          // hardcode
  { label: 'Assets At Risk', count: 23 },     // hardcode
];
// "OSV Platform v3.2.1" hardcode — version không bao giờ cập nhật
```

---

## API Endpoint (Public — không cần auth)

```
GET /api/v2/public/stats → Platform-wide stats
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/auth/hooks/usePublicStats.ts`

```typescript
import axios from 'axios';  // Plain axios — không dùng apiClient (không có auth header)

export interface PublicStats {
  totalCVEs: string;
  scansToday: number;
  findingAccuracy: string;
  uptimeSLA: string;
  threatIndicators: {
    criticalThreats: number;
    kevActive: number;
    assetsAtRisk: number;
  };
}

export function usePublicStats() {
  return useQuery<PublicStats>({
    queryKey: ['public', 'stats'],
    queryFn: async () => {
      const { data } = await axios.get<PublicStats>('/api/v2/public/stats');
      return data;
    },
    staleTime: 5 * 60_000,
    retry: false,  // Login page — không retry nếu backend down
  });
}
```

### [MODIFY] `vite.config.ts` — thêm build version

```typescript
export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(process.env.npm_package_version ?? '0.0.0'),
  },
  // ...
});
```

### [MODIFY] `src/vite-env.d.ts` — thêm type declaration

```typescript
declare const __APP_VERSION__: string;
```

### [MODIFY] `src/features/auth/components/LoginScreen.tsx`

**Xóa:**
```typescript
const stats = [...]
const threatIndicators = [...]
// "OSV Platform v3.2.1"
```

**Thêm:**
```typescript
import { usePublicStats } from '../hooks/usePublicStats';

const { data: publicStats } = usePublicStats();

// Stats với fallback "—" khi chưa load
const stats = publicStats ? [
  { label: 'CVEs Tracked', value: publicStats.totalCVEs },
  { label: 'Scans Today', value: publicStats.scansToday.toLocaleString() },
  { label: 'Findings Accuracy', value: publicStats.findingAccuracy },
  { label: 'Uptime SLA', value: publicStats.uptimeSLA },
] : [
  { label: 'CVEs Tracked', value: '—' },
  { label: 'Scans Today', value: '—' },
  { label: 'Findings Accuracy', value: '—' },
  { label: 'Uptime SLA', value: '—' },
];

// Version từ build env
<p>OSV Platform v{__APP_VERSION__} · © {new Date().getFullYear()} OSV Security Inc.</p>
```

### [MODIFY] `src/mocks/handlers/auth.handlers.ts` — thêm public stats

```typescript
http.get('/api/v2/public/stats', () => {
  return HttpResponse.json({
    totalCVEs: '240K+',
    scansToday: 1847,
    findingAccuracy: '98.4%',
    uptimeSLA: '99.99%',
    threatIndicators: {
      criticalThreats: 14,
      kevActive: 7,
      assetsAtRisk: 23,
    },
  });
}),
```

---

## Tiêu chí hoàn thành

- [x] `features/auth/hooks/usePublicStats.ts` tạo xong (plain axios, retry: false)
- [x] `LoginScreen.tsx` không còn hardcode `const stats = [...]`, `const threatIndicators = [...]`
- [x] Stats hiển thị "—" khi chưa load (graceful fallback với `?? '—'`)
- [x] Version string lấy từ `__APP_VERSION__` (Vite define) với fallback '0.0.0'
- [x] `vite.config.ts` có `__APP_VERSION__` define từ `process.env.npm_package_version`
- [x] `src/vite-env.d.ts` khai báo `declare const __APP_VERSION__: string`
- [x] MSW public stats handler tại `/api/v2/public/stats` (trong auth.handlers.ts)
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/auth/hooks/usePublicStats.ts`](../../../../ui/src/features/auth/hooks/usePublicStats.ts) — [NEW]
- [`features/auth/components/LoginScreen.tsx`](../../../../ui/src/features/auth/components/LoginScreen.tsx) — [MODIFY]
- [`vite.config.ts`](../../../../ui/vite.config.ts) — [MODIFY] +define.__APP_VERSION__
- [`src/vite-env.d.ts`](../../../../ui/src/vite-env.d.ts) — [NEW] type declaration
- [`mocks/handlers/auth.handlers.ts`](../../../../ui/src/mocks/handlers/auth.handlers.ts) — [MODIFY] +public stats handler

---

## Kiểm tra

```bash
# Mở http://localhost:3000/login
# 1. Stats hiển thị: "240K+", "1,847", "98.4%", "99.99%"
# 2. Threat indicators: Critical Threats 14, KEV Active 7, Assets At Risk 23
# 3. Footer: "OSV Platform v0.0.0" (lấy từ package.json version)
# 4. Tắt MSW (VITE_ENABLE_MSW=false) → stats hiển thị "—" (không crash)
```
