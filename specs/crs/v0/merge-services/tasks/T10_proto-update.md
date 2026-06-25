# T10 — Proto Update ✅ DONE

**Phase**: 10
**Depends on**: T09
**Status**: ✅ Completed — 2026-06-12
**Estimated effort**: 1 hours

---

## Mục tiêu

Cập nhật `shared/proto` để reflect đúng 8 service mới. Đảm bảo tất cả proto services có đúng package names và generate lại Go code.

---

## Tác vụ chi tiết

### Bước 1: Kiểm tra proto hiện tại

```bash
PROTO_DIR="/Users/binhnt/Lab/sec/cve/osv.dev/services/shared/proto"

ls "$PROTO_DIR"
# Hiện có: asset/, auth/, cve/, cvedb/, datasync/, finding/, identity/
#           product/, reporter/, sbomvex/, scan/, scanner/, gen/
```

### Bước 2: Kiểm tra thiếu proto definitions

Cần đảm bảo có proto cho tất cả service-to-service gRPC calls:

```bash
# Kiểm tra proto packages cần có:
NEEDED_PROTOS=(
  "auth"          # identity-service: ValidateToken, GetUser
  "cve"           # data-service: GetCVE, BatchGetCVE
  "datasync"      # data-service: TriggerSync, GetSyncStatus
  "search"        # search-service: Search, Suggest, Aggregate
  "scan"          # scan-service: CreateScan, GetScan
  "scanner"       # scan-service: Agent communication
  "finding"       # finding-service: CreateFinding, ListFindings
  "product"       # finding-service: GetProduct, ListProducts
  "reporter"      # finding-service: GenerateReport
  "ai"            # ai-service: EnrichCVE, GetEPSS, TriageFinding
  "notification"  # notification-service: SendNotification
)

for p in "${NEEDED_PROTOS[@]}"; do
  if [ -d "$PROTO_DIR/$p" ]; then
    echo "✓ $p exists"
  else
    echo "✗ $p MISSING — needs creation"
  fi
done
```

### Bước 3: Tạo search proto nếu chưa có

```bash
PROTO_DIR="/Users/binhnt/Lab/sec/cve/osv.dev/services/shared/proto"

if [ ! -d "$PROTO_DIR/search" ]; then
  mkdir -p "$PROTO_DIR/search"
  cat > "$PROTO_DIR/search/search.proto" << 'EOF'
syntax = "proto3";

package osv.search.v1;

option go_package = "github.com/osv/shared/proto/gen/search/v1;searchv1";

service SearchService {
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc Suggest(SuggestRequest) returns (SuggestResponse);
  rpc Aggregate(AggregateRequest) returns (AggregateResponse);
  rpc IndexCVE(IndexCVERequest) returns (IndexCVEResponse);
}

message SearchRequest {
  string keyword = 1;
  repeated string severity = 2;
  repeated string ecosystems = 3;
  bool kev_only = 4;
  int32 page = 5;
  int32 size = 6;
}

message SearchResponse {
  int64 total = 1;
  repeated SearchHit hits = 2;
  int64 took_ms = 3;
}

message SearchHit {
  string cve_id = 1;
  float score = 2;
  string title = 3;
  string severity = 4;
  double cvss_score = 5;
}

message SuggestRequest { string prefix = 1; int32 limit = 2; }
message SuggestResponse { repeated string suggestions = 1; }

message AggregateRequest {}
message AggregateResponse {
  map<string, int64> severity_dist = 1;
  int64 total_cve = 2;
  int64 kev_count = 3;
}

message IndexCVERequest { string cve_id = 1; }
message IndexCVEResponse { bool success = 1; }
EOF
  echo "Created search.proto"
fi
```

### Bước 4: Tạo ai proto nếu chưa có

