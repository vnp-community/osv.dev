# BUG-BE-011 — AI Service Unavailable (503) — Tất Cả /ai/* Endpoints

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-011 |
| **Severity** | 🟡 Medium |
| **Priority** | P2 |
| **Component** | Backend / AI Enrichment Service |
| **Endpoints** | `GET /api/v1/ai/triage/queue`, `GET /api/v1/ai/enrichment`, `GET /api/v1/ai/enrichment/{cveId}` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

Tất cả AI Center endpoints đều trả về **HTTP 503 Service Unavailable** với message `"Upstream service unavailable"`. AI Enrichment service chưa hoạt động trên server staging.

## Tái hiện

```bash
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/triage/queue
# => HTTP 503
# {"error":"SERVICE_UNAVAILABLE","message":"Upstream service unavailable"}

curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/enrichment
# => HTTP 503

curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/enrichment/CVE-2021-44228
# => HTTP 503
```

## Root Cause

Từ `deploy/dev/.env`:
```
AI_BACKEND=openai
AI_MODEL=gpt-4o-mini
OPENAI_API_KEY=          ← TRỐNG
```

AI enrichment service không start được (hoặc start nhưng không thể kết nối OpenAI) vì `OPENAI_API_KEY` trống.

## Kiểm tra

```bash
# Kiểm tra container AI service có đang chạy không
docker ps | grep ai
docker logs osv-backend-ai-enrichment-1 --tail 50

# Kiểm tra gateway có thể reach AI service không
docker exec osv-backend-gateway-1 curl http://ai-enrichment:50052/health
```

## Fix Options

**Option A — Set OpenAI API Key (nhanh nhất)**:
```bash
# Thêm vào deploy/dev/.env
OPENAI_API_KEY=sk-proj-xxxxxx
# Restart AI service
docker compose restart ai-enrichment
```

**Option B — Dùng Ollama (local, miễn phí)**:
```bash
# Trong deploy/dev/.env
AI_BACKEND=ollama
OLLAMA_URL=http://ollama:11434
OLLAMA_LLM_MODEL=llama3.2
```

**Option C — Disable AI features trên staging**:
Gateway nên trả **200 với empty/disabled state** thay vì 503 khi AI không available:
```json
{
  "items": [],
  "total": 0,
  "status": "ai_service_disabled"
}
```

## Ảnh hưởng

- AI Center (`/ai`) toàn bộ không dùng được
- AI Triage queue trống
- CVE enrichment không chạy được thủ công
- Dashboard AI insights bị ảnh hưởng
