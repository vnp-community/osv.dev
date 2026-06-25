# TASK-V5-002: Sửa KEV Stats Response Schema

## Mô tả
`GET /api/v2/kev/stats` trả về 200 nhưng test schema validation fail.

## Vấn đề
Test kiểm tra `body["by_vendor"]` và `body["recent_additions"]` ở top-level của response.
Current response không có các keys này ở đúng vị trí.

## Các bước thực thi

### 1. Tìm KEV stats handler
```bash
grep -rn "kev.*stats\|stats.*kev\|/kev/stats" services/data-service/ --include="*.go" | head -20
```

### 2. Kiểm tra response hiện tại
```bash
curl -s "https://c12.openledger.vn/api/v2/kev/stats" | python3 -m json.tool
```

### 3. Cập nhật handler
Đảm bảo response có dạng:
```json
{
  "total": 1623,
  "by_vendor": { "Microsoft": 150, "Apache": 120 },
  "recent_additions": ["CVE-2024-...", ...],
  "last_updated": "2026-06-23"
}
```

## Acceptance Criteria
- [ ] `GET /api/v2/kev/stats` → 200 với `by_vendor` và `recent_additions` ở top-level
- [ ] Test `kev_stats_response_schema` → PASS

## Status: TODO
