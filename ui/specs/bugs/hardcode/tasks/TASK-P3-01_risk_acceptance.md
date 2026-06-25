# TASK-P3-01 — Fix `RiskAcceptanceCenter.tsx` → `useRiskAcceptances()`

**Phase:** 3 — Core Features  
**Nguồn giải pháp:** [`solutions/05_risk_acceptance.md`](../solutions/05_risk_acceptance.md)  
**Ưu tiên:** 🟡 Core — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/findings/components/RiskAcceptanceCenter.tsx
const acceptances = [
  { id: 'RA-012', severity: 'High', daysLeft: 92, isExpired: false, ... },
  // 5 risk acceptance giả
];
// Approve/Reject button không có tác dụng
```

---

## API Endpoints

```
GET    /api/v1/risk-acceptances         → Danh sách (ENDPOINTS.riskAcceptances.list)
POST   /api/v1/risk-acceptances         → Tạo mới
DELETE /api/v1/risk-acceptances/:id     → Revoke
```

---

## Danh sách files cần tạo/sửa

### [MODIFY] `src/shared/types/finding.ts` — thêm RiskAcceptance type

```typescript
export interface RiskAcceptance {
  id: string;
  productId: string;
  productName: string;
  findingIds: string[];
  findingTitle: string;
  cveId?: string;
  severity: Severity;
  expirationDate: string;
  retestDate?: string;
  reason: string;
  approvedBy: string;
  approvedById: string;
  isExpired: boolean;
  daysLeft: number;
  createdAt: string;
}

export interface RiskAcceptancesResponse {
  acceptances: RiskAcceptance[];
  total: number;
}
```

### [NEW] `src/features/findings/hooks/useRiskAcceptances.ts`

```typescript
export function useRiskAcceptances(params?: { productId?: string; status?: 'active' | 'expired' }) {
  return useQuery<RiskAcceptancesResponse>({
    queryKey: ['risk-acceptances', 'list', params],
    queryFn: async () => {
      const { data } = await apiClient.get<RiskAcceptancesResponse>(
        ENDPOINTS.riskAcceptances.list, { params }
      );
      return data;
    },
    staleTime: 60_000,
  });
}

export function useRevokeRiskAcceptance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.delete(ENDPOINTS.riskAcceptances.delete(id)),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['risk-acceptances'] }); },
  });
}
```

### [MODIFY] `src/features/findings/components/RiskAcceptanceCenter.tsx`

Xem code đầy đủ tại: [`solutions/05_risk_acceptance.md`](../solutions/05_risk_acceptance.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `const acceptances = [...]`
- Dùng `useRiskAcceptances({ status: filter })`
- Stats cards (active, expiring, expired) tính từ server data
- "Revoke" button thực sự gọi DELETE mutation
- Filter all/active/expired gửi params lên API

### [MODIFY] `src/shared/api/endpoints.ts` — thêm riskAcceptances

```typescript
riskAcceptances: {
  list:   '/api/v1/risk-acceptances',
  create: '/api/v1/risk-acceptances',
  delete: (id: string) => `/api/v1/risk-acceptances/${id}`,
},
```

### [NEW] `src/mocks/handlers/findings.handlers.ts`

```typescript
export const riskAcceptanceHandlers = [
  http.get('/api/v1/risk-acceptances', ...),
  http.delete('/api/v1/risk-acceptances/:id', ...),
];
```

Xem data fixture đầy đủ tại: [`solutions/05_risk_acceptance.md`](../solutions/05_risk_acceptance.md) — mục "MSW Handler"

---

## Tiêu chí hoàn thành

- [x] `features/findings/hooks/useRiskAcceptances.ts` tạo xong (GET + PATCH approve/reject + DELETE + POST create)
- [x] `RiskAcceptanceCenter.tsx` không còn `const acceptances = [...]`
- [x] Filter all/pending/approved/expired gửi `status` param lên API
- [x] Approve/Reject button gọi PATCH mutation và refresh list
- [x] formatExpiration() từ ISO date string (không hardcode)
- [x] getDaysLeft() tính realtime từ ISO date
- [x] Loading spinner + error state với Retry button
- [x] MSW handler: import từ risk-acceptances.fixture.ts, mutable raStore
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/findings/hooks/useRiskAcceptances.ts`](../../../../ui/src/features/findings/hooks/useRiskAcceptances.ts) — [NEW] 3 hooks
- [`features/findings/components/RiskAcceptanceCenter.tsx`](../../../../ui/src/features/findings/components/RiskAcceptanceCenter.tsx) — [MODIFY] Refactored
- [`mocks/handlers/finding.handlers.ts`](../../../../ui/src/mocks/handlers/finding.handlers.ts) — [MODIFY] RA section → {items, total}, PATCH handler, import fixture


---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Risk Acceptance Center
# 1. Verify: acceptances từ MSW (RA-012, RA-011, RA-009)
# 2. Stats: 2 Active, 1 Expiring soon (RA-011 có 15 days left), 1 Expired
# 3. Filter "expired" → chỉ thấy RA-009
# 4. Click "Revoke" trên RA-012 → disappears từ list
```
