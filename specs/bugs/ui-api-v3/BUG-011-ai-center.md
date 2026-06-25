# BUG-011: AI Center — Triage Queue, Enrichment, Insights Chưa Implement

**ID**: BUG-011  
**Domain**: AI Center  
**Mức độ**: 🔴 HIGH  
**Loại**: `404 Not Found`  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status |
|---|---|---|
| `GET`  | `/api/v1/ai/triage/queue`       | **404** |
| `GET`  | `/api/v1/ai/enrichment`         | **404** |
| `POST` | `/api/v1/ai/enrichment/trigger` | **404** |
| `GET`  | `/api/v1/ai/insights`           | **404** |

## Endpoints Đang Hoạt Động

| Method | Endpoint | Status |
|---|---|---|
| `POST` | `/api/v1/ai/triage/{findingId}`        | ✅ pass |
| `POST` | `/api/v1/ai/triage/{findingId}/review` | ✅ pass |
| `GET`  | `/api/v1/ai/enrichment/{cveId}`        | ✅ pass |

## Mô Tả

Server đã implement per-resource AI actions nhưng thiếu toàn bộ aggregate endpoints:

### `GET /ai/triage/queue` — 404
`AITriage.tsx` hiển thị hàng đợi các findings chờ AI xem xét. Không có route này → queue render trống.

### `GET /ai/enrichment` — 404
Danh sách CVEs đã được AI enrichment không load được.

### `POST /ai/enrichment/trigger` — 404  
Không thể trigger enrichment batch job từ UI.

### `GET /ai/insights` — 404
`AIInsights.tsx` gọi endpoint này để hiển thị AI-generated security insights. Toàn bộ trang bị blank.

## Giải Pháp Đề Xuất

```
GET /api/v1/ai/triage/queue?status=pending&page=1&limit=20
→ 200 OK
{
  "items": [{ "finding_id": "...", "cve_id": "...", "status": "pending", "queued_at": "..." }],
  "total": 7
}

GET /api/v1/ai/insights
→ 200 OK
{
  "items": [{ "type": "trend", "title": "...", "content": "...", "severity": "high", "generated_at": "..." }]
}

GET /api/v1/ai/enrichment?status=completed&page=1
→ 200 OK { "items": [...], "total": 100 }

POST /api/v1/ai/enrichment/trigger
Body: { "cve_ids": ["CVE-2021-44228"] }  // optional — if empty, trigger all pending
→ 202 Accepted { "job_id": "..." }
```

## File UI Bị Ảnh Hưởng

- `src/features/ai-center/components/AITriage.tsx`
- `src/features/ai-center/components/AIInsights.tsx`
- `src/features/ai-center/hooks/useAITriage.ts`
