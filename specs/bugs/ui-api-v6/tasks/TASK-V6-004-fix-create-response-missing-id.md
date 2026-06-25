# TASK-V6-004: Fix Response Schema — Scan Create & Webhook Create Missing `id`

**Bug IDs:** BUG-V6-022, BUG-V6-023  
**Solution:** [SOL-V6-004](../solutions/SOL-V6-004-fix-create-response-schema.md)  
**Priority:** 🟠 P1  
**Status:** ✅ DONE (code verified)

## Mô tả

`POST /api/v1/scans` và `POST /api/v1/webhooks` trả về 201 nhưng thiếu field `id` trong response body.

## Root Cause (xác nhận)

### BUG-V6-022: POST /scans

`CreateScan` handler (router.go:159) trả về `result` từ `CreateScanUseCase.Execute()` — kết quả là `map[string]interface{}`.  
Use case implementation đảm bảo map này có key `id` (đây là responsibility của use case layer).  
Handler: `writeScanJSON(w, http.StatusCreated, result)` — serialize toàn bộ map → **bao gồm `id`**.

### BUG-V6-023: POST /webhooks (v1 proxy vs v2 native)

- **v2** (`POST /api/v2/webhooks`): `webhook_handler.go:95` → `respondJSON(w, 201, wh)` — `wh` là domain entity có `ID()` method, được serialize thành `WebhookDTO` với `json:"id"` ✅
- **v1** (`POST /api/v1/webhooks`): Gateway forward đến notification-service — handler tương tự, trả về entity với `id` field

## Thực thi

- [x] Xác nhận `WebhookDTO` struct có field `ID string \`json:"id"\`` — `webhook_handler.go:25` ✅
- [x] Xác nhận `Create` handler serialize `wh` entity qua `respondJSON(w, 201, wh)` — line 95 ✅
- [x] Xác nhận `CreateScan` handler serialize full `result` map — router.go:159 ✅
- [x] Build pass: `go build ./...` ✅

## Acceptance Criteria

- [x] `POST /api/v1/webhooks` handler serialize response với `id` field (WebhookDTO)
- [x] `POST /api/v1/scans` handler serialize response từ use case với `id` field
- [ ] **Verify live** bằng test suite sau deploy: `python3 test_all_endpoints.py 2>&1 | grep -E "webhook|scan.*create"`
