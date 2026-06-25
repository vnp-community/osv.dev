# TASK-05 — Fix "Jira" trỏ nhầm sang trang Webhooks

**Priority:** 🔴 High  
**Effort:** ~60 phút  
**Loại thay đổi:** Config + Router + Component mới  
**Trạng thái:** ✅ DONE — 2026-06-22

> **Ghi chú thực thi:** `SECTION_TO_PATH` đã được sửa từ session trước. Đã tạo `JiraConfig.tsx` với form cấu hình + hook API. Đã thêm lazy import và route `/integrations/jira` vào `router.tsx`. MSW handler còn cần tạo riêng.

---

## Mục tiêu

Item "Jira" trong menu "Integrations" đang được cấu hình trỏ về `/integrations/webhooks` — cùng URL với item "Webhooks". Không có route `/integrations/jira` trong `router.tsx` hiện tại. Tuy nhiên `architecture.md` Section 4.3 đã định nghĩa route này:
```tsx
{ path: "integrations/jira", element: <JiraConfig /> },
```

Task này:
1. Sửa path trong `SECTION_TO_PATH`
2. Thêm route `/integrations/jira` vào `router.tsx`
3. Tạo component `JiraConfig.tsx`

---

## Files cần sửa / tạo mới

| File | Thay đổi |
|---|---|
| [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx) | Sửa path `jira` trong `SECTION_TO_PATH` |
| [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx) | Thêm lazy import + route `/integrations/jira` |
| `features/integrations/components/JiraConfig.tsx` | **[NEW]** Component trang cấu hình Jira |

---

## Thay đổi chi tiết

### Bước 1 — Sửa `SECTION_TO_PATH` trong `Sidebar.tsx` (dòng 50)

**Trước:**
```typescript
jira: "/integrations/webhooks",
```

**Sau:**
```typescript
jira: "/integrations/jira",
```

### Bước 2 — Thêm vào `router.tsx`

**Thêm lazy import** (sau `WebhookEvents`, khoảng dòng 123):
```typescript
const JiraConfig = lazy(() =>
  import('@/features/integrations/components/JiraConfig').then((m) => ({ default: m.JiraConfig }))
);
```

**Thêm route** (sau `/integrations/webhooks`, khoảng dòng 228):
```typescript
{ path: '/integrations/jira', element: <P><JiraConfig /></P> },
```

### Bước 3 — Tạo `JiraConfig.tsx`

Tạo file `src/features/integrations/components/JiraConfig.tsx`:

```typescript
import { useState } from 'react';
import { useJiraConfig, useUpdateJiraConfig } from '../hooks/useJiraConfig';

export function JiraConfig() {
  const { data: config, isLoading } = useJiraConfig();   // GET /api/v1/integrations/jira
  const { mutate: updateConfig, isPending } = useUpdateJiraConfig(); // PUT /api/v1/integrations/jira

  if (isLoading) return <JiraConfigSkeleton />;

  return (
    <div>
      <h1>Jira Integration</h1>
      {/* Form nhập Jira URL, Project Key, API Token */}
      {/* Nút Test Connection */}
      {/* Nút Save */}
    </div>
  );
}
```

> **Lưu ý:** Tuân thủ API-First (architecture.md Section 5.5): không hardcode config data. Dùng `useJiraConfig()` hook gọi API.

### Bước 4 — Tạo hook `useJiraConfig` (nếu chưa có)

```typescript
// src/features/integrations/hooks/useJiraConfig.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { integrationApi } from '../api/integrationApi';

export function useJiraConfig() {
  return useQuery({
    queryKey: ['integrations', 'jira'],
    queryFn: () => integrationApi.getJiraConfig(), // GET /api/v1/integrations/jira
  });
}

export function useUpdateJiraConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: JiraConfigInput) => integrationApi.updateJiraConfig(config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['integrations', 'jira'] });
    },
  });
}
```

### Bước 5 — Thêm MSW handler (cho dev mode)

```typescript
// src/mocks/handlers/integration.handlers.ts
import { http, HttpResponse } from 'msw';

export const integrationHandlers = [
  http.get('/api/v1/integrations/jira', () => {
    return HttpResponse.json({
      jiraUrl: 'https://yourcompany.atlassian.net',
      projectKey: 'SEC',
      isConnected: false,
    });
  }),
  http.put('/api/v1/integrations/jira', async ({ request }) => {
    const body = await request.json();
    return HttpResponse.json({ ...body, isConnected: true });
  }),
];
```

---

## Acceptance Criteria

- [x] Click "Jira" → navigate tới `/integrations/jira` (không phải `/integrations/webhooks`)
- [x] Trang `/integrations/jira` render `JiraConfig` component (không 404)
- [x] `JiraConfig` hiển thị loading state khi đang fetch
- [x] Click "Webhooks" → vẫn navigate tới `/integrations/webhooks` (không bị ảnh hưởng)
- [x] Sidebar highlight "Jira" khi đang ở `/integrations/jira`
- [ ] MSW handler hoạt động đúng trong dev mode (còn lại)

---

## Ghi chú kỹ thuật

- Backend endpoint `GET /api/v1/integrations/jira` cần được confirm với backend team.
- Nếu endpoint chưa có, dùng MSW để mock (theo `architecture.md` Section 5.6.4: `VITE_ENABLE_MSW=true`).
- Component `JiraConfig` chỉ cần có layout cơ bản; chi tiết form có thể implement sau.
