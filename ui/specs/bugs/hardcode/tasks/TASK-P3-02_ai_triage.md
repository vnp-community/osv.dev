# TASK-P3-02 — Fix `AITriage.tsx` → `useAITriageQueue()`

**Phase:** 3 — Core Features  
**Nguồn giải pháp:** [`solutions/08_ai_triage.md`](../solutions/08_ai_triage.md)  
**Ưu tiên:** 🟡 Core — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/ai-center/components/AITriage.tsx
const queue = [
  { id: 'F-2847', title: 'Apache Log4j2 JNDI RCE', verdict: 'Confirmed', confidence: 0.98, ... },
  // 6 triage items hardcode
];
const metrics = [
  { label: 'Accepted Today', value: 8 },  // hardcode
  { label: 'Avg Confidence', value: '92%' }, // hardcode
];
// Accept/Reject button không có tác dụng
```

---

## API Endpoints

```
GET  /api/v1/ai/triage/queue              → Queue với pagination + stats
POST /api/v1/ai/triage/:findingId/review  → Submit human decision
```
> Cả 2 endpoints đã có trong ENDPOINTS.ai.triageQueue và ENDPOINTS.ai.triageReview

---

## Danh sách files cần tạo/sửa

### [NEW/MODIFY] `src/features/ai-center/types.ts`

```typescript
export type AITriageRemarks = 'Confirmed' | 'FalsePositive' | 'NotAffected' | 'Unexplored';
export type HumanDecision = 'accepted' | 'overridden' | 'rejected';

export interface AITriageQueueItem {
  findingId: string;
  findingTitle: string;
  cveId?: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  aiResult: {
    remarks: AITriageRemarks;
    confidence: number;
    justification: string;
    actions: string[];
    generatedAt: string;
  };
  humanDecision?: HumanDecision;
  humanNote?: string;
  reviewedBy?: string;
  reviewedAt?: string;
}

export interface AITriageQueueResponse {
  items: AITriageQueueItem[];
  total: number;
  stats: {
    pending: number;
    acceptedToday: number;
    avgConfidence: number;
    falsePositiveRate: number;
  };
}

export interface ReviewTriageRequest {
  decision: HumanDecision;
  note?: string;
}
```

### [NEW] `src/features/ai-center/hooks/useAITriageQueue.ts`

```typescript
export function useAITriageQueue(params?: { status?: string; severity?: string; page?: number }) {
  return useQuery<AITriageQueueResponse>({
    queryKey: ['ai', 'triage', 'queue', params],
    queryFn: async () => {
      const { data } = await apiClient.get<AITriageQueueResponse>(
        ENDPOINTS.ai.triageQueue, { params }
      );
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,  // Auto-refresh
  });
}

export function useReviewTriage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ findingId, ...body }: { findingId: string } & ReviewTriageRequest) =>
      apiClient.post(ENDPOINTS.ai.triageReview(findingId), body),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['ai', 'triage'] }); },
  });
}
```

### [MODIFY] `src/features/ai-center/components/AITriage.tsx`

Xem code đầy đủ tại: [`solutions/08_ai_triage.md`](../solutions/08_ai_triage.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `const queue = [...]`, `const metrics = [...]`
- Import `useAITriageQueue`, `useReviewTriage`
- Stats (Pending, Accepted Today, Avg Confidence, FP Rate) từ `stats` object trong response
- Accept/Override/Reject buttons gọi `reviewTriage.mutateAsync()`
- Filter pending/accepted/overridden gửi `status` param lên API
- `refetchInterval: 30_000` — tự động refresh queue

### [NEW] `src/mocks/handlers/ai.handlers.ts`

```typescript
export const aiHandlers = [
  http.get('/api/v1/ai/triage/queue', ...),
  http.post('/api/v1/ai/triage/:findingId/review', ...),
];
```

Xem data fixture và handler đầy đủ tại: [`solutions/08_ai_triage.md`](../solutions/08_ai_triage.md) — mục "MSW Handler"

---

## Tiêu chí hoàn thành

- [x] `features/ai-center/hooks/useAITriage.ts` tạo xong (GET queue + POST review)
- [x] `AITriage.tsx` không còn `const queue = [...]`
- [x] Stats metrics (pending, accepted_today, avg_confidence) hiển thị từ server
- [x] Accept/Reject buttons gọi `reviewTriage.mutate()` + invalidate
- [x] Sau khi review, item chuyển sang trạng thái mới (mutable triageStore)
- [x] formatTimeAgo() từ ISO string (không hardcode "10m ago")
- [x] MSW handler: 6 items, filter theo status, mutable triageStore
- [x] Loading/error state
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/ai-center/hooks/useAITriage.ts`](../../../../ui/src/features/ai-center/hooks/useAITriage.ts) — [NEW] 2 hooks
- [`features/ai-center/components/AITriage.tsx`](../../../../ui/src/features/ai-center/components/AITriage.tsx) — [MODIFY] Refactored
- [`mocks/handlers/ai.handlers.ts`](../../../../ui/src/mocks/handlers/ai.handlers.ts) — [MODIFY] AITriageQueueResponse format, mutable store

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → AI Center → AI Triage
# 1. Verify: queue items từ MSW (F-2847 Confirmed, F-2843 FalsePositive)
# 2. Stats: "1 pending", "1 accepted today" (từ server)
# 3. Click "Accept AI" trên F-2847 → item chuyển sang "AI suggestion accepted"
# 4. Filter "accepted" → chỉ thấy items đã review
```