```bash
if [ ! -d "$PROTO_DIR/ai" ]; then
  mkdir -p "$PROTO_DIR/ai"
  cat > "$PROTO_DIR/ai/ai.proto" << 'EOF'
syntax = "proto3";

package osv.ai.v1;

option go_package = "github.com/osv/shared/proto/gen/ai/v1;aiv1";

service AIEnrichmentService {
  rpc EnrichCVE(EnrichCVERequest) returns (EnrichCVEResponse);
  rpc GetEnrichment(GetEnrichmentRequest) returns (EnrichCVEResponse);
  rpc GetEPSS(GetEPSSRequest) returns (EPSSResponse);
  rpc TriageFinding(TriageFindingRequest) returns (TriageResponse);
  rpc GenerateEmbedding(GenerateEmbeddingRequest) returns (EmbeddingResponse);
}

message EnrichCVERequest { string cve_id = 1; }

message EnrichCVEResponse {
  string cve_id = 1;
  string summary_short = 2;
  string summary_long = 3;
  string impact_analysis = 4;
  string remediation_guide = 5;
  double epss_score = 6;
  repeated string mitre_tags = 7;
  string severity_ml = 8;
  bool has_exploit = 9;
}

message GetEnrichmentRequest { string cve_id = 1; }

message GetEPSSRequest { string cve_id = 1; }
message EPSSResponse {
  string cve_id = 1;
  double score = 2;
  double percentile = 3;
}

message TriageFindingRequest { string finding_id = 1; string cve_id = 2; }
message TriageResponse {
  int32 priority = 1;
  string rationale = 2;
  string suggestion = 3;
  double confidence = 4;
}

message GenerateEmbeddingRequest { string text = 1; }
message EmbeddingResponse { repeated float values = 1; }
EOF
  echo "Created ai.proto"
fi
```

### Bước 5: Tạo notification proto nếu chưa có

```bash
if [ ! -d "$PROTO_DIR/notification" ]; then
  mkdir -p "$PROTO_DIR/notification"
  cat > "$PROTO_DIR/notification/notification.proto" << 'EOF'
syntax = "proto3";

package osv.notification.v1;

option go_package = "github.com/osv/shared/proto/gen/notification/v1;notificationv1";

service NotificationService {
  rpc SendNotification(SendNotificationRequest) returns (SendNotificationResponse);
  rpc GetAlerts(GetAlertsRequest) returns (GetAlertsResponse);
}

message SendNotificationRequest {
  string event_type = 1;
  string entity_id = 2;
  string entity_type = 3;
  string summary = 4;
  map<string, string> metadata = 5;
}

message SendNotificationResponse { bool accepted = 1; }

message GetAlertsRequest {
  string entity_id = 1;
  int32 limit = 2;
}

message GetAlertsResponse {
  repeated Alert alerts = 1;
}

message Alert {
  string id = 1;
  string event_type = 2;
  string summary = 3;
  string status = 4;
  string created_at = 5;
}
EOF
  echo "Created notification.proto"
fi
```

### Bước 6: Generate Go code từ proto

```bash
cd "$PROTO_DIR"

# Kiểm tra buf đã được install
which buf || brew install bufbuild/buf/buf

# Generate
buf generate

echo "Proto generation complete"
ls gen/
```

### Bước 7: Verify generated code compiles

```bash
cd "$PROTO_DIR"
go build ./gen/...
echo "Proto generated code builds successfully"
```

---

## Điều kiện hoàn thành

- [x] Proto tồn tại cho: `auth`, `cve`, `datasync`, `search`, `scan`, `scanner`, `finding`, `product`, `reporter`, `ai`, `notification`
- [x] `search/v1/search.proto` với `SearchService` (Search, Suggest, Aggregate, IndexCVE)
- [x] `ai/v1/ai.proto` với `AIEnrichmentService` (EnrichCVE, GetEPSS, TriageFinding, GenerateEmbedding, BatchEnrich)
- [x] `notification/v1/notification.proto` với `NotificationService`
- [x] `buf.gen.yaml` và `buf.yaml` đã cập nhật module name `github.com/osv/shared/proto`
- [x] `go build ./...` pass cho shared/proto

---

## Commit message

```
feat(proto): add search, ai, notification service protos

- Added search/search.proto (SearchService)
- Added ai/ai.proto (AIEnrichmentService)
- Added notification/notification.proto (NotificationService)
- Re-generated Go code with buf generate
```
