# TASK-P3-03 — Fix `APIKeyManagement.tsx` → `useAPIKeys()` + Backend Key Gen

**Phase:** 3 — Core Features  
**Nguồn giải pháp:** [`solutions/09_api_key_management.md`](../solutions/09_api_key_management.md)  
**Ưu tiên:** 🟡 Core — ưu tiên cao (⚠️ bảo mật)  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/integrations/components/APIKeyManagement.tsx
const apiKeys = [
  { id: 'k-001', name: 'CI/CD Pipeline', key: 'osv_prod_xK7m...', ... },
  // hardcode
];

// ❌ LỖ HỔNG BẢO MẬT NGHIÊM TRỌNG — Generate key dùng Math.random():
const key = 'osv_' + Math.random().toString(36).substr(2, 32);
// Math.random() KHÔNG phải CSPRNG — key có thể bị predict
```

> **⚠️ Security Impact:** Math.random() không phải cryptographically secure. API keys generate bằng Math.random() có thể bị attacker predict nếu biết timestamp. Key phải được generate ở backend dùng `crypto/rand` (Go) hoặc `crypto.randomBytes()` (Node.js).

---

## API Endpoints

```
GET    /api/v1/api-keys         → Danh sách (ENDPOINTS.apiKeys.list)
POST   /api/v1/api-keys         → Tạo key mới — BACKEND generate (ENDPOINTS.apiKeys.create)
DELETE /api/v1/api-keys/:id     → Revoke key (ENDPOINTS.apiKeys.revoke)
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/integrations/types.ts`

```typescript
export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  createdAt: string;
  lastUsedAt?: string;
  expiresAt?: string;
  status: 'active' | 'revoked';
  createdBy: string;
}

export interface CreateAPIKeyResponse {
  key: APIKey;
  rawKey: string;  // Full key — chỉ trả về 1 lần, không lưu backend
}

export interface APIKeysResponse {
  keys: APIKey[];
  total: number;
}

export interface CreateAPIKeyRequest {
  name: string;
  scopes: string[];
  expiresAt?: string;
}
```

### [NEW] `src/features/integrations/hooks/useAPIKeys.ts`

```typescript
export function useAPIKeys() { ... }
export function useCreateAPIKey() { ... }  // Backend generates key
export function useRevokeAPIKey() { ... }
```

Xem code đầy đủ tại: [`solutions/09_api_key_management.md`](../solutions/09_api_key_management.md)

### [MODIFY] `src/features/integrations/components/APIKeyManagement.tsx`

Xem code đầy đủ tại: [`solutions/09_api_key_management.md`](../solutions/09_api_key_management.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `const apiKeys = [...]`
- **XÓA** toàn bộ code `Math.random()` generate key ở frontend
- Sau khi POST thành công, server trả về `rawKey` — hiển thị 1 lần
- Copy button để copy full key vào clipboard
- Warning banner: "Key sẽ không hiển thị lại — lưu ngay vào vault"

### [MODIFY] `src/shared/api/endpoints.ts` — thêm apiKeys

```typescript
apiKeys: {
  list:   '/api/v1/api-keys',
  create: '/api/v1/api-keys',
  revoke: (id: string) => `/api/v1/api-keys/${id}`,
},
```

### [NEW/MODIFY] `src/mocks/handlers/integrations.handlers.ts`

```typescript
// MSW dùng crypto module (Node.js context) thay Math.random()
// Trong production backend Go: crypto/rand.Read()
export const integrationHandlers = [
  http.get('/api/v1/api-keys', ...),
  http.post('/api/v1/api-keys', ...),  // Returns rawKey
  http.delete('/api/v1/api-keys/:id', ...),
];
```

Xem data fixture và handler đầy đủ tại: [`solutions/09_api_key_management.md`](../solutions/09_api_key_management.md) — mục "MSW Handler"

---

## Lưu ý Backend (ngoài scope frontend)

> Backend Go phải implement:
> ```go
> func GenerateAPIKey() (prefix, rawKey, hashedKey string, err error) {
>     b := make([]byte, 32)
>     if _, err = rand.Read(b); err != nil { return }
>     rawKey = fmt.Sprintf("osv_prod_%s", base64.URLEncoding.EncodeToString(b))
>     prefix = rawKey[:16]
>     hashedKey = hashSHA256(rawKey)  // Chỉ lưu hash vào DB
>     return
> }
> ```
> Backend **chỉ lưu hash** của key, không bao giờ lưu raw key.

---

## Tiêu chí hoàn thành

- [x] `features/integrations/hooks/useAPIKeys.ts` tạo xong (GET + POST + DELETE)
- [x] `APIKeyManagement.tsx` không còn `const apiKeys = [...]`
- [x] **KHÔNG CÒN** `Math.random()` trong frontend key generation
- [x] Sau "Generate Key": banner hiển thị `plain_key` 1 lần với copy button
- [x] Copy button copy full key vào clipboard
- [x] "Revoke" button gọi DELETE mutation và refresh list (soft-revoke)
- [x] MSW handler: import từ api-keys.fixture.ts, plain_key trong response
- [x] formatDate/formatRelative từ ISO (không hardcode)
- [x] Loading/error state
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/integrations/hooks/useAPIKeys.ts`](../../../../ui/src/features/integrations/hooks/useAPIKeys.ts) — [NEW] 3 hooks
- [`features/integrations/components/APIKeyManagement.tsx`](../../../../ui/src/features/integrations/components/APIKeyManagement.tsx) — [MODIFY] Refactored, CreateKeyModal
- [`mocks/handlers/integration.handlers.ts`](../../../../ui/src/mocks/handlers/integration.handlers.ts) — [MODIFY] Import fixture, plain_key response

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Integrations → API Keys
# 1. Verify: 4 keys từ MSW (3 active, 1 revoked)
# 2. Click "Generate Key" → điền form → Generate
# 3. Verify: banner xuất hiện với key mới (format: osv_prod_...)
# 4. Click Copy → check clipboard có full key
# 5. Click Dismiss → banner ẩn (key không còn hiển thị nữa)
# 6. Revoke CI/CD Pipeline → status đổi sang "revoked"
```
