# TASK-003 — data-service: Fix CWE List trả `null` → `[]`

**Bug**: [BUG-001](../BUG-001-cve-cwe.md)  
**Solution**: [SOL-001](../solutions/SOL-001-cve-cwe.md)  
**Priority**: 🟠 P1  
**Effort**: ~10 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v2/cwe` trả `{ "data": null }` khi chưa có data. Frontend `CWELibrary` gọi `.map()` trực tiếp → crash. Fix tại tầng repository và handler trong data-service.

---

## File cần sửa

Tìm CWE handler trong data-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal -name "*.go" \
  | xargs grep -l "cwe\|CWE" | head -10
```

---

## Thay đổi — Repository

**Tìm** hàm List CWE trong repo:

```go
var cweList []CWE       // ← SAI
// hoặc
var cweList []*CWE
```

**Thay bằng**:

```go
cweList := make([]*CWE, 0)   // ← ĐÚNG: never nil
```

---

## Thay đổi — Handler

**Tìm** handler GET `/api/v2/cwe`:

```go
respondJSON(w, http.StatusOK, map[string]interface{}{
    "data": cweList,
})
```

**Thêm nil guard**:

```go
if cweList == nil {
    cweList = make([]*CWE, 0)
}
respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":  cweList,
    "total": len(cweList),
})
```

---

## Response Schema

```json
{
  "data": [],
  "total": 0
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/cwe` trả `{"data": []}` khi không có data
- [ ] `GET /api/v2/cwe/{id}` trả `404` khi không tìm thấy (không crash)
- [ ] `go build ./...` trong data-service không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service
go build ./...

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/cwe" | jq '.data | type'
# Expected: "array"
```
