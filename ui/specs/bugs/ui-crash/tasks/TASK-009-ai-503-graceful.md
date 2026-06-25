# TASK-009 — Improve AI Services 503: Phân biệt "Service Down" vs lỗi khác

**Bugs**: [BUG-ai-triage.md](../BUG-ai-triage.md), [BUG-ai-enrichment.md](../BUG-ai-enrichment.md)  
**Solution**: [SOL-C-server-errors.md](../solutions/SOL-C-server-errors.md)  
**Priority**: 🟡 P2  
**Effort**: ~10 phút  
**Status**: `[x] DONE`

---

## Mô tả

AI Triage và AI Enrichment đã có `isError` handling (hiển thị "Failed to load..." + Retry button).  
Tuy nhiên, 503 = Service Unavailable nghĩa là AI service chưa được deploy — Retry sẽ không giải quyết được.  
Cần **phân biệt 503** và hiển thị thông báo phù hợp ("Service starting up...") thay vì "Retry".

---

## Files cần sửa

1. [`ui/src/features/ai-center/components/AITriage.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/ai-center/components/AITriage.tsx)
2. [`ui/src/features/ai-center/components/AIEnrichment.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/ai-center/components/AIEnrichment.tsx)

---

## Thêm import AxiosError (nếu chưa có)

```typescript
import type { AxiosError } from 'axios';
```

---

## Thay đổi trong `AITriage.tsx`

**Tìm** block `isError` hiện tại (khoảng line 64–80):

```typescript
  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load AI triage queue</p>
          <button
            onClick={() => refetch()}
            className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
            style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
          >
            <RefreshCw size={13} /> Retry
          </button>
        </div>
      </div>
    );
  }
```

**Thay bằng**:

```typescript
  if (isError) {
    const is503 = (error as AxiosError)?.response?.status === 503;
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>
            {is503 ? "AI Service Unavailable" : "Failed to load AI triage queue"}
          </p>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 4 }}>
            {is503
              ? "AI processing service is starting up. No action needed."
              : "Please try again."}
          </p>
          {!is503 && (
            <button
              onClick={() => refetch()}
              className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
              style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
            >
              <RefreshCw size={13} /> Retry
            </button>
          )}
        </div>
      </div>
    );
  }
```

## Áp dụng thay đổi tương tự cho `AIEnrichment.tsx`

Tìm block `isError` trong `AIEnrichment.tsx` và áp dụng cùng pattern — thay message thành:
- 503: `"AI Enrichment Service Unavailable"` + `"AI enrichment service is starting up."`
- Khác: `"Failed to load AI enrichment"` + Retry button

---

## Acceptance Criteria

- [ ] Khi AI service 503 → hiển thị "AI Service Unavailable" mà không có nút Retry vô dụng
- [ ] Khi lỗi khác (401, 500) → hiển thị "Failed to load..." + Retry button
- [ ] TypeScript không có lỗi mới (`error` được cast đúng type)

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep -E "AITriage|AIEnrichment"
```

## Backend action (ngoài phạm vi UI task)

```bash
# Deploy AI service
docker-compose -f deploy/dev/docker-compose.server.yaml up -d ai-center
# Verify
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/ai/triage/queue"
```
