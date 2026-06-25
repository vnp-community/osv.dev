# P2 — Feature Completion (Trung hạn)

> **Bugs**: MOCK-001, MOCK-004, MOCK-006, MOCK-009, MOCK-010, MOCK-013  
> **Mức độ**: 🟡 Medium — Features bị disabled hoàn toàn  
> **Timeline**: Tuần 3-4

---

## MOCK-001 — Fix: Wire PostgreSQL ReportRepo + MinIO Storage

### Vấn đề
`nilReportRepo` thay thế real repo → `POST /api/v1/reports` không lưu gì.

### Giải pháp

Theo `01-architecture.md §3.5`: Report service được embed trong finding-service, dùng MinIO artifact storage.

#### Bước 1: Tạo PostgreSQL ReportRepo

**File mới**: `services/finding-service/internal/infra/postgres/report_repo.go`

```go
package postgres

import (
    "context"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/finding-service/internal/domain/report"
)

type ReportRepo struct {
    db *pgxpool.Pool
}

func NewReportRepo(db *pgxpool.Pool) *ReportRepo {
    return &ReportRepo{db: db}
}

func (r *ReportRepo) Save(ctx context.Context, rep *report.Report) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO reports (id, product_id, title, format, status, storage_key,
                            generated_by, generated_at, error_msg, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (id) DO UPDATE SET
            status = EXCLUDED.status,
            storage_key = EXCLUDED.storage_key,
            generated_at = EXCLUDED.generated_at,
            error_msg = EXCLUDED.error_msg
    `, rep.ID, rep.ProductID, rep.Title, rep.Format, rep.Status,
       rep.StorageKey, rep.GeneratedBy, rep.GeneratedAt, rep.ErrorMsg, rep.CreatedAt)
    return err
}

func (r *ReportRepo) FindByID(ctx context.Context, id string) (*report.Report, error) {
    row := r.db.QueryRow(ctx, `
        SELECT id, product_id, title, format, status, storage_key,
               generated_by, generated_at, error_msg, created_at
        FROM reports WHERE id = $1
    `, id)
    rep := &report.Report{}
    err := row.Scan(&rep.ID, &rep.ProductID, &rep.Title, &rep.Format,
        &rep.Status, &rep.StorageKey, &rep.GeneratedBy, &rep.GeneratedAt,
        &rep.ErrorMsg, &rep.CreatedAt)
    if err != nil {
        return nil, err
    }
    return rep, nil
}

