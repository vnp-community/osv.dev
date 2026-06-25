# TASK-P4-02 — Fix `ScanDashboard.tsx` → `useScanStats()` thay `Math.random()`

**Phase:** 4 — Scan & Report  
**Nguồn giải pháp:** [`solutions/06_07_scan_history_dashboard.md` — Solution 07](../solutions/06_07_scan_history_dashboard.md)  
**Ưu tiên:** 🔵 Scan  
**Phụ thuộc:** TASK-P1-03

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/scanning/components/ScanDashboard.tsx
const weeklyActivity = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map(day => ({
  day,
  scans: Math.floor(Math.random() * 10),      // ❌ Thay đổi mỗi render!
  findings: Math.floor(Math.random() * 100),   // ❌ Không phản ánh thực tế
}));
```

> **Vấn đề:** Chart data thay đổi mỗi khi component re-render → chart "nhảy lung tung" → gây confusion cho user.

---

## API Endpoints mới cần thêm

```
GET /api/v1/scans/stats         → KPI stats (running, completed_today, findings, scheduled)
GET /api/v1/scans/stats/weekly  → Weekly activity chart data (stable)
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/scanning/hooks/useScanStats.ts`

```typescript
export interface ScanStats {
  activeScans: number;
  completedToday: number;
  totalFindings: number;
  scheduledScans: number;
}

export interface WeeklyActivity {
  day: string;    // 'Mon', 'Tue', ...
  scans: number;
  findings: number;
}

export function useScanStats() {
  return useQuery<ScanStats>({
    queryKey: ['scans', 'stats'],
    queryFn: async () => {
      const { data } = await apiClient.get<ScanStats>('/api/v1/scans/stats');
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,  // Auto-refresh mỗi 30s
  });
}

export function useWeeklyScanActivity() {
  return useQuery<WeeklyActivity[]>({
    queryKey: ['scans', 'stats', 'weekly'],
    queryFn: async () => {
      const { data } = await apiClient.get<WeeklyActivity[]>('/api/v1/scans/stats/weekly');
      return data;
    },
    staleTime: 5 * 60_000,
  });
}
```

### [MODIFY] `src/features/scanning/components/ScanDashboard.tsx`

**Thay đổi chính:**
- Xóa `Math.random()` cho weeklyActivity
- Import `useScanStats`, `useWeeklyScanActivity`
- Weekly chart: `data={weeklyQuery.data ?? []}` (stable data từ server)
- KPI stats (active scans, completed today, ...) từ `statsQuery.data`

```typescript
import { useScanStats, useWeeklyScanActivity } from '../hooks/useScanStats';
import { useScans } from '../hooks/useScans';

export function ScanDashboard() {
  const statsQuery = useScanStats();
  const weeklyQuery = useWeeklyScanActivity();
  const runningScansQuery = useScans({ status: 'running' });
  // ...
}
```

### [MODIFY] `src/shared/api/endpoints.ts` — thêm scan stats

```typescript
scans: {
  // ... existing endpoints
  stats:       '/api/v1/scans/stats',
  statsWeekly: '/api/v1/scans/stats/weekly',
},
```

### [MODIFY] `src/mocks/handlers/scan.handlers.ts` — thêm stats handlers

```typescript
http.get('/api/v1/scans/stats', () => {
  return HttpResponse.json({
    activeScans: 2,
    completedToday: 8,
    totalFindings: 347,
    scheduledScans: 3,
  });
}),

http.get('/api/v1/scans/stats/weekly', () => {
  // Dữ liệu stable — không random
  return HttpResponse.json([
    { day: 'Mon', scans: 3, findings: 42 },
    { day: 'Tue', scans: 5, findings: 78 },
    { day: 'Wed', scans: 4, findings: 55 },
    { day: 'Thu', scans: 7, findings: 93 },
    { day: 'Fri', scans: 6, findings: 67 },
    { day: 'Sat', scans: 2, findings: 28 },
    { day: 'Sun', scans: 1, findings: 15 },
  ]);
}),
```

---

## Tiêu chí hoàn thành

- [x] `features/scanning/hooks/useScanStats.ts` tạo xong (useScanStats + useWeeklyScanActivity)
- [x] `ScanDashboard.tsx` không còn `Math.random()` cho weeklyActivity
- [x] Weekly chart dùng `weeklyQuery.data ?? []` (stable, không thay đổi khi re-render)
- [x] MSW handler `/api/v1/scans/stats` tính từ scansFixture (không hardcode)
- [x] MSW handler `/api/v1/scans/stats/weekly` trả về data tĩnh
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/scanning/hooks/useScanStats.ts`](../../../../ui/src/features/scanning/hooks/useScanStats.ts) — [NEW] 2 hooks
- [`features/scanning/components/ScanDashboard.tsx`](../../../../ui/src/features/scanning/components/ScanDashboard.tsx) — [MODIFY] Math.random() → useWeeklyScanActivity()
- [`mocks/handlers/scan.handlers.ts`](../../../../ui/src/mocks/handlers/scan.handlers.ts) — [MODIFY] +stats, +weekly handlers

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Scan Dashboard
# 1. KPI cards: "2 Active Scans", "8 Completed Today", "347 Findings"
# 2. Weekly chart hiển thị stable (Mon-Sun với data cố định)
# 3. Resize browser → chart vẫn hiển thị cùng values (không random)
# 4. Refresh page → chart values KHÔNG thay đổi
```
