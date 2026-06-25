# TASK-P2-04 — Fix `SystemSettings.tsx` → `useSystemSettings()`

**Phase:** 2 — Admin Module  
**Nguồn giải pháp:** [`solutions/04_admin_settings.md`](../solutions/04_admin_settings.md)  
**Ưu tiên:** 🟠 Admin — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P2-01 (dùng chung types.ts)

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/admin/components/SystemSettings.tsx
const aiProviders = [
  { name: 'OpenAI', model: 'gpt-4o', status: 'active', ... },
  // hardcode 3 AI providers
];
// Form mặc định hardcode giá trị:
// Platform Name = "OSV Platform"
// SMTP = "smtp.company.com"
// Min Length = "12", Session Timeout = "60"
```

---

## API Endpoints

```
GET /api/v1/admin/settings → Load toàn bộ settings
PUT /api/v1/admin/settings → Save settings
```

---

## Danh sách files cần tạo/sửa

### [MODIFY] `src/features/admin/types.ts` — thêm SystemSettings types

```typescript
export interface SystemSettings {
  general: {
    platformName: string;
    organization: string;
    supportEmail: string;
    timezone: string;
    logoUrl?: string;
  };
  smtp: {
    host: string;
    port: number;
    username?: string;
    useTls: boolean;
    fromEmail: string;
  };
  security: {
    passwordMinLength: number;
    passwordMaxAgeDays: number;
    sessionTimeoutMinutes: number;
    maxConcurrentSessions: number;
    mfaRequired: boolean;
    allowOAuth: boolean;
  };
  ai: {
    providers: AIProviderConfig[];
    activeProviderId: string;
  };
}

export interface AIProviderConfig {
  id: string;
  name: string;
  model: string;
  status: 'active' | 'standby' | 'inactive';
  latencyMs?: number;
  requestsPerDay?: number;
  costPerDay?: number;
}
```

### [NEW] `src/features/admin/hooks/useSystemSettings.ts`

```typescript
export function useSystemSettings() {
  return useQuery<SystemSettings>({
    queryKey: ['admin', 'settings'],
    queryFn: async () => {
      const { data } = await apiClient.get<SystemSettings>(ENDPOINTS.admin.settings);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (settings: Partial<SystemSettings>) =>
      apiClient.put(ENDPOINTS.admin.settings, settings),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'settings'] });
    },
  });
}
```

### [MODIFY] `src/features/admin/components/SystemSettings.tsx`

Xem code đầy đủ tại: [`solutions/04_admin_settings.md`](../solutions/04_admin_settings.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `const aiProviders = [...]`
- Dùng `useSystemSettings()` để load form values ban đầu
- `useEffect` sync form state từ server data
- "Save Changes" gọi `updateSettings.mutateAsync(form)`
- AI Providers tab: render từ `data.ai.providers`

### [MODIFY] `src/mocks/handlers/admin.handlers.ts` — thêm settings handlers

```typescript
// Mutable store để PUT có tác dụng
let settingsStore: SystemSettings = { ... };

http.get('/api/v1/admin/settings', () => HttpResponse.json(settingsStore)),
http.put('/api/v1/admin/settings', async ({ request }) => {
  const body = await request.json();
  settingsStore = { ...settingsStore, ...body };
  return HttpResponse.json(settingsStore);
}),
```

Xem data đầy đủ tại: [`solutions/04_admin_settings.md`](../solutions/04_admin_settings.md) — mục "MSW Handler"

---

## Tiêu chí hoàn thành

- [x] `features/admin/types.ts` có SystemSettings, AIProviderConfig
- [x] `features/admin/hooks/useSystemSettings.ts` tạo xong (GET + PUT)
- [x] `SystemSettings.tsx` không còn hardcode form values
- [x] Form load giá trị từ server khi mở (General section editable)
- [x] "Save Changes" gọi PUT và invalidate query + hiển thị "Saved!" feedback
- [x] AI Providers tab render danh sách từ server (không hardcode 3 providers)
- [x] MSW handler có mutable settingsStore (PUT thực sự thay đổi state)
- [x] Loading/error state
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/admin/types.ts`](../../../../ui/src/features/admin/types.ts) — SystemSettings, AIProviderConfig
- [`features/admin/hooks/useSystemSettings.ts`](../../../../ui/src/features/admin/hooks/useSystemSettings.ts) — [NEW] GET + PUT
- [`features/admin/components/SystemSettings.tsx`](../../../../ui/src/features/admin/components/SystemSettings.tsx) — [MODIFY] Refactored
- [`mocks/handlers/admin.handlers.ts`](../../../../ui/src/mocks/handlers/admin.handlers.ts) — mutable settingsStore + PUT handler

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Admin → Settings
# 1. Verify: Platform Name = "OSV Platform" load từ MSW
# 2. Tab "AI Providers": thấy OpenAI (active), Azure (standby), Ollama (inactive)
# 3. Đổi Platform Name → Save → refresh lại → vẫn giữ giá trị mới (MSW mutable store)
# 4. Tab "Security": Min Password Length = 12 từ server
```
