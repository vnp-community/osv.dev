# BUG-H2-006 — notification-service: AlertsHandler nil → 503 /notifications

## Metadata
- **ID**: BUG-H2-006
- **Service**: `notification-service`
- **File**: [`router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/router.go)
- **Lines**: 86–94
- **Severity**: 🟠 Medium
- **Category**: Nil Guard / Missing Wire
- **Status**: ✅ Verified — AlertsHandler đã được wire đúng trong embedded.go L86-93. Bug chỉ xảy ra nếu ahHandler == nil, hiện tại không bị ảnh hưởng.


## Mô tả

Trong `SetupRouter`, nếu `ah` (`AlertsHandler`) là `nil`, toàn bộ `/api/v2/notifications` và `/api/v1/notifications` routes sẽ trả `503 Service Unavailable`.

```go
// router.go L86-94 — BUG
} else {
    unavailable := func(w http.ResponseWriter, r *http.Request) {
        respondJSON(w, http.StatusServiceUnavailable,
            map[string]string{"error": "notification service not fully initialized"})
    }
    r.Get("/api/v2/notifications", unavailable)
    r.Get("/api/v2/notifications/stream", unavailable)
}
```

## Root Cause

`embedded.go` của notification-service có thể không wire `AlertsHandler` khi DB pool nil hoặc khi có một số dependency bị thiếu, dẫn đến `ah=nil` → 503 production.

## Tác động

- Notification bell trên UI luôn hiển thị lỗi 503
- In-app notification system bị broken hoàn toàn

## Fix

Kiểm tra `embedded.go` của notification-service và đảm bảo `AlertsHandler` luôn được inject khi `pool != nil`.

## References
- [router.go:L86-94](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/router.go#L86-L94)
