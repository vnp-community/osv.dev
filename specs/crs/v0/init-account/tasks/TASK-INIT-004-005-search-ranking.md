# TASK-INIT-004 — search-service: SEARCH_ prefix + REDIS_PASSWORD + init.sh

> **Solution**: [SOL-INIT-004](../solutions/SOL-INIT-004-to-006-search-ranking-notification.md)  
> **Files thực tế**: `cmd/server/main.go` (111 dòng)

---

## Tổng quan

2 thay đổi + 1 file mới:
1. **Sửa** `cmd/server/main.go` — thêm `SEARCH_` prefix cho ports và `REDIS_PASSWORD` vào redis.Options
2. **Tạo** `scripts/init.sh` — tạo OpenSearch index với mapping từ spec §4.2

---

## Bước 1 — Sửa `cmd/server/main.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/cmd/server/main.go`

### Thay đổi 1.1 — Dòng 52-53: thêm SEARCH_ prefix

```diff
-	grpcPort := envOr("GRPC_PORT", "50056")
-	httpPort := envOr("HTTP_PORT", "8082")
+	grpcPort := envOr("SEARCH_GRPC_PORT", envOr("GRPC_PORT", "50056"))
+	httpPort := envOr("SEARCH_HTTP_PORT", envOr("HTTP_PORT", "8083"))
```

### Thay đổi 1.2 — Dòng 72-73: thêm REDIS_PASSWORD

```diff
-	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
-	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
+	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
+	redisClient := redis.NewClient(&redis.Options{
+		Addr:     redisAddr,
+		Password: envOr("REDIS_PASSWORD", ""),
+	})
```

### Thay đổi 1.3 — Dòng 66-69: cải thiện /health response

```diff
 	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
-		w.WriteHeader(http.StatusOK)
-		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
+		w.Header().Set("Content-Type", "application/json")
+		w.WriteHeader(http.StatusOK)
+		fmt.Fprintf(w, `{"status":"ok","service":"search-service","backend":"%s"}`,
+			envOr("SEARCH_BACKEND", "auto")) //nolint:errcheck
 	})
```

### Thay đổi 1.4 — Thêm `"fmt"` vào imports

```diff
 import (
 	"context"
+	"fmt"
 	"net"
 	"net/http"
 	"os"
```

---

## Bước 2 — Tạo `scripts/init.sh`

**Action**: Tạo file mới

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/scripts/init.sh`

Nội dung từ SOL-INIT-004. Script:
- Kiểm tra Redis connectivity
- Tạo OpenSearch index `vulnerabilities` với mapping từ spec §4.2
- Xử lý graceful khi OpenSearch không available (fallback to PG)

**Sau khi tạo**: `chmod +x services/search-service/scripts/init.sh`

---

## Acceptance Criteria

- [ ] `SEARCH_GRPC_PORT` và `SEARCH_HTTP_PORT` được đọc từ env
- [ ] `REDIS_PASSWORD` được truyền vào `redis.Options`
- [ ] `GET /health` trả về JSON với field `backend`
- [ ] `scripts/init.sh` tồn tại và executable
- [ ] `go build ./cmd/server` không lỗi

---

# TASK-INIT-005 — ranking-service: RANKING_PORT env + /health endpoint + init.sh

> **Solution**: [SOL-INIT-005](../solutions/SOL-INIT-004-to-006-search-ranking-notification.md)  
> **Files thực tế**: `cmd/server/main.go` (109 dòng) — đọc `PORT`, không có `RANKING_PORT`

---

## Tổng quan

2 thay đổi + 1 file mới:
1. **Sửa** `cmd/server/main.go` — thêm `RANKING_PORT` fallback (dòng 30)
2. **Kiểm tra** router xem `/health` đã có chưa
3. **Tạo** `scripts/init.sh`

---

## Bước 1 — Sửa `cmd/server/main.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/ranking-service/cmd/server/main.go`

### Thay đổi 1.1 — Dòng 30: thêm RANKING_PORT fallback

```diff
-	port     := envDefault("PORT", "8088")
+	port     := envDefault("RANKING_PORT", envDefault("PORT", "8088"))
```

**Lưu ý**: `envDefault()` tại dòng 103-108 đã đọc `os.Getenv()` đúng cách → không cần fix.  
`ensureIndexes()` tại dòng 86-101 đã tạo đúng indexes → không cần thêm vào init.sh.

---

## Bước 2 — Kiểm tra `/health` trong router

**File**: Kiểm tra `internal/delivery/http/` (cần xem file router/handler của ranking-service)

Nếu router chưa có `/health` endpoint:

**Tìm** file router handler của ranking-service:

```bash
grep -r "/health" /Users/binhnt/Lab/sec/cve/osv.dev/services/ranking-service/
```

Nếu không có → **thêm** vào `cmd/server/main.go` (sau dòng 58 `router := deliveryhttp.NewRouter(handler)`):

Tạo một mux bọc ngoài router có sẵn để thêm `/health`:

```go
// Wrap router with health check
mainMux := http.NewServeMux()
mainMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"ranking-service","port":"%s"}`, port)
})
mainMux.Handle("/", router)

srv := &http.Server{
    Addr:    ":" + port,
    Handler: mainMux,  // ← thay router thành mainMux
    ...
}
```

Thêm `"fmt"` vào imports nếu chưa có.

---

## Bước 3 — Tạo `scripts/init.sh`

**Action**: Tạo file mới

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/ranking-service/scripts/init.sh`

Script đơn giản — kiểm tra MongoDB connectivity. `ensureIndexes()` trong `main.go` đã xử lý index creation.

**Sau khi tạo**: `chmod +x services/ranking-service/scripts/init.sh`

---

## Acceptance Criteria

- [ ] `RANKING_PORT` được đọc từ env
- [ ] `GET /health` trả về 200 JSON response
- [ ] `scripts/init.sh` tồn tại và executable
- [ ] `go build ./cmd/server` không lỗi
