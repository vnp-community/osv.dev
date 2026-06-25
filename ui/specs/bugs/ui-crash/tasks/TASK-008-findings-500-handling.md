# TASK-008 — Fix Findings 500: Normalize `findingApi.list` response

**Bug**: [BUG-findings.md](../BUG-findings.md)  
**Solution**: [SOL-C-server-errors.md](../solutions/SOL-C-server-errors.md)  
**Priority**: 🔴 P0  
**Effort**: ~10 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/findings` trả về HTTP 500. Mặc dù là lỗi server, UI cần:
1. **Normalize** response trong `findingApi.list` để không crash nếu 500 trả về partial data
2. **Đảm bảo** `FindingsList` component hiển thị `ErrorState` đúng cách thay vì blank screen

---

## Files cần sửa

1. [`ui/src/features/findings/api/findingApi.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/findings/api/findingApi.ts)
2. Kiểm tra [`ui/src/features/findings/components/FindingsList.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/findings/components/FindingsList.tsx) — nếu thiếu error handling thì thêm

---

## Thay đổi 1 — Normalize `findingApi.list`

**Tìm** trong `findingApi.ts` hàm `list`:

```typescript
  list: async (params: FindingsListParams): Promise<FindingsListResponse> => {
    const { data } = await apiClient.get<FindingsListResponse>(
      ENDPOINTS.findings.list,
      { params }
    );
    return data;
  },
```

**Thay bằng**:

```typescript
  list: async (params: FindingsListParams): Promise<FindingsListResponse> => {
    const { data } = await apiClient.get<FindingsListResponse>(
      ENDPOINTS.findings.list,
      { params }
    );
    // Normalize — đảm bảo findings luôn là array dù server trả partial/null
    return {
      findings:    Array.isArray(data?.findings)    ? data.findings    : [],
      total:       typeof data?.total === 'number'  ? data.total       : 0,
      page:        data?.page        ?? 1,
      page_size:   data?.page_size   ?? 20,
      by_severity: data?.by_severity ?? {},
      by_status:   data?.by_status   ?? {},
      sla_stats:   data?.sla_stats   ?? { breached: 0, at_risk: 0, ok: 0 },
    };
  },
```

## Thay đổi 2 — Kiểm tra error handling trong `FindingsList.tsx`

**Tìm** pattern trong component, đảm bảo có `isError` handling:

```typescript
// Kiểm tra có pattern này chưa:
if (isError) return <ErrorState message={...} onRetry={refetch} />;
```

**Nếu chưa có**, tìm đoạn:
```typescript
const { data, isLoading } = useFindings(params);
```

**Thay bằng**:
```typescript
const { data, isLoading, isError, error, refetch } = useFindings(params);
```

Và thêm sau `if (isLoading)` block:
```typescript
if (isError) {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center">
        <AlertTriangle size={32} style={{ color: 'var(--color-status-error, #EF4444)', margin: '0 auto 12px' }} />
        <p style={{ color: 'var(--color-text-secondary, #9CA3AF)', fontSize: 14 }}>
          Failed to load findings
        </p>
        <p style={{ color: 'var(--color-text-muted, #6B7280)', fontSize: 12, marginTop: 4 }}>
          {(error as Error)?.message ?? 'Server error. Please try again.'}
        </p>
        <button
          onClick={() => refetch()}
          style={{ marginTop: 12, padding: '8px 16px', borderRadius: 8,
            background: 'var(--color-primary-bg, rgba(79,140,255,0.1))',
            color: 'var(--color-primary, #4F8CFF)', border: 'none', cursor: 'pointer' }}
        >
          Retry
        </button>
      </div>
    </div>
  );
}
```

---

## Backend investigation (song song)

```bash
# Check backend logs khi gọi /api/v1/findings
TOKEN=$(curl -s -X POST https://c12.openledger.vn/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openvulnscan.io","password":"Admin@123!ChangeMe"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))")

curl -v -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/findings?page=1&page_size=5" 2>&1 | tail -30
```

---

## Acceptance Criteria

- [ ] `findingApi.list` không throw nếu server trả partial data
- [ ] Trang `/findings` hiển thị "Failed to load findings" với nút Retry thay vì blank/crash
- [ ] Khi server fix 500 → trang tự động recover
- [ ] TypeScript không có lỗi mới

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep -E "findingApi|FindingsList"
```