func (r *ReportRepo) ListByProduct(ctx context.Context, productID, userID string, limit, offset int) ([]*report.Report, int, error) {
    // Đếm total
    var total int
    r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM reports
        WHERE ($1 = '' OR product_id = $1) AND ($2 = '' OR generated_by = $2)
    `, productID, userID).Scan(&total)

    rows, err := r.db.Query(ctx, `
        SELECT id, product_id, title, format, status, storage_key,
               generated_by, generated_at, error_msg, created_at
        FROM reports
        WHERE ($1 = '' OR product_id = $1) AND ($2 = '' OR generated_by = $2)
        ORDER BY created_at DESC LIMIT $3 OFFSET $4
    `, productID, userID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    // scan rows...
    return reports, total, nil
}

func (r *ReportRepo) Delete(ctx context.Context, id string) error {
    _, err := r.db.Exec(ctx, "DELETE FROM reports WHERE id = $1", id)
    return err
}

func (r *ReportRepo) DeleteExpired(ctx context.Context) (int, error) {
    result, err := r.db.Exec(ctx,
        "DELETE FROM reports WHERE created_at < NOW() - INTERVAL '30 days'")
    return int(result.RowsAffected()), err
}
```

#### Bước 2: Tạo MinIO Storage Adapter

**File mới**: `services/finding-service/internal/infra/minio/report_storage.go`

```go
package minio

import (
    "context"
    "time"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

// ReportStorage implements reportuc.Storage using MinIO S3-compatible storage.
type ReportStorage struct {
    client *minio.Client
    bucket string
}

func NewReportStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*ReportStorage, error) {
    client, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        return nil, fmt.Errorf("minio init: %w", err)
    }
    return &ReportStorage{client: client, bucket: bucket}, nil
}

// PresignedURL generates a pre-signed GET URL (15min TTL).
func (s *ReportStorage) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
    u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
    if err != nil {
        return "", fmt.Errorf("presigned url: %w", err)
    }
    return u.String(), nil
}

// Upload uploads report bytes to MinIO.
func (s *ReportStorage) Upload(ctx context.Context, key string, data []byte, contentType string) error {
    _, err := s.client.PutObject(ctx, s.bucket, key,
        bytes.NewReader(data), int64(len(data)),
        minio.PutObjectOptions{ContentType: contentType})
    return err
}
```

#### Bước 3: Sửa embedded.go — Wire real Report components

**File sửa**: `services/finding-service/embedded.go:L64-L68`

```go
// CR-010: ReportHandler — wire real components khi MinIO được cấu hình
reportRepo := postgres.NewReportRepo(pool)

var generateUC *reportuc.GenerateUseCase
var reportStorage reportuc.Storage

minioEndpoint  := os.Getenv("MINIO_ENDPOINT")     // e.g., "localhost:9000"
minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
minioBucket    := os.Getenv("MINIO_REPORT_BUCKET")
if minioBucket == "" {
    minioBucket = "osv-reports"
}

if minioEndpoint != "" && minioAccessKey != "" {
    storage, err := miniorepo.NewReportStorage(
        minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, false)
    if err != nil {
        logger.Warn().Err(err).Msg("MinIO storage init failed, report generation disabled")
    } else {
        reportStorage = storage
        generateUC = reportuc.NewGenerateUseCase(findingRepo, reportRepo, storage)
        logger.Info().Str("bucket", minioBucket).Msg("Report generation enabled")
    }
} else {
    logger.Warn().Msg("MINIO_ENDPOINT not set, report generation disabled (GET templates still works)")
}

// Wire handler với real components (generateUC có thể nil — được guard bởi MOCK-002 fix)
reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, reportStorage)
```

#### Schema DB cần tạo
```sql
CREATE TABLE IF NOT EXISTS reports (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   VARCHAR(36) NOT NULL,
    title        VARCHAR(255) NOT NULL,
    format       VARCHAR(10) NOT NULL,  -- pdf|xlsx|json|csv
    status       VARCHAR(20) DEFAULT 'pending',  -- pending|processing|completed|failed
    storage_key  TEXT,                  -- MinIO object key
    generated_by VARCHAR(36),           -- user ID
    generated_at TIMESTAMPTZ,
    error_msg    TEXT,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_reports_product_id ON reports(product_id);
CREATE INDEX idx_reports_generated_by ON reports(generated_by);
```

---

## MOCK-004 — Fix: Wire 7 nil Handlers trong finding-service

### Vấn đề
6 nhóm handlers là `nil` → routes trả 404.

### Giải pháp

Mỗi handler cần được khởi tạo từ PostgreSQL repo tương ứng. Theo domain model `01-architecture.md §3.5`:

```
ProductType → Product → Engagement → Test → Finding → RiskAcceptance
```

**File sửa**: `services/finding-service/embedded.go`

```go
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
    // === Repositories ===
    productTypeRepo  := postgres.NewProductTypeRepo(pool)
    engagementRepo   := postgres.NewEngagementRepo(pool)
    testRepo         := postgres.NewTestRepo(pool)
    productRepo      := postgres.NewProductRepo(pool)
    findingRepo      := postgres.NewFindingRepo(pool)
    findingGroupRepo := postgres.NewFindingGroupRepo(pool)
    noteRepo         := postgres.NewNoteRepo(pool)
    memberRepo       := postgres.NewMemberRepo(pool)        // FIX MOCK-004
    toolConfigRepo   := postgres.NewToolConfigRepo(pool)    // FIX MOCK-004
    riskAcceptRepo   := postgres.NewRiskAcceptanceRepo(pool) // FIX MOCK-004

    // === NATS publisher (giữ nguyên) ===
    /* ... */

    // === Use Cases ===
    bulkUC   := findinguc.NewBulkUpdate(findingRepo, pub)
    statusUC := findinguc.NewStatusTransition(findingRepo, pub)

    // InternalUseCase: aggregate stats cho BFF dashboard
    // Theo 01-architecture.md: DashboardBFF fan-out tới /internal/stats
    internalUC := findinguc.NewInternalStats(findingRepo, productRepo, pub) // FIX MOCK-004

    // SLA use case: cần findingRepo + slaConfigRepo
    slaConfigRepo := postgres.NewSLAConfigRepo(pool)
    slaUC         := findinguc.NewSLADashboard(findingRepo, slaConfigRepo)   // FIX MOCK-004

    // === Handlers ===
    productSeed  := httpdelivery.NewProductSeedHandler(productTypeRepo, productRepo, engagementRepo, testRepo, logger)
    findingSeed  := httpdelivery.NewFindingSeedHandler(findingRepo, testRepo, engagementRepo, logger)
    findingGroup := httpdelivery.NewFindingGroupHandler(findingGroupRepo, logger)
    bulkHandler  := httpdelivery.NewBulkHandler(bulkUC, findingRepo, pub, logger) // MOCK-003 fix: pub not nil
    productHandler  := httpdelivery.NewProductHandler(productRepo, logger)
    findingHandler  := httpdelivery.NewFindingHandler(findingRepo, statusUC, logger)
    noteHandler     := httpdelivery.NewNoteHandler(findingRepo, noteRepo)

    // FIX MOCK-004: Wire các handlers còn thiếu
    engagementHandler  := httpdelivery.NewEngagementHandler(engagementRepo, logger)
    testHandler        := httpdelivery.NewTestHandler(testRepo, engagementRepo, logger)
    memberHandler      := httpdelivery.NewMemberHandler(memberRepo, productRepo, logger)
    toolHandler        := httpdelivery.NewToolConfigHandler(toolConfigRepo, logger)
    riskAcceptHandler  := httpdelivery.NewRiskAcceptanceHandler(riskAcceptRepo, findingRepo, pub, logger)
    internalHandler    := httpdelivery.NewInternalHandler(internalUC, logger)
    slaHandler         := httpdelivery.NewSLAHandler(slaUC, logger)

    // FIX MOCK-001: Report handler với real components
    reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, reportStorage)

    router := httpdelivery.NewRouter(
        findingHandler,    // ✅ FindingHandler
        bulkHandler,       // ✅ BulkHandler
        noteHandler,       // ✅ NoteHandler
        engagementHandler, // ✅ FIX: EngagementHandler
        testHandler,       // ✅ FIX: TestHandler
        memberHandler,     // ✅ FIX: MemberHandler
        toolHandler,       // ✅ FIX: ToolHandler
        reportHandler,     // ✅ FIX: ReportHandler (real)
        riskAcceptHandler, // ✅ FIX: RiskAcceptanceHandler
        internalHandler,   // ✅ FIX: InternalHandler
        slaHandler,        // ✅ FIX: SLAHandler
        productHandler,    // ✅ ProductHandler
        productSeed,       // ✅ ProductSeedHandler
        findingSeed,       // ✅ FindingSeedHandler
        findingGroup,      // ✅ FindingGroupHandler
        logger,
    )
    /* ... */
}
```

---

## MOCK-006 — Fix: Wire ScanRepo, ScanAPIHandler, StatsRepo

### Vấn đề
3 handlers dùng `nil` repo → scan data không được lưu.

### Giải pháp

**File sửa**: `services/scan-service/embedded.go`

```go
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
    // FIX MOCK-006: Wire real PostgreSQL repositories
    scanRepo  := postgres.NewScanRepo(pool)
    statsRepo := postgres.NewScanStatsRepo(pool)
    agentRepo := postgres.NewAgentRepo(pool)  // FIX MOCK-007

    // Wire handlers với real repos
    agentHandler := httpdelivery.NewAgentHandler(agentRepo, logger)
    scanHandler  := httpdelivery.NewScanAPIHandler(scanRepo, logger)
    statsHandler := httpdelivery.NewStatsHandler(statsRepo, logger)

    router := httpdelivery.NewRouterFull(
        nil,          // importHandler — chưa wire
        nil,          // parserHandler — chưa wire
        agentHandler, // ✅ FIX: real repo
        scanHandler,  // ✅ FIX: real repo
        nil,          // scheduleHandler — chưa wire
        statsHandler, // ✅ FIX: real repo
        logger,
    )
    mux.Handle("/", router)
    return nil
}
```

#### Tables cần tạo
```sql
CREATE TABLE IF NOT EXISTS scans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  VARCHAR(36),
    test_id     VARCHAR(36),
    tool        VARCHAR(50) NOT NULL,
    status      VARCHAR(20) DEFAULT 'pending',
    targets     JSONB DEFAULT '[]',
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    created_by  VARCHAR(36)
);
CREATE INDEX idx_scans_status ON scans(status);
CREATE INDEX idx_scans_product_id ON scans(product_id);
```

---

## MOCK-009 — Fix: Wire OpenSearch Client

### Vấn đề
`cvesearch.New(..., nil, ...)` — OpenSearch bị bỏ qua.

### Giải pháp

Theo `01-architecture.md §3.3`: Search-service dùng dual backend — OpenSearch trước, fallback Postgres.

**File sửa**: `services/search-service/embedded.go:L96`

```go
// FIX MOCK-009: Wire OpenSearch client khi OPENSEARCH_URL được cấu hình
var osClient *opensearch.Client

osURL := os.Getenv("OPENSEARCH_URL")  // e.g., "http://localhost:9200"
if osURL != "" {
    var err error
    osClient, err = opensearch.NewClient(opensearch.Config{
        URL:      osURL,
        Username: os.Getenv("OPENSEARCH_USERNAME"),
        Password: os.Getenv("OPENSEARCH_PASSWORD"),
    })
    if err != nil {
        log.Warn().Err(err).Msg("search-service: OpenSearch init failed, using Postgres FTS")
        osClient = nil
    } else {
        log.Info().Str("url", osURL).Msg("search-service: OpenSearch enabled")
    }
}

// Wire searchUC với osClient (có thể nil → fallback tự động sang Postgres)
// Theo 02-technical-design.md §11.1: SearchUseCase tự handle fallback
searchUC := cvesearch.New(cveRepo, cacheRepo, osClient, log)
```

---

## MOCK-010 — Fix: Wire InternalHandler cho OpenSearch Indexing

### Vấn đề
`NewRouter(..., nil, ...)` — InternalHandler nil → `/internal/opensearch/index` bị tắt.

### Giải pháp

**File sửa**: `services/search-service/embedded.go:L101`

```go
// FIX MOCK-010: Wire InternalHandler khi osClient available
var internalH *deliveryhttp.InternalHandler
if osClient != nil {
    internalH = deliveryhttp.NewInternalHandler(osClient, log)
}

// Wire router với internalH (nil-safe — router có if internalH != nil guard)
router := deliveryhttp.NewRouter(h, taxH, vendorH, internalH, statsH, log)
```

---

## MOCK-013 — Fix: SearchAddr trong EmbeddedConfig

### Vấn đề
`search-service` hardcoded `http://localhost:8083` ở 3 chỗ trong gateway.

### Giải pháp

**File sửa**: `services/gateway-service/embedded.go`

```go
// EmbeddedConfig — thêm SearchAddr field
type EmbeddedConfig struct {
    JWTSecret        string
    IdentityAddr     string
    DataAddr         string
    SearchAddr       string  // ✅ ĐÃ CÓ — nhưng chưa được dùng → FIX bên dưới
    FindingAddr      string
    ScanAddr         string
    NotificationAddr string
    AIAddr           string
    RankingAddr      string
    AssetAddr        string
    ProductAddr      string
    SLAAddr          string
}

func WireEmbedded(ctx context.Context, log zerolog.Logger, rdb *redis.Client,
    cfg EmbeddedConfig, mux *http.ServeMux) error {
    // ...
    identityHTTP     := coalesce(cfg.IdentityAddr,     "http://localhost:8081")
    dataHTTP         := coalesce(cfg.DataAddr,         "http://localhost:8082")
    // FIX MOCK-013: dùng cfg.SearchAddr thay vì hardcode
    searchHTTP        := coalesce(cfg.SearchAddr,       "http://localhost:8083")
    findingHTTP      := coalesce(cfg.FindingAddr,      "http://localhost:8085")
    // ... các addr khác giữ nguyên

    upstreamURLs := map[string]string{
        "identity-service":     identityHTTP,
        "data-service":         dataHTTP,
        "search-service":       searchHTTP,   // ✅ FIX: dùng searchHTTP
        "notification-service": notificationHTTP,
        "finding-service":      findingHTTP,
        "ai-service":           aiHTTP,
        "scan-service":         scanHTTP,
        "asset-service":        assetHTTP,
        "product-service":      productHTTP,
        "sla-service":          slaHTTP,

        // Logical aliases
        "identity":          identityHTTP,
        "finding-mgmt":      findingHTTP,
        "sla":               slaHTTP,
        "notification":      notificationHTTP,
        "product-mgmt":      findingHTTP,
        "search":            searchHTTP,      // ✅ FIX: dùng searchHTTP
        "ai":                aiHTTP,
        "scan-orchestrator": scanHTTP,
        "report":            findingHTTP,
        "audit":             findingHTTP,
        "jira":              findingHTTP,
    }

    // UIAPIHandler cũng dùng searchHTTP
    uiAPI := handlers.NewUIAPIHandler(map[string]string{
        "data":         dataHTTP,
        "search":       searchHTTP,      // ✅ FIX: dùng searchHTTP
        "finding":      findingHTTP,
        // ...
    })
}
```

#### Cập nhật monolith main để pass SearchAddr

**File sửa**: `apps/osv/main.go` hoặc `apps/osv/cmd/server/main.go`

```go
gatewayCfg := gateway.EmbeddedConfig{
    JWTSecret:        os.Getenv("JWT_SECRET"),
    IdentityAddr:     os.Getenv("IDENTITY_SERVICE_ADDR"),
    SearchAddr:       os.Getenv("SEARCH_SERVICE_ADDR"),  // ← thêm
    FindingAddr:      os.Getenv("FINDING_SERVICE_ADDR"),
    // ...
}
```
