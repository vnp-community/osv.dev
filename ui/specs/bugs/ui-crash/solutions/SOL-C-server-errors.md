# SOL-C — API 500/503 Errors: Server Errors & Service Unavailable

**Bugs**: BUG-003 (Findings 500), BUG-007 (AI Triage 503), BUG-008 (AI Enrichment 503)  
**Priority**: 🔴 P0 (BUG-003) | 🟡 P2 (BUG-007, BUG-008)

---

## BUG-003 — Findings: `GET /api/v1/findings` → 500 Internal Server Error

**Route**: `/findings` | **File**: [BUG-findings.md](../BUG-findings.md)

### Root Cause Analysis

HTTP 500 từ backend có thể do:
1. **Database query lỗi** — query findings với filter mặc định bị timeout hoặc SQL error
2. **Nil pointer** trong Go handler khi request không có query params
3. **Schema mismatch** — database column không match với struct

### Giải pháp

#### UI-side (ngắn hạn): Đảm bảo error handling hoạt động

```typescript
// features/findings/hooks/useFindings.ts
export function useFindings(params?: FindingsParams) {
  return useQuery({
    queryKey: findingKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get(ENDPOINTS.findings.list, { params });
      return {
        findings:   Array.isArray(data?.findings) ? data.findings : [],
        total:      data?.total ?? 0,
        page:       data?.page ?? 1,
        page_size:  data?.page_size ?? 20,
      };
    },
    retry: 1,  // Chỉ retry 1 lần với 500 error
    staleTime: 30_000,
  });
}
```

```typescript
// FindingsList.tsx
export function FindingsList() {
  const { data, isLoading, isError, error, refetch } = useFindings(params);

  if (isLoading) return <FindingsListSkeleton />;
  if (isError) return (
    <ErrorState
      title="Failed to load findings"
      message="The server encountered an error. Please try again."
      onRetry={refetch}
    />
  );
  // ...
}
```

#### Backend-side (dài hạn): Debug và fix

```bash
# Check backend logs khi gọi /api/v1/findings
curl -v -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/findings?page=1&page_size=20" 2>&1

# Hoặc check với empty params
curl -v -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/findings" 2>&1
```

**Khả năng cao**: Thêm default params để tránh nil panic:
```go
// Go handler — thêm defaults
page := r.URL.Query().Get("page")
if page == "" { page = "1" }
pageSize := r.URL.Query().Get("page_size")
if pageSize == "" { pageSize = "20" }
```

---

## BUG-007 & BUG-008 — AI Services: 503 Service Unavailable

**Routes**: `/ai/triage`, `/ai/enrichment`  
**Files**: [BUG-ai-triage.md](../BUG-ai-triage.md), [BUG-ai-enrichment.md](../BUG-ai-enrichment.md)

### Root Cause

AI service (ai-center hoặc ai-triage microservice) chưa được deploy hoặc đang down.  
HTTP 503 = Service Unavailable — backend gateway nhận request nhưng upstream service không phản hồi.

### Endpoints bị 503

```
GET/POST /api/v1/ai/triage/queue     → 503
GET/POST /api/v1/ai/enrichment       → 503
```

### Giải pháp

#### UI-side: Graceful degradation với service unavailable state

```typescript
// features/ai-center/components/AITriage.tsx
export function AITriage() {
  const { data, isLoading, isError, error } = useAITriageQueue();

  if (isLoading) return <AITriageSkeleton />;
  
  // Phân biệt 503 với lỗi khác
  const is503 = (error as AxiosError)?.response?.status === 503;
  
  if (isError) return (
    <div className="flex-1 flex items-center justify-center">
      <EmptyState
        icon={<Brain />}
        title={is503 ? "AI Service Unavailable" : "Failed to load AI queue"}
        description={
          is503
            ? "AI processing service is starting up. This may take a few minutes."
            : "Please try refreshing the page."
        }
        action={!is503 ? { label: "Retry", onClick: refetch } : undefined}
      />
    </div>
  );
  // ...
}
```

#### Backend/Infra: Deploy AI service

```bash
# Kiểm tra trạng thái AI service container
docker ps | grep ai

# Kiểm tra logs
docker logs <ai-service-container> --tail 50

# Restart nếu cần
docker-compose -f deploy/dev/docker-compose.server.yaml restart ai-center
```

#### Verify sau khi fix

```bash
TOKEN="..."
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/ai/triage/queue"
# Kỳ vọng: 200
```

---

## Summary

| Bug | Loại | UI fix | Backend fix |
|-----|------|--------|------------|
| BUG-003 `/findings` 500 | Server error | ErrorState + retry | Debug Go handler, thêm default params |
| BUG-007 `/ai/triage` 503 | Service down | 503-aware EmptyState | Deploy ai-center service |
| BUG-008 `/ai/enrichment` 503 | Service down | 503-aware EmptyState | Deploy ai-center service |
