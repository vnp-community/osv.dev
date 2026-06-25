# BUG-006: Findings — PATCH, Notes GET, Bulk Actions Bị Lỗi

**ID**: BUG-006  
**Domain**: Findings & Risk  
**Mức độ**: 🔴 HIGH  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status | Loại Lỗi |
|---|---|---|---|
| `PATCH` | `/api/v1/findings/{id}`        | **405** | Method Not Allowed |
| `GET`   | `/api/v1/findings/{id}/notes`  | **405** | Method Not Allowed |
| `POST`  | `/api/v1/findings/bulk/close`  | **405** | Method Not Allowed |
| `POST`  | `/api/v1/findings/bulk/reopen` | **405** | Method Not Allowed |
| `POST`  | `/api/v1/findings/bulk/assign` | **404** | Route missing |

## Endpoints Đang Hoạt Động

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/findings`           | ✅ 500 (route OK) |
| `GET`  | `/api/v1/findings/stats`     | ✅ 400 |
| `GET`  | `/api/v1/findings/{id}`      | ✅ 400 |
| `POST` | `/api/v1/findings/{id}/notes`| ✅ 400 |
| `GET`  | `/api/v1/findings/{id}/audit`| ✅ pass |

## Mô Tả

### PATCH `/findings/{id}` → 405
UI gửi `PATCH` để partial update finding (severity, status, assignee). Server không hỗ trợ. Có thể server chỉ accept `PUT`.

**Hệ quả**: "Update Finding" button fail silently.

### GET `/findings/{id}/notes` → 405  
Paradox: `POST /notes` (200/400) hoạt động nhưng `GET /notes` trả 405. Router có thể chỉ có POST handler. Notes tab sẽ render trống.

### Bulk Close/Reopen → 405
Route tồn tại (trả 405 thay vì 404) nhưng không accept POST. Có thể server dùng `PUT /findings/bulk` với action field trong body.

### Bulk Assign → 404  
Route hoàn toàn chưa implement.

## Giải Pháp Đề Xuất

```
PATCH /api/v1/findings/{id}
Body: { "status": "closed", "assignee_id": "...", "severity": "high" }
→ 200 OK { finding }

GET /api/v1/findings/{id}/notes
→ 200 OK { "items": [{ "id": "...", "text": "...", "author": {...}, "created_at": "..." }] }

POST /api/v1/findings/bulk/close
Body: { "finding_ids": ["id1", "id2"], "reason": "..." }
→ 200 OK { "updated": 2 }

POST /api/v1/findings/bulk/assign
Body: { "finding_ids": ["id1", "id2"], "assignee_id": "..." }
→ 200 OK { "updated": 2 }
```

## File UI Bị Ảnh Hưởng

- `src/features/findings/components/FindingDetail.tsx`
- `src/features/findings/components/FindingsList.tsx` (bulk actions toolbar)
- `src/features/findings/hooks/useFindings.ts`
