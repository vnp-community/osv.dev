# TASK-V5-006: Pull Ollama nomic-embed-text Model

## Mô tả
`POST /api/v2/cves/search/semantic` → 500 vì Ollama container chưa có model `nomic-embed-text`.

## Root Cause
Ollama container chạy nhưng không tự động pull model khi khởi động.
`ai-service` embed gọi Ollama API với model `nomic-embed-text` → 404 from Ollama → `semantic search failed`.

## Giải pháp

### Bước 1: Pull model trực tiếp trên server (Quick Fix)
```bash
ssh ubuntu@172.20.2.48 'docker exec osv-ollama ollama pull nomic-embed-text'
```
Lưu ý: Model khoảng 274MB, cần kết nối internet từ server.

### Bước 2: Thêm init container vào docker-compose (Permanent Fix)
Trong `deploy/dev/docker-compose.server.yml`, thêm service ollama-init:
```yaml
ollama-init:
  image: curlimages/curl:latest
  depends_on:
    ollama:
      condition: service_started
  command: >
    sh -c "sleep 5 && curl -X POST http://ollama:11434/api/pull -d '{\"name\":\"nomic-embed-text\"}'"
  networks:
    - osv-internal
  restart: "no"
```

### Bước 3 (Alternative): Graceful Degradation
Nếu model chưa available, `ai-service` nên trả về 503 (Service Unavailable) thay vì 500 để client biết service đang khởi động.

## Acceptance Criteria
- [ ] `POST /api/v2/cves/search/semantic` → 200 hoặc ít nhất không phải 500
- [ ] Test `semantic_search_returns_200` → PASS

## Status: TODO
