# TASK-V5-005: Sửa API Keys List Response Format

## Mô tả
`GET /api/v1/api-keys` trả về `{ "keys": [], "total": 0 }` thay vì direct array `[]`.

## Root Cause
Handler `ListAPIKeys()` trong `services/identity-service/adapter/handler/http/api_key_handler.go`:
```go
writeJSON(w, http.StatusOK, map[string]any{
    "keys":  items,
    "total": len(items),
})
```
Test kiểm tra: `isinstance(body, list)` — tức là response phải là JSON array.

## Giải pháp

**Option A (theo spec):** Trả về direct array:
```go
writeJSON(w, http.StatusOK, items)
```

**Option B (giữ backward compat):** Trả về wrapped nhưng với key `items` thay vì `keys`:
```json
{ "items": [], "total": 0 }
```
Tuy nhiên test yêu cầu array, nên Option A là đúng.

## Acceptance Criteria
- [ ] `GET /api/v1/api-keys` → 200 với JSON array `[...]`
- [ ] Test `api_keys_list_returns_200_as_array` → PASS

## Status: TODO
