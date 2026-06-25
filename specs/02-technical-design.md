# OSV Platform — Technical Design Document (TDD)

> **Version:** 3.0  
> **Date:** 2026-06-23  
> **Status:** Active  
> **Project:** OSV Platform — Go Microservices  

---

## 1. Giới Thiệu

### 1.1 Mục Đích Tài Liệu

Tài liệu này mô tả chi tiết kỹ thuật của từng component trong hệ thống OSV Platform Go Microservices, bao gồm:
- Design decisions và rationale
- Interface contracts giữa các services
- Data schemas và storage design
- Algorithm design cho các tính năng quan trọng
- Error handling và recovery strategies

### 1.2 Phạm Vi

Tài liệu bao phủ toàn bộ hệ thống:
- API Gateway (`apps/osv`)
- Data-service (CVE ingestion + enrichment)
- Search-service (OpenSearch + pgvector)
- Finding-service (Product hierarchy + findings)
- Scan-service (Parser factory + dedup engine)
- SLA-service, Notification-service, JIRA-service, Audit-service
- Identity-service, Ranking-service
- **[v3.0 Planned]** Auth-service, Scan-service-OVS, Finding-service-OVS, Product-service, AI-service, Report-service, Asset-service

---

## 2. Clean Architecture Design

### 2.1 Layer Structure

Mỗi service tuân theo Clean Architecture với 4 layers:

```
domain/        # Entities, value objects, interfaces, domain events
usecase/       # Business logic, use case orchestration
adapter/       # HTTP handlers, NATS publishers/subscribers, gRPC adapters
infra/         # PostgreSQL repositories, Redis cache, OpenSearch client
```

### 2.2 Dependency Rule

Dependencies chỉ được phép đi vào trong (outer → inner):
- Infrastructure → Adapter → Use Case → Domain
- Domain KHÔNG được import bất kỳ outer layer nào

```go
// domain/finding.go — pure Go, no external deps
type Finding struct {
    ID            uuid.UUID
    Title         string
    Severity      Severity  // value object
    State         FindingState
    HashCode      string
    DuplicateOfID *uuid.UUID
    SLAExpiry     *time.Time
    ProductID     uuid.UUID
    TestID        uuid.UUID
}

// usecase/finding_usecase.go — depends only on domain interfaces
type FindingUseCase struct {
    repo    FindingRepository  // interface
    nats    EventPublisher     // interface
    auditor AuditLogger        // interface
}

// infra/postgres/finding_repo.go — implements domain.FindingRepository
type PostgresFindingRepository struct {
    db *pgxpool.Pool
}
```

### 2.3 Repository Pattern

```go
// domain/interfaces.go
type FindingRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*Finding, error)
    GetByHash(ctx context.Context, hash string, productID uuid.UUID) (*Finding, error)
    Create(ctx context.Context, f *Finding) error
    Update(ctx context.Context, f *Finding) error
    ListByProduct(ctx context.Context, productID uuid.UUID, filter FindingFilter) ([]Finding, error)
}
```

---

## 3. API Gateway Technical Design

### 3.1 Reverse Proxy Architecture

```go
// apps/osv/internal/gateway/proxy.go
type ReverseProxy struct {
    transport *http.Transport
    timeout   time.Duration
}

func (p *ReverseProxy) Forward(target string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Construct upstream URL: target + r.URL.Path + r.URL.RawQuery
        upstreamURL := "http://" + target + r.URL.RequestURI()
        
        // Clone request, set upstream URL
        req, _ := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
        req.Header = r.Header.Clone()
        
        // Execute
        resp, err := p.transport.RoundTrip(req)
        // Copy response headers + body
    }
}

// Variants:
func (p *ReverseProxy) ForwardWithTimeout(target string, timeout time.Duration)
func (p *ReverseProxy) ForwardWithMaxBody(target string, maxBytes int64)
```

### 3.2 Authentication Middleware

```go
// apps/osv/internal/gateway/auth/middleware.go

type AuthMiddleware interface {
    Authenticate(http.Handler) http.Handler
}

func (m *authMiddleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var (
            userID    string
            userRole  string
            userPerms []string
        )
        
        // Try JWT first
        if bearer := extractBearer(r); bearer != "" {
            claims, err := m.validateJWT(bearer)
            if err == nil {
                userID = claims.Sub
                userRole = claims.Role
                userPerms = claims.Permissions
                goto authorized
            }
        }
        
        // Try API Key
        if apiKey := r.Header.Get("X-Api-Key"); apiKey != "" {
            key, err := m.identityClient.ValidateAPIKey(r.Context(), apiKey)
            if err == nil && !key.Revoked && !key.Expired() {
                userID = key.UserID.String()
                userRole = key.Role
                userPerms = key.Scopes
                goto authorized
            }
        }
        
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
        
    authorized:
        // Inject headers for upstream
        r = r.WithContext(r.Context())
        r.Header.Set("X-User-ID", userID)
        r.Header.Set("X-User-Role", userRole)
        r.Header.Set("X-User-Perms", strings.Join(userPerms, ","))
        next.ServeHTTP(w, r)
    })
}
```

### 3.3 Rate Limiting (Redis Token Bucket)

```go
// apps/osv/internal/gateway/ratelimit/limiter.go

type RateLimiter struct {
    redis *redis.Client
}

// Limit creates middleware for a rate limit spec like "60/minute"
func (rl *RateLimiter) Limit(spec string) func(http.Handler) http.Handler {
    limit, window := parseSpec(spec) // e.g., 60, time.Minute
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := extractIP(r)
            key := "ratelimit:" + ip + ":" + spec
            
            // Sliding window counter via Redis
            now := time.Now().UnixMilli()
            windowStart := now - window.Milliseconds()
            
            pipe := rl.redis.Pipeline()
            pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(windowStart, 10))
            pipe.ZCard(ctx, key)
            pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
            pipe.Expire(ctx, key, window)
            results, _ := pipe.Exec(ctx)
            
            count := results[1].(*redis.IntCmd).Val()
            if count >= int64(limit) {
                w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
                w.Header().Set("Retry-After", "60")
                http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 4. Data-Service Technical Design

### 4.1 Fetcher Registry & Scheduler

```go
// services/data-service/internal/fetcher/registry.go

type FetcherRegistry struct {
    fetchers map[string]CVEFetcher
    mu       sync.RWMutex
}

func (r *FetcherRegistry) Register(f CVEFetcher) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.fetchers[f.Source()] = f
}

// Scheduler runs each fetcher per its configured interval
type FetchScheduler struct {
    registry *FetcherRegistry
    repo     CVERepository
    nats     *nats.Conn
}

func (s *FetchScheduler) Run(ctx context.Context) {
    // cron entries per fetcher
    for name, interval := range fetchIntervals {
        go s.runFetcher(ctx, name, interval)
    }
}

func (s *FetchScheduler) runFetcher(ctx context.Context, name string, interval time.Duration) {
    ticker := time.NewTicker(interval)
    for {
        select {
        case <-ticker.C:
            fetcher := s.registry.Get(name)
            since := s.repo.GetLastSync(name)
            
            ch, err := fetcher.FetchSince(ctx, since)
            if err != nil {
                log.Error().Err(err).Str("fetcher", name).Msg("fetch failed")
                continue
            }
            
            for record := range ch {
                s.upsert(ctx, record)
            }
            s.repo.UpdateLastSync(name, time.Now())
            
        case <-ctx.Done():
            return
        }
    }
}
```

### 4.2 CVE Upsert Pipeline

```go
// services/data-service/internal/pipeline/upsert.go

func (p *CVEPipeline) Upsert(ctx context.Context, record CVERecord) error {
    // 1. Normalize
    cve := normalize(record)
    
    // 2. PostgreSQL upsert
    err := p.repo.Upsert(ctx, cve)
    if err != nil {
        return fmt.Errorf("upsert postgres: %w", err)
    }
    
    // 3. OpenSearch index (fire-and-forget)
    go func() {
        if err := p.search.Index(context.Background(), cve); err != nil {
            log.Warn().Err(err).Str("cve_id", cve.ID).Msg("opensearch index failed")
        }
    }()
    
    // 4. NATS publish
    evt := &CVESyncedEvent{
        CveID:  cve.ID,
        Action: "updated", // or "created"
    }
    return p.nats.PublishJSON("ingestion.cve.synced", evt)
}
```

### 4.3 EPSS Daily Sync

```go
// services/data-service/internal/sync/epss.go

func (s *EPSSSyncer) Sync(ctx context.Context) error {
    // Download CSV.GZ from FIRST.org
    url := "https://epss.cyentia.com/epss_scores-current.csv.gz"
    resp, err := s.httpClient.Get(url)
    // ...
    
    gz, err := gzip.NewReader(resp.Body)
    reader := csv.NewReader(gz)
    
    // Skip header (model_version, score_date)
    header, _ := reader.Read()
    _ = header
    
    // Batch update
    batch := make([]EPSSScore, 0, 1000)
    for {
        row, err := reader.Read()
        if err == io.EOF { break }
        
        // row[0]=cve, row[1]=epss_score, row[2]=percentile
        batch = append(batch, EPSSScore{
            CveID:      row[0],
            Score:      parseFloat(row[1]),
            Percentile: parseFloat(row[2]),
        })
        
        if len(batch) >= 1000 {
            s.repo.BatchUpdateEPSS(ctx, batch)
            batch = batch[:0]
        }
    }
    
    return s.repo.BatchUpdateEPSS(ctx, batch)
}
```

### 4.4 KEV Diff Detection

```go
// services/data-service/internal/sync/kev.go

func (s *KEVSyncer) Sync(ctx context.Context) error {
    // Fetch current KEV catalog
    resp, _ := s.httpClient.Get("https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json")
    var catalog KEVCatalog
    json.NewDecoder(resp.Body).Decode(&catalog)
    
    // Get existing KEV IDs from DB
    existing, _ := s.repo.GetAllKEVIDs(ctx)
    existingSet := toSet(existing)
    
    var newEntries []KEVEntry
    for _, vuln := range catalog.Vulnerabilities {
        if !existingSet.Contains(vuln.CveID) {
            newEntries = append(newEntries, vuln)
        }
    }
    
    if len(newEntries) > 0 {
        // Upsert new entries
        s.repo.UpsertKEV(ctx, newEntries)
        
        // Publish NATS event for new KEV entries
        s.nats.PublishJSON("kev.new", &KEVNewEvent{
            CveIDs:    extractIDs(newEntries),
            DateAdded: time.Now().UTC(),
        })
    }
    
    return nil
}
```

---

## 5. Finding-Service Technical Design

### 5.1 Finding State Machine

```go
// services/finding-service/internal/domain/finding_state.go

type FindingState string

const (
    StateActive       FindingState = "Active"
    StateMitigated    FindingState = "Mitigated"
    StateFalsePositive FindingState = "FalsePositive"
    StateRiskAccepted FindingState = "RiskAccepted"
    StateOutOfScope   FindingState = "OutOfScope"
    StateDuplicate    FindingState = "Duplicate"
)

// Valid transitions
var validTransitions = map[FindingState][]FindingState{
    StateActive: {
        StateMitigated, StateFalsePositive,
        StateRiskAccepted, StateOutOfScope, StateDuplicate,
    },
    StateMitigated:    {StateActive}, // reopen
    StateFalsePositive: {StateActive},
    StateRiskAccepted: {StateActive},
    StateOutOfScope:   {StateActive},
    StateDuplicate:    {}, // no transitions from Duplicate
}

func (f *Finding) TransitionTo(newState FindingState, userID uuid.UUID) error {
    allowed := validTransitions[f.State]
    for _, s := range allowed {
        if s == newState {
            f.PreviousState = f.State
            f.State = newState
            f.ModifiedAt = time.Now()
            f.ModifiedBy = userID
            return nil
        }
    }
    return fmt.Errorf("invalid transition: %s → %s", f.State, newState)
}
```

### 5.2 Deduplication Algorithm

```go
// services/finding-service/internal/domain/dedup.go

func ComputeHashCode(title, componentName, componentVersion, cveID string) string {
    data := title + "|" + componentName + "|" + componentVersion + "|" + cveID
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// Use case: on finding create
func (uc *FindingUseCase) Create(ctx context.Context, req CreateFindingRequest) (*Finding, error) {
    hash := ComputeHashCode(req.Title, req.Component, req.Version, req.CveID)
    
    // Check for existing non-duplicate finding with same hash in same product
    existing, err := uc.repo.GetByHash(ctx, hash, req.ProductID)
    if err == nil && existing != nil {
        // Create as duplicate
        return uc.createDuplicate(ctx, req, hash, existing.ID)
    }
    
    // Create as new finding
    f := &Finding{
        ID:            uuid.New(),
        Title:         req.Title,
        HashCode:      hash,
        State:         StateActive,
        SLAExpiry:     computeSLAExpiry(req.Severity),
        ProductID:     req.ProductID,
    }
    
    if err := uc.repo.Create(ctx, f); err != nil {
        return nil, err
    }
    
    uc.nats.Publish("finding.created", FindingCreatedEvent{
        FindingID: f.ID,
        Severity:  f.Severity,
        ProductID: f.ProductID,
    })
    
    return f, nil
}
```

### 5.3 SLA Computation

```go
// services/finding-service/internal/domain/sla.go

type SLAConfig struct {
    CriticalDays int
    HighDays     int
    MediumDays   int
    LowDays      int
}

var defaultSLA = SLAConfig{
    CriticalDays: 7,
    HighDays:     30,
    MediumDays:   90,
    LowDays:      180,
}

func computeSLAExpiry(severity Severity, cfg *SLAConfig) *time.Time {
    if cfg == nil {
        cfg = &defaultSLA
    }
    
    var days int
    switch severity {
    case SeverityCritical: days = cfg.CriticalDays
    case SeverityHigh:     days = cfg.HighDays
    case SeverityMedium:   days = cfg.MediumDays
    case SeverityLow:      days = cfg.LowDays
    case SeverityInfo:     return nil // no SLA
    }
    
    expiry := time.Now().UTC().AddDate(0, 0, days)
    return &expiry
}
```

### 5.4 Product Grading Algorithm

```go
// services/finding-service/internal/usecase/grading.go

type GradeResult struct {
    ProductID     uuid.UUID
    Grade         string  // "A", "B", "C", "D", "F"
    CriticalCount int
    HighCount     int
    TotalActive   int
    ComputedAt    time.Time
}

func ComputeGrade(criticalCount, highCount, totalActive int) string {
    switch {
    case criticalCount >= 3 || totalActive > 20:
        return "F"
    case criticalCount >= 1 && criticalCount <= 2:
        return "D"
    case criticalCount == 0 && highCount > 5:
        return "C"
    case criticalCount == 0 && highCount <= 5 && highCount > 0:
        return "B"
    default:
        return "A"
    }
}

func (uc *GradingUseCase) ComputeForProduct(ctx context.Context, productID uuid.UUID) (*GradeResult, error) {
    counts, err := uc.repo.GetActiveFindingCounts(ctx, productID)
    if err != nil {
        return nil, err
    }
    
    grade := ComputeGrade(counts.Critical, counts.High, counts.Total)
    
    result := &GradeResult{
        ProductID:     productID,
        Grade:         grade,
        CriticalCount: counts.Critical,
        HighCount:     counts.High,
        TotalActive:   counts.Total,
        ComputedAt:    time.Now().UTC(),
    }
    
    // Cache grade in Redis for dashboard
    uc.cache.Set(ctx, "grade:"+productID.String(), grade, 5*time.Minute)
    
    return result, nil
}
```

---

## 6. Scan-Service Technical Design

### 6.1 Parser Factory

```go
// services/scan-service/internal/parser/factory.go

type ScanParser interface {
    Parse(ctx context.Context, r io.Reader) ([]RawFinding, error)
    Name() string
}

type ParserFactory struct {
    parsers map[string]ScanParser
}

func NewParserFactory() *ParserFactory {
    f := &ParserFactory{parsers: make(map[string]ScanParser)}
    
    // Register all parsers
    f.Register(&NmapXMLParser{})
    f.Register(&ZAPJSONParser{})
    f.Register(&BanditParser{})
    f.Register(&TrivyParser{})
    f.Register(&SnykParser{})
    f.Register(&SemgrepParser{})
    f.Register(&SARIFParser{})       // Generic SARIF
    f.Register(&CycloneDXParser{})
    f.Register(&CheckmarxParser{})
    f.Register(&VeracodeParser{})
    f.Register(&BurpSuiteParser{})
    f.Register(&NessusParser{})
    f.Register(&OpenSCAPParser{})
    f.Register(&GitleaksParser{})
    f.Register(&TruffleHogParser{})
    f.Register(&HadolintParser{})
    f.Register(&TFSecParser{})
    f.Register(&KubescapeParser{})
    f.Register(&GrypeParser{})
    f.Register(&OWASPZAPParser{})
    f.Register(&RetireJSParser{})
    
    return f
}

func (f *ParserFactory) GetParser(toolName string) (ScanParser, error) {
    p, ok := f.parsers[strings.ToLower(toolName)]
    if !ok {
        return nil, fmt.Errorf("unsupported tool: %s", toolName)
    }
    return p, nil
}
```

### 6.2 Import Pipeline (12 Steps)

```go
// services/scan-service/internal/usecase/import_usecase.go

func (uc *ImportUseCase) Import(ctx context.Context, req ImportRequest) (*ImportResult, error) {
    result := &ImportResult{}
    
    // Step 1: Validate file
    if err := validateFile(req.File, req.MaxSize); err != nil {
        return nil, fmt.Errorf("step1 validate: %w", err)
    }
    
    // Step 2: Detect parser
    parser, err := uc.factory.GetParser(req.ToolName)
    if err != nil {
        return nil, fmt.Errorf("step2 detect: %w", err)
    }
    
    // Step 3: Parse
    rawFindings, err := parser.Parse(ctx, req.File)
    if err != nil {
        return nil, fmt.Errorf("step3 parse: %w", err)
    }
    
    // Step 4: Normalize
    normalized := normalizeFindings(rawFindings, req.TestID)
    
    // Step 5: Enrich with CVE data
    enriched := uc.enrichWithCVE(ctx, normalized)
    
    // Step 6: Compute hashes
    for i := range enriched {
        enriched[i].HashCode = ComputeHashCode(
            enriched[i].Title,
            enriched[i].Component,
            enriched[i].Version,
            enriched[i].CveID,
        )
    }
    
    // Step 7: Check existing (dedup)
    hashes := extractHashes(enriched)
    existing, _ := uc.findingRepo.GetByHashes(ctx, hashes, req.ProductID)
    existingMap := toHashMap(existing)
    
    // Step 8: Classify: new vs duplicate
    var toCreate, duplicates []Finding
    for _, f := range enriched {
        if orig, ok := existingMap[f.HashCode]; ok {
            f.DuplicateOfID = &orig.ID
            f.State = StateDuplicate
            duplicates = append(duplicates, f)
        } else {
            toCreate = append(toCreate, f)
        }
    }
    
    // Step 9: Compute SLA deadlines
    for i := range toCreate {
        toCreate[i].SLAExpiry = computeSLAExpiry(toCreate[i].Severity, req.SLAConfig)
    }
    
    // Step 10: Persist (transaction)
    tx, _ := uc.db.Begin(ctx)
    uc.findingRepo.BatchCreate(ctx, tx, append(toCreate, duplicates...))
    tx.Commit(ctx)
    
    // Step 11: Publish NATS event
    uc.nats.PublishJSON("finding.batch_created", BatchCreatedEvent{
        ScanID:     req.ScanID,
        FindingIDs: extractIDs(toCreate),
    })
    
    // Step 12: Return summary
    result.Created = len(toCreate)
    result.Duplicates = len(duplicates)
    result.Total = len(enriched)
    return result, nil
}
```

---

## 7. SLA-Service Technical Design

### 7.1 Breach Detection Cron

```go
// services/sla-service/internal/usecase/breach_detector.go

func (uc *BreachDetector) Run(ctx context.Context) {
    // Run daily at 00:00 UTC
    cron := gocron.NewScheduler(time.UTC)
    cron.Every(1).Day().At("00:00").Do(func() {
        uc.detectBreaches(context.Background())
    })
    cron.StartAsync()
    <-ctx.Done()
}

func (uc *BreachDetector) detectBreaches(ctx context.Context) {
    // Find all active, non-breached, overdue findings
    findings, err := uc.repo.FindBreached(ctx)
    if err != nil {
        log.Error().Err(err).Msg("breach detection query failed")
        return
    }
    
    for _, f := range findings {
        // Mark as breached
        uc.repo.MarkBreached(ctx, f.ID)
        
        // Publish event
        uc.nats.PublishJSON("finding.sla.breached", SLABreachedEvent{
            FindingID: f.ID,
            Severity:  f.Severity,
            ExpiresAt: f.SLAExpiry,
            ProductID: f.ProductID,
        })
        
        log.Info().
            Str("finding_id", f.ID.String()).
            Str("severity", string(f.Severity)).
            Time("expired_at", *f.SLAExpiry).
            Msg("SLA breach detected")
    }
}
```

---

## 8. Notification-Service Technical Design

### 8.1 Channel Dispatcher

```go
// services/notification-service/internal/usecase/dispatcher.go

type ChannelDispatcher struct {
    email   EmailSender
    slack   SlackSender
    teams   TeamsSender
    webhook WebhookSender
    alerts  AlertStore  // in-app
}

func (d *ChannelDispatcher) Dispatch(ctx context.Context, evt NotificationEvent) error {
    rules, _ := d.rulesRepo.GetMatchingRules(ctx, evt.EventType)
    
    for _, rule := range rules {
        switch rule.Channel {
        case "email":
            go d.email.Send(ctx, rule.Config, evt)
        case "slack":
            go d.slack.Send(ctx, rule.Config, evt)
        case "teams":
            go d.teams.Send(ctx, rule.Config, evt)
        case "webhook":
            go d.webhook.Send(ctx, rule.Config, evt)
        case "in-app":
            d.alerts.Store(ctx, rule.UserID, evt)
        }
    }
    
    return nil
}
```

### 8.2 Webhook Delivery with Retry

```go
// services/notification-service/internal/delivery/webhook.go

func (s *WebhookSender) Send(ctx context.Context, cfg WebhookConfig, evt NotificationEvent) error {
    payload, _ := json.Marshal(evt)
    
    // SSRF protection
    if isPrivateIP(cfg.URL) {
        return ErrSSRFBlocked
    }
    
    // HMAC signature
    mac := hmac.New(sha256.New, []byte(cfg.Secret))
    mac.Write(payload)
    signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    
    // Retry loop: 3 attempts, backoff: 1s, 2s, 4s
    backoff := 1 * time.Second
    for attempt := 1; attempt <= 3; attempt++ {
        req, _ := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(payload))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("X-OSV-Signature", signature)
        req.Header.Set("X-OSV-Event", evt.EventType)
        
        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(req)
        
        if err == nil && resp.StatusCode < 500 {
            return nil  // Success
        }
        
        if attempt < 3 {
            time.Sleep(backoff)
            backoff *= 2
        }
    }
    
    return ErrWebhookDeliveryFailed
}

func isPrivateIP(rawURL string) bool {
    u, err := url.Parse(rawURL)
    if err != nil {
        return true // reject if unparseable
    }
    addrs, _ := net.LookupHost(u.Hostname())
    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        // Check RFC 1918, loopback, link-local, etc.
        for _, block := range privateIPBlocks {
            if block.Contains(ip) {
                return true
            }
        }
    }
    return false
}
```

---

## 9. Audit-Service Technical Design

### 9.1 HMAC Event Signing

```go
// services/audit-service/internal/domain/audit_event.go

type AuditEvent struct {
    ID           uuid.UUID
    ActorID      uuid.UUID
    ActorEmail   string
    Action       string    // "finding.close", "product.create", ...
    ResourceType string
    ResourceID   uuid.UUID
    BeforeJSON   []byte    // JSONB snapshot
    AfterJSON    []byte    // JSONB snapshot
    HMACSig      string    // HMAC-SHA256 of canonical form
    CreatedAt    time.Time
}

func (e *AuditEvent) ComputeHMAC(secret []byte) string {
    // Canonical form: all fields concatenated deterministically
    canonical := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d",
        e.ID.String(),
        e.ActorID.String(),
        e.Action,
        e.ResourceType,
        e.ResourceID.String(),
        string(e.AfterJSON),
        e.CreatedAt.UnixNano(),
    )
    
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(canonical))
    return hex.EncodeToString(mac.Sum(nil))
}

func (e *AuditEvent) Verify(secret []byte) bool {
    expected := e.ComputeHMAC(secret)
    return hmac.Equal([]byte(e.HMACSig), []byte(expected))
}
```

### 9.2 Partitioned Table Design

```sql
-- Monthly partitions for audit events
CREATE TABLE audit_events (
    id            UUID NOT NULL DEFAULT gen_random_uuid(),
    actor_id      UUID NOT NULL,
    actor_email   VARCHAR(255),
    action        VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id   UUID,
    before_json   JSONB,
    after_json    JSONB,
    hmac_sig      CHAR(64) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- Auto-create monthly partitions
CREATE TABLE audit_events_2026_06 PARTITION OF audit_events
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- Row-Level Security: no UPDATE or DELETE allowed
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_select_only ON audit_events
    FOR SELECT USING (true);
-- Revoke write from application role
REVOKE INSERT, UPDATE, DELETE ON audit_events FROM osv_app;
GRANT INSERT ON audit_events TO osv_audit_writer;  -- Only audit-service
```

### 9.3 NATS Fan-Out Subscriptions (40+ event types)

```go
// services/audit-service/internal/adapter/nats_subscriber.go

func (s *AuditSubscriber) Subscribe(nc *nats.Conn) {
    subjects := []string{
        "finding.created",
        "finding.status.changed",
        "finding.batch_created",
        "finding.sla.breached",
        "risk_acceptance.created",
        "risk_acceptance.expired",
        "jira.issue.created",
        "jira.issue.resolved",
        "kev.new",
        "ingestion.cve.synced",
        // ... 30+ more
    }
    
    for _, subject := range subjects {
        sub := subject
        nc.Subscribe(sub, func(msg *nats.Msg) {
            var evt map[string]interface{}
            json.Unmarshal(msg.Data, &evt)
            
            auditEvent := &AuditEvent{
                Action:     sub,
                AfterJSON:  msg.Data,
                CreatedAt:  time.Now().UTC(),
                // ... extract actor from event payload
            }
            auditEvent.HMACSig = auditEvent.ComputeHMAC(s.hmacSecret)
            
            s.repo.Insert(context.Background(), auditEvent)
        })
    }
}
```

---

## 10. Identity-Service Technical Design

### 10.1 Auth Chain (Local + LDAP)

```go
// services/identity-service/internal/usecase/auth_chain.go

type AuthChain struct {
    local LocalAuthenticator
    ldap  LDAPAuthenticator
    order []string  // Configurable: ["local", "ldap"] or ["ldap", "local"]
}

func (c *AuthChain) Authenticate(ctx context.Context, username, password string) (*User, error) {
    for _, provider := range c.order {
        var user *User
        var err error
        
        switch provider {
        case "local":
            user, err = c.local.Authenticate(ctx, username, password)
        case "ldap":
            user, err = c.ldap.Authenticate(ctx, username, password)
        }
        
        if err == nil {
            return user, nil  // First success wins
        }
        
        if !errors.Is(err, ErrInvalidCredentials) {
            log.Error().Err(err).Str("provider", provider).Msg("auth provider error")
        }
    }
    
    return nil, ErrInvalidCredentials
}
```

### 10.2 API Key Validation (Constant-Time)

```go
// services/identity-service/internal/usecase/apikey.go

func (uc *APIKeyUseCase) Validate(ctx context.Context, rawKey string) (*APIKey, error) {
    if len(rawKey) < 12 {
        return nil, ErrInvalidKey
    }
    
    // Lookup by prefix (first 12 chars) — index scan
    prefix := rawKey[:12]
    key, err := uc.repo.FindByPrefix(ctx, prefix)
    if err != nil {
        return nil, ErrInvalidKey
    }
    
    // Compute hash of provided key
    hash := sha256.Sum256([]byte(rawKey))
    hashHex := hex.EncodeToString(hash[:])
    
    // Constant-time comparison to prevent timing attacks
    if !hmac.Equal([]byte(hashHex), []byte(key.HashSHA256)) {
        return nil, ErrInvalidKey
    }
    
    // Check revoked + expired
    if key.Revoked || (key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt)) {
        return nil, ErrKeyRevoked
    }
    
    return key, nil
}
```

---

## 11. Search-Service Technical Design

### 11.1 Dual Backend Search

```go
// services/search-service/internal/usecase/search.go

type SearchUseCase struct {
    opensearch OpenSearchClient
    postgres   CVERepository
    cache      *redis.Client
}

func (uc *SearchUseCase) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
    // Try OpenSearch first
    result, err := uc.opensearch.Search(ctx, req)
    if err != nil {
        log.Warn().Err(err).Msg("OpenSearch unavailable, falling back to PostgreSQL")
        return uc.searchPostgres(ctx, req)
    }
    return result, nil
}

// OpenSearch query builder
func buildOSQuery(req SearchRequest) map[string]interface{} {
    query := map[string]interface{}{
        "bool": map[string]interface{}{
            "must": []interface{}{},
            "filter": []interface{}{},
        },
    }
    
    // Full-text query
    if req.Query != "" {
        must := query["bool"].(map[string]interface{})["must"].([]interface{})
        query["bool"].(map[string]interface{})["must"] = append(must, map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query":  req.Query,
                "fields": []string{"description^2", "cve_id^3", "vendors", "products"},
            },
        })
    }
    
    // Filters
    filters := query["bool"].(map[string]interface{})["filter"].([]interface{})
    if req.MinEPSS > 0 {
        filters = append(filters, map[string]interface{}{
            "range": map[string]interface{}{
                "epss_score": map[string]interface{}{"gte": req.MinEPSS},
            },
        })
    }
    if req.CWE != "" {
        filters = append(filters, map[string]interface{}{
            "term": map[string]interface{}{"cwe_ids": req.CWE},
        })
    }
    // ... more filters
    
    return map[string]interface{}{"query": query}
}
```

### 11.2 Semantic Search (pgvector)

```go
// services/search-service/internal/usecase/semantic.go

func (uc *SemanticSearchUseCase) Search(ctx context.Context, query string, limit int) ([]CVEResult, error) {
    // Generate embedding for query
    embedding, err := uc.ai.GenerateEmbedding(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("embedding generation: %w", err)
    }
    
    // pgvector cosine similarity search
    results, err := uc.repo.SearchBySimilarity(ctx, embedding, limit, 0.7)
    if err != nil {
        return nil, err
    }
    
    return results, nil
}

// SQL in repository:
// SELECT cve_id, description, severity_v3, cvss_v3_score, epss_score,
//        1 - (embedding <=> $1) AS similarity
// FROM cves
// WHERE 1 - (embedding <=> $1) > $2
// ORDER BY embedding <=> $1
// LIMIT $3
```

---

## 12. Error Handling Design

### 12.1 Error Types

```go
// services/shared/errors/errors.go

var (
    ErrNotFound      = errors.New("not found")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrForbidden     = errors.New("forbidden")
    ErrConflict      = errors.New("conflict")
    ErrInvalidInput  = errors.New("invalid input")
    ErrInternalError = errors.New("internal server error")
)

// HTTP mapping
func HTTPStatus(err error) int {
    switch {
    case errors.Is(err, ErrNotFound):     return 404
    case errors.Is(err, ErrUnauthorized): return 401
    case errors.Is(err, ErrForbidden):    return 403
    case errors.Is(err, ErrConflict):     return 409
    case errors.Is(err, ErrInvalidInput): return 400
    default:                               return 500
    }
}
```

### 12.2 Circuit Breaker (external APIs)

```go
// Used in fetchers for external API calls
cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "nvd-api",
    MaxRequests: 1,           // half-open: 1 request
    Interval:    30 * time.Second,
    Timeout:     60 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
})

result, err := cb.Execute(func() (interface{}, error) {
    return fetchFromNVD(ctx)
})
```

---

## 13. Testing Strategy

### 13.1 Unit Tests (Domain + Use Case)

```go
// Target: ≥ 80% coverage on domain + usecase layers
// No external deps in these tests

func TestFindingStateMachine(t *testing.T) {
    f := &Finding{State: StateActive}
    
    err := f.TransitionTo(StateMitigated, userID)
    assert.NoError(t, err)
    assert.Equal(t, StateMitigated, f.State)
    
    // Invalid transition
    err = f.TransitionTo(StateDuplicate, userID)
    // Duplicate → no reopen
    f.State = StateDuplicate
    err = f.TransitionTo(StateActive, userID)
    assert.Error(t, err)
}
```

### 13.2 Integration Tests (Docker Compose)

```yaml
# Each service: docker-compose.test.yml
services:
  postgres:
    image: pgvector/pgvector:pg16
  redis:
    image: redis:7-alpine
  nats:
    image: nats:2-alpine
    command: -js
  opensearch:
    image: opensearchproject/opensearch:2
    
  test:
    build: .
    command: go test ./... -tags=integration -v
    depends_on: [postgres, redis, nats, opensearch]
    environment:
      POSTGRES_DSN: postgres://test:test@postgres/testdb
      REDIS_URL: redis://redis:6379
      NATS_URL: nats://nats:4222
```

### 13.3 Contract Tests (API)

```go
// Verify HTTP API contracts don't break
func TestFindingAPIContract(t *testing.T) {
    srv := startTestServer()
    
    resp, _ := http.Post(srv.URL+"/api/v2/findings", "application/json",
        strings.NewReader(`{"title":"test","severity":"HIGH","product_id":"...","cve_id":"CVE-2024-1234"}`))
    
    assert.Equal(t, 201, resp.StatusCode)
    
    var body map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&body)
    assert.NotEmpty(t, body["id"])
    assert.Equal(t, "Active", body["state"])
    assert.NotEmpty(t, body["sla_expiration_date"])
}
```

---

## 14. OpenVulnScan Services Technical Design (v3.0)

> **Nguồn**: CR-OVS-001 → CR-OVS-007 + SOL-OVS-001 → SOL-OVS-007

### 14.1 Auth-Service — JWT RS256 + Argon2id + TOTP + OAuth2 (CR-OVS-003)

```go
// auth-service/internal/domain/entity/user.go

type Role string
const (
    RoleAdmin    Role = "admin"
    RoleUser     Role = "user"
    RoleReadOnly Role = "readonly"
    RoleAgent    Role = "agent"
)

// RolePermissions — RBAC matrix
var RolePermissions = map[Role][]string{
    RoleAdmin:    {"scan:create", "scan:read", "asset:write", "asset:read",
                   "user:manage", "report:download", "system:configure",
                   "finding:write", "finding:read"},
    RoleUser:     {"scan:create", "scan:read", "asset:write", "asset:read",
                   "report:download", "finding:write", "finding:read"},
    RoleReadOnly: {"scan:read", "asset:read", "report:download", "finding:read"},
    RoleAgent:    {"asset:write", "agent:report"},
}

// JWT Claims (RS256, 15-min TTL)
type JWTClaims struct {
    jwt.RegisteredClaims
    Role        string   `json:"role"`
    Permissions []string `json:"permissions"`
}
```

**Login flow** (Argon2id verification + MFA + token generation):
```go
// auth-service/internal/usecase/login/usecase.go

func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*LoginOutput, error) {
    user, _ := uc.userRepo.FindByEmail(ctx, in.Email)
    
    // Argon2id verify
    if ok, _ := argon2id.Verify(in.Password, user.HashedPassword); !ok {
        user.FailedLoginAttempts++
        if user.FailedLoginAttempts >= 5 { user.IsActive = false }
        uc.userRepo.Update(ctx, user)
        return nil, ErrInvalidCredentials
    }
    
    // TOTP check if MFA enabled
    if user.MFAEnabled {
        secret, _ := decryptTOTPSecret(user.MFATOTPSecret)
        if !totp.Validate(in.MFACode, secret) { return nil, ErrInvalidMFACode }
    }
    
    // JWT RS256 signed token (JTI stored in Redis for blacklist)
    jti := uuid.New().String()
    accessToken, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, JWTClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID.String(),
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            ID:        jti,
        },
        Role: string(user.Role), Permissions: user.Permissions(),
    }).SignedString(uc.privateKey)
    
    uc.redis.Set(ctx, "auth:jwt:"+jti, user.ID.String(), 15*time.Minute)
    
    // Refresh token with TokenFamily for reuse detection
    refreshToken := uuid.New().String()
    uc.sessionRepo.Save(ctx, &entity.Session{
        RefreshTokenHash: sha256Hex(refreshToken),
        TokenFamily:      uuid.New(),
        ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
    })
    return &LoginOutput{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: 900}, nil
}
```

**gRPC ValidateToken** (hot path — <1ms, Redis only):
```go
// auth-service/internal/usecase/validate_token/usecase.go

func (uc *ValidateTokenUseCase) Execute(ctx context.Context, token string) (*ValidateOutput, error) {
    claims, err := uc.jwtParser.ParseWithClaims(token) // RS256 public key verify — no I/O
    if err != nil { return nil, ErrInvalidToken }
    
    exists, _ := uc.redis.Exists(ctx, "auth:jwt:"+claims.ID).Result()
    if exists == 0 { return nil, ErrTokenRevoked } // JTI not found = expired or revoked
    
    return &ValidateOutput{UserID: claims.Subject, Role: claims.Role, Permissions: claims.Permissions}, nil
}
```

**API Key** (`ovs_` prefix, SHA-256 stored, constant-time compare):
```go
plainKey := "ovs_" + base58.Encode(secureRandom(32))  // "ovs_4xKmNpQvR8..."
prefix := plainKey[:12]   // for lookup by prefix (index scan)
keyHash := sha256Hex(plainKey)   // stored in DB
// Validation: prefix lookup → constant-time SHA-256 compare
```

---

### 14.2 Scan-Service — Nmap/ZAP/Agent + SSE (CR-OVS-001)

**Scan State Machine**:
```go
// scan-service/internal/domain/entity/scan.go

type ScanStatus string
const (
    ScanStatusPending   ScanStatus = "pending"
    ScanStatusQueued    ScanStatus = "queued"
    ScanStatusRunning   ScanStatus = "running"
    ScanStatusCompleted ScanStatus = "completed"
    ScanStatusFailed    ScanStatus = "failed"
    ScanStatusCancelled ScanStatus = "cancelled"
)

var validTransitions = map[ScanStatus][]ScanStatus{
    ScanStatusPending: {ScanStatusQueued, ScanStatusCancelled},
    ScanStatusQueued:  {ScanStatusRunning, ScanStatusCancelled},
    ScanStatusRunning: {ScanStatusCompleted, ScanStatusFailed, ScanStatusCancelled},
}

func (s ScanStatus) CanTransitionTo(next ScanStatus) bool {
    for _, allowed := range validTransitions[s] {
        if allowed == next { return true }
    }
    return false
}
```

**Nmap Scanner** (subprocess + XML parsing):
```go
// scan-service/internal/scanner/nmap/scanner.go

func (s *NmapScanner) FullScan(ctx context.Context, targets []string, opts ScanOptions) ([]*Finding, error) {
    args := []string{
        "-sV", "-O", "--script=vulners",  // CVE detection via vulners NSE
        "-oX", "-",                        // XML to stdout
        "--open", "-T"+strconv.Itoa(opts.Intensity),
    }
    if opts.Ports != "" { args = append(args, "-p", opts.Ports) }
    args = append(args, targets...)
    
    cmd := exec.CommandContext(ctx, s.nmapPath, args...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    if err := cmd.Run(); err != nil && cmd.ProcessState.ExitCode() > 1 {
        return nil, fmt.Errorf("nmap failed: %s", stderr.String())
    }
    
    return s.parseXMLOutput(stdout.Bytes()) // Extracts IP, OS, open ports, CVE IDs from vulners
}

var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)
func extractCVEIDs(vulnersOutput string) []string {
    return dedup(cveRegex.FindAllString(vulnersOutput, -1))
}
```

**ZAP Scanner** (REST API integration):
```go
// scan-service/internal/scanner/zap/scanner.go
// http://zap:8090 — ZAP container running alongside

func (s *ZAPScanner) ActiveScan(ctx context.Context, targetURL string, opts ScanOptions) ([]*WebAlert, error) {
    spiderID, _ := s.startSpider(ctx, targetURL, opts.MaxDepth)     // GET /JSON/spider/action/scan/
    s.waitForProgress(ctx, "spider", spiderID, opts.ZAPConfig.SpiderTimeout)
    
    scanID, _ := s.startActiveScan(ctx, targetURL)                  // GET /JSON/ascan/action/scan/
    s.waitForProgress(ctx, "ascan", scanID, opts.ZAPConfig.ActiveScanTimeout) // default 600s
    
    return s.getAlerts(ctx, targetURL) // GET /JSON/core/view/alerts/?baseurl={url}
}
```

**SSE Progress Stream**:
```go
// GET /api/v1/scans/{id}/stream
func (h *Handler) StreamScanProgress(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher := w.(http.Flusher)
    ticker := time.NewTicker(2 * time.Second)
    for {
        select {
        case <-r.Context().Done(): return
        case <-ticker.C:
            scan, _ := h.scanRepo.FindByID(r.Context(), scanID)
            data, _ := json.Marshal(map[string]interface{}{
                "status": scan.Status, "progress": scan.Progress,
            })
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
            if scan.IsTerminal() {
                fmt.Fprintf(w, "event: done\ndata: {}\n\n")
                flusher.Flush()
                return
            }
        }
    }
}
```

---

### 14.3 Finding-Service OVS — 6-State Machine + SHA-256 Dedup (CR-OVS-002)

**6-State Machine via Boolean Flags** (priority: Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active):
```go
// finding-service/internal/domain/finding/state_machine.go

// CurrentState derives logical state from boolean flags
func (f *Finding) CurrentState() FindingState {
    switch {
    case f.Duplicate:     return StateDuplicate
    case f.FalsePositive: return StateFalsePositive
    case f.OutOfScope:    return StateOutOfScope
    case f.RiskAccepted:  return StateRiskAccepted
    case f.IsMitigated:   return StateMitigated
    default:              return StateActive
    }
}

func (f *Finding) Close(mitigatedByID *uuid.UUID) error {
    if f.CurrentState() != StateActive { return ErrInvalidTransition }
    now := time.Now().UTC()
    f.IsMitigated = true; f.Active = false
    f.MitigatedAt = &now; f.MitigatedByID = mitigatedByID
    return nil
}

func (f *Finding) MarkFalsePositive() error {
    if f.CurrentState() != StateActive { return ErrInvalidTransition }
    f.FalsePositive = true; f.Active = false
    return nil
}
```

**SHA-256 Deduplication**:
```go
// finding-service/internal/domain/finding/entity.go

func (f *Finding) computeHash() {
    h := sha256.New()
    fmt.Fprintf(h, "%s|%s|%s|%s", f.Title, f.ComponentName, f.ComponentVersion, f.CVE)
    f.HashCode = hex.EncodeToString(h.Sum(nil))
}

// On creation: check existing finding with same hash in same product
existing, err := uc.findingRepo.FindByHash(ctx, finding.HashCode, finding.ProductID)
if err == nil && existing != nil {
    finding.Duplicate = true
    finding.Active = false
    finding.DuplicateFindingID = &existing.ID
}
```

---

### 14.4 Product-Service — Hierarchy + CI/CD Orchestrator (CR-OVS-004)

**Product Hierarchy**:
```
ProductType → Product → Engagement → Test → Finding
```

**CI/CD Orchestrator** (single endpoint, auto-creates entire hierarchy):
```go
// product-service/internal/usecase/orchestrator/cicd.go

func (uc *CICDOrchestrator) Execute(ctx context.Context, in CICDTriggerInput) (*CICDTriggerOutput, error) {
    // 1. Find-or-Create ProductType + Product (idempotent by name)
    productType, _ := uc.productTypeRepo.FindByName(ctx, in.ProductType)
    if productType == nil { productType, _ = uc.productTypeRepo.Create(ctx, ...) }
    
    product, _ := uc.productRepo.FindByName(ctx, in.ProductName)
    if product == nil { product, _ = uc.createProduct.Execute(ctx, ...) }
    
    // 2. Create Engagement per CI/CD run (each build = new engagement)
    engagement, _ := uc.createEngagement.Execute(ctx, CreateEngagementInput{
        ProductID:   product.ID,
        Name:        fmt.Sprintf("CI/CD Pipeline - Build #%s", in.BuildID),
        EngagementType: TypeCICD,
        BuildID:     in.BuildID,
        CommitHash:  in.CommitHash,
        BranchTag:   in.BranchTag,
    })
    
    // 3. Create Test + import findings via finding-service gRPC
    test, _ := uc.createTest.Execute(ctx, CreateTestInput{EngagementID: engagement.ID, ...})
    for _, sf := range in.ScanResults {
        result, _ := uc.findingSvcClient.CreateFinding(ctx, &FindingRequest{
            TestID: test.ID, EngagementID: engagement.ID, ProductID: product.ID, ...
        })
        if result.Duplicate { duplicates++ } else { newFindings++ }
    }
    
    // 4. Close engagement
    engagement.Close()
    uc.engagementRepo.Update(ctx, engagement)
    
    return &CICDTriggerOutput{
        NewFindings: newFindings, Duplicates: duplicates,
        ExitCode: exitCode(newFindings), // 0=clean, 1=CVEs found
    }, nil
}
```

---

### 14.5 AI-Service — LLM Provider Chain + Embedding + EPSS + Triage (CR-OVS-005)

**Provider Chain** (failover: Ollama → OpenAI → Azure):
```go
// ai-service/internal/domain/enrichment/provider_chain.go

type ProviderChain struct {
    providers []LLMProvider  // Ordered list: Ollama first, then OpenAI, then Azure
}

func (c *ProviderChain) Generate(ctx context.Context, prompt string) (string, error) {
    var lastErr error
    for _, p := range c.providers {
        result, err := p.Generate(ctx, prompt)
        if err == nil { return result, nil }
        lastErr = err
        log.Warn().Err(err).Str("provider", p.Name()).Msg("LLM provider failed, trying next")
    }
    return "", fmt.Errorf("all providers failed: %w", lastErr)
}
```

**Severity Classification** (CVSS-first, LLM fallback):
```go
func (c *SeverityClassifier) Classify(ctx context.Context, summary, details string, cvss []CVSSSeverity) (*SeverityPrediction, error) {
    // Priority 1: CVSSv3 (deterministic, confidence=1.0)
    for _, s := range cvss {
        if s.Type == "CVSS_V3" {
            return &SeverityPrediction{Severity: fromCVSSv3(s.Score), Confidence: 1.0, Source: "cvss_v3"}, nil
        }
    }
    // Priority 2: CVSSv2 (confidence=0.95)
    // Priority 3: LLM — Ollama→OpenAI→Azure
    prompt := fmt.Sprintf(`Classify severity. JSON only: {"severity":"CRITICAL|HIGH|MEDIUM|LOW", "confidence":0.0-1.0, "reasoning":"..."}\nSummary: %s`, truncate(summary, 500))
    response, err := c.llmChain.Generate(ctx, prompt)
    if err != nil { return defaultPrediction(), nil } // fallback MEDIUM
    // parse JSON response
}
```

**Embedding Cache** (Redis, 7-day TTL):
```go
const embedCacheTTL = 7 * 24 * time.Hour

func (s *EmbeddingService) GenerateForVuln(ctx context.Context, vulnID, summary, details string) ([]float32, error) {
    cacheKey := "osv:embed:" + vulnID
    if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
        return decodeFloat32Slice(cached), nil // little-endian float32
    }
    
    text := truncate(summary+"\n\n"+details, 8000)
    embedding, err := s.provider.GenerateEmbedding(ctx, text)
    if err != nil { return nil, err }
    
    s.redis.Set(ctx, cacheKey, encodeFloat32Slice(embedding), embedCacheTTL)
    return embedding, nil
}
```

**Parallel EnrichCVE** (4 goroutines):
```go
func (uc *EnrichCVEUseCase) Execute(ctx context.Context, in EnrichCVEInput) (*EnrichCVEOutput, error) {
    out := &EnrichCVEOutput{}
    var wg sync.WaitGroup; var mu sync.Mutex
    wg.Add(4)
    
    go func() { defer wg.Done(); emb, _ := uc.embeddingService.GenerateForVuln(ctx, in.CVEID, ...); mu.Lock(); out.Embedding = emb; mu.Unlock() }()
    go func() { defer wg.Done(); pred, _ := uc.severityClassifier.Classify(ctx, ...); mu.Lock(); out.Severity = *pred; mu.Unlock() }()
    go func() { defer wg.Done(); exploit, _ := uc.exploitDetector.Check(ctx, in.CVEID); mu.Lock(); out.ExploitInfo = *exploit; mu.Unlock() }()
    go func() { defer wg.Done(); tags, _ := uc.mitreTagger.Tag(ctx, ...); mu.Lock(); out.MITRETags = tags; mu.Unlock() }()
    
    wg.Wait()
    uc.eventBus.Publish(ctx, "ai.cve.enriched", &CVEEnrichedEvent{CVEID: in.CVEID, Severity: out.Severity.Severity})
    return out, nil
}
```

---

### 14.6 Report-Service — Multi-Format + MinIO (CR-OVS-006)

**Formatter Registry + Parallel Generation**:
```go
// report-service/internal/usecase/generate/usecase.go

func (uc *GenerateReportUseCase) generateAllFormats(ctx context.Context, input *entity.ReportInput) (*entity.ReportOutput, error) {
    output := &entity.ReportOutput{Reports: make(map[entity.OutputFormat][]byte)}
    var mu sync.Mutex; var wg sync.WaitGroup
    
    for _, format := range input.Formats {
        wg.Add(1)
        go func(f entity.OutputFormat) {
            defer wg.Done()
            if formatter := uc.formatterRegistry[f]; formatter != nil {
                if data, err := formatter.Format(ctx, input); err == nil {
                    mu.Lock(); output.Reports[f] = data; mu.Unlock()
                }
            }
        }(format)
    }
    wg.Wait()
    
    if countCVEsAboveThreshold(input.CVEData, input.MinSeverity, input.MinScore) > 0 {
        output.ExitCode = 1 // CI/CD: 0=clean, 1=CVEs found
    }
    return output, nil
}
```

**Formatter Stack**:
- **HTML**: Bootstrap 5.3, light/dark theme, Chart.js severity distribution
- **PDF**: HTML → wkhtmltopdf (Chromium headless)
- **Excel**: `excelize` library, DefectDojo column format, severity color coding
- **CSV**: VEX fields (Justification, Response), EPSS, CVSS vector
- **Console**: ANSI colored output (Critical=Bold Red, High=Red, Medium=Yellow)

**MinIO artifact storage**: `reports/{run_id}/{run_id}.{format}` → presigned download URL

---

### 14.7 Asset-Service — Registry + Risk Scoring + Scheduler (CR-OVS-007)

**Auto-Upsert from Scan**:
```go
// asset-service/internal/usecase/asset/upsert.go
// NATS consumer: scan.scan.completed

func (uc *UpsertAssetUseCase) Execute(ctx context.Context, in UpsertAssetInput) (*entity.Asset, error) {
    existing, err := uc.assetRepo.FindByIP(ctx, in.IPAddress)
    if err != nil || existing == nil {
        return uc.assetRepo.Save(ctx, &entity.Asset{
            IPAddress: in.IPAddress, Hostname: in.Hostname,
            OS: in.OS, Services: in.Services, WebTech: in.WebTech,
            Status: entity.AssetStatusActive,
            LastScanID: &in.ScanID,
        })
    }
    // Update existing: services, OS, web tech
    existing.Hostname = in.Hostname; existing.OS = in.OS
    existing.Services = in.Services; existing.WebTech = in.WebTech
    existing.Status = entity.AssetStatusActive
    existing.LastScanID = &in.ScanID
    return existing, uc.assetRepo.Update(ctx, existing)
}
```

**Risk Score** (derived from active findings):
```go
// risk_score computation: 10.0 if Critical, 7.0 if High, 4.0 if Medium, 1.0 if Low
func computeRiskScore(criticalCount, highCount, mediumCount int) float64 {
    if criticalCount > 0 { return 10.0 }
    if highCount > 0 { return math.Min(10.0, float64(highCount)*2.0 + 5.0) }
    if mediumCount > 0 { return math.Min(5.0, float64(mediumCount)*0.5 + 2.0) }
    return 0.0
}
```

**Scheduled Scan Cron** (1-minute ticker, NATS-triggered):
```go
// scan-service/internal/scheduler/scheduler.go

func (s *Scheduler) Start(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    go func() {
        for {
            select {
            case <-ticker.C:
                now := time.Now().UTC()
                dueSched, _ := s.scheduledScanRepo.FindDue(ctx, now) // next_run_at <= now AND enabled=true
                for _, sched := range dueSched {
                    s.createScanUC.Execute(ctx, CreateScanInput{Targets: sched.Targets, Type: sched.ScanType, ...})
                    nextRun := sched.ComputeNextRun() // cron.ParseStandard(cron_expr).Next(time.Now())
                    sched.LastRunAt = &now; sched.NextRunAt = &nextRun
                    s.scheduledScanRepo.Update(ctx, sched)
                }
            case <-ctx.Done():
                ticker.Stop(); return
            }
        }
    }()
}

// Default cron expressions:
// daily   = "0 2 * * *"   (2:00 AM daily)
// weekly  = "0 2 * * 0"   (2:00 AM Sunday)
// hourly  = "0 * * * *"   (every hour)
// custom  = any valid 5-field cron expression
```

---

## 11. Production-Grade Engineering Standards (v3.0)

> **Tài liệu bắt buộc** — Mọi AI agent và developer phải tuân theo khi implement bất kỳ component nào.

### 11.1 Service Implementation Checklist

Mỗi service phải pass **tất cả** các mục sau:

```
[ ] Domain layer
    [x] Entity structs với validation methods
    [x] Repository interfaces trong domain/interfaces.go
    [x] Domain events (nếu cần publish)
    [x] Value objects (Severity, Status, etc.)
    [x] Zero external imports

[ ] UseCase layer  
    [x] Nhận dependencies qua constructor injection
    [x] Implement đầy đủ — KHÔNG return nil, nil
    [x] Error wrapping: fmt.Errorf("ucName: %w", err)
    [x] Context propagation cho tất cả calls
    [x] Unit tests (mock repo → test business logic)

[ ] Repository layer
    [x] Implement đầy đủ tất cả methods trong interface
    [x] Sử dụng pgx/v5 pool (không raw database/sql)
    [x] Idempotent upsert: ON CONFLICT DO UPDATE
    [x] Pagination với cursor/offset
    [x] Soft delete (deleted_at) cho transactional data

[ ] Handler layer
    [x] Decode → validate → usecase → encode
    [x] Không có business logic trong handler
    [x] Trả đúng HTTP status codes
    [x] Không có hardcode strings/dates/enums
    [x] Header extraction: X-User-ID, X-User-Role từ middleware

[ ] Migration
    [x] SQL file với IF NOT EXISTS
    [x] Seed data cho reference tables
    [x] Rollback migration (down)
    [x] Đã chạy trên staging/server

[ ] Integration test
    [x] tests/client/ test pass ≥ 80%
    [x] Test mọi happy path và error paths chính
```

### 11.2 Go Code Conventions

#### Constructor Pattern (bắt buộc)

```go
// ĐÚNG: Inject qua constructor
type FindingService struct {
    repo    FindingRepository  // interface
    nats    EventPublisher     // interface  
    auditor AuditLogger        // interface
    log     zerolog.Logger
}

func NewFindingService(
    repo FindingRepository,
    nats EventPublisher,
    auditor AuditLogger,
    log zerolog.Logger,
) *FindingService {
    return &FindingService{repo: repo, nats: nats, auditor: auditor, log: log}
}

// SAI: Global state / singleton
var globalRepo = &PostgresFindingRepo{db: globalDB}  // NEVER

// SAI: Tạo dependency trong struct
func NewHandler() *Handler {
    return &Handler{
        db: connectDB(),  // NEVER — inject vào, không tạo trong
    }
}
```

#### Error Handling Pattern

```go
// ĐÚNG: Wrapped errors với context
func (uc *FindingUseCase) Create(ctx context.Context, in CreateInput) (*Finding, error) {
    if err := in.Validate(); err != nil {
        return nil, fmt.Errorf("finding.Create: validate: %w", err)
    }
    
    existing, err := uc.repo.FindByHash(ctx, in.Hash, in.ProductID)
    if err != nil && !errors.Is(err, ErrNotFound) {
        return nil, fmt.Errorf("finding.Create: check duplicate: %w", err)
    }
    
    f := &Finding{ID: uuid.New(), Title: in.Title, ...}
    if err := uc.repo.Create(ctx, f); err != nil {
        return nil, fmt.Errorf("finding.Create: persist: %w", err)
    }
    
    _ = uc.nats.Publish(ctx, "finding.created", f.ID)
    return f, nil
}

// SAI:
func (uc *UseCase) Execute(ctx context.Context) error {
    return nil  // NEVER: không làm gì
}
```

#### Repository Pattern (pgx/v5)

```go
// ĐÚNG: Full implementation
func (r *PostgresFindingRepo) Create(ctx context.Context, f *Finding) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO findings (id, title, severity, state, product_id, created_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
        ON CONFLICT (id) DO NOTHING
    `, f.ID, f.Title, f.Severity, f.State, f.ProductID)
    if err != nil {
        return fmt.Errorf("finding_repo.Create: %w", err)
    }
    return nil
}

func (r *PostgresFindingRepo) List(ctx context.Context, filter FindingFilter) ([]*Finding, int, error) {
    // Build WHERE clause dynamically
    where, args := buildFindingWhere(filter)
    
    // Count query
    var total int
    err := r.pool.QueryRow(ctx,
        "SELECT COUNT(*) FROM findings WHERE "+where, args...,
    ).Scan(&total)
    if err != nil {
        return nil, 0, fmt.Errorf("finding_repo.List count: %w", err)
    }
    
    // Data query with pagination
    rows, err := r.pool.Query(ctx,
        "SELECT id, title, severity, state, product_id, created_at FROM findings WHERE "+
        where+" ORDER BY created_at DESC LIMIT $N OFFSET $M",
        append(args, filter.Limit, filter.Offset)...,
    )
    // ... scan rows ...
    return findings, total, nil
}
```

### 11.3 Handler Layer Standards

```go
// ĐÚNG: Handler pattern
func (h *FindingHandler) Create(w http.ResponseWriter, r *http.Request) {
    // 1. Extract user context từ middleware headers
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        writeError(w, http.StatusUnauthorized, "missing user context")
        return
    }
    
    // 2. Decode và validate request
    var req CreateFindingRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if err := req.Validate(); err != nil {
        writeError(w, http.StatusUnprocessableEntity, err.Error())
        return
    }
    
    // 3. Call usecase
    result, err := h.uc.Create(r.Context(), req.ToInput(userID))
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            writeError(w, http.StatusNotFound, err.Error())
            return
        }
        h.log.Error().Err(err).Msg("FindingHandler.Create")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    
    // 4. Encode response
    writeJSON(w, http.StatusCreated, toFindingResponse(result))
}

// KHÔNG BAO GIỜ trong handler:
// - Gọi r.db.Query() trực tiếp
// - return map[string]interface{}{"date": "2026-01-01"}
// - Hardcode enum values
// - Business logic (if finding.severity == "critical" { ... })
```

### 11.4 Database Migration Standards

```sql
-- File naming: NNN_service_description.sql (e.g., 001_initial_schema.sql)
-- Mỗi migration PHẢI:
-- 1. Dùng IF NOT EXISTS (idempotent)
-- 2. Có schema prefix cho services riêng biệt
-- 3. Có index cho foreign keys và query patterns
-- 4. Seed reference data trong cùng file

-- Schema isolation
CREATE SCHEMA IF NOT EXISTS finding;

-- Entity table
CREATE TABLE IF NOT EXISTS finding.findings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       VARCHAR(500) NOT NULL,
    severity    VARCHAR(20)  NOT NULL CHECK (severity IN ('critical','high','medium','low','informational')),
    state       VARCHAR(20)  NOT NULL DEFAULT 'active'
                             CHECK (state IN ('active','resolved','risk_accepted','duplicate')),
    product_id  UUID         NOT NULL,
    created_by  UUID,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_findings_product ON finding.findings(product_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_findings_severity ON finding.findings(severity) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_findings_state ON finding.findings(state);
CREATE INDEX IF NOT EXISTS idx_findings_created ON finding.findings(created_at DESC);

-- Trigger: auto-update updated_at
CREATE OR REPLACE FUNCTION finding.update_timestamp()
RETURNS TRIGGER AS $$ BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON finding.findings
    FOR EACH ROW EXECUTE FUNCTION finding.update_timestamp();
```

### 11.5 Configuration Standards

```go
// Mọi service phải có Config struct với env tags
type Config struct {
    // Required — service sẽ fail fast nếu thiếu
    DatabaseURL  string `env:"DATABASE_URL,required"`
    JWTSecret    string `env:"JWT_SECRET,required"`
    
    // Optional với defaults hợp lý
    HTTPPort     int           `env:"HTTP_PORT"       envDefault:"8085"`
    RedisAddr    string        `env:"REDIS_ADDR"      envDefault:"localhost:6379"`
    NATSAddr     string        `env:"NATS_URL"        envDefault:"nats://localhost:4222"`
    LogLevel     string        `env:"LOG_LEVEL"       envDefault:"info"`
    ReadTimeout  time.Duration `env:"READ_TIMEOUT"    envDefault:"30s"`
    WriteTimeout time.Duration `env:"WRITE_TIMEOUT"   envDefault:"30s"`
    MaxConns     int           `env:"DB_MAX_CONNS"    envDefault:"20"`
    
    // Secrets — KHÔNG log, KHÔNG marshal vào JSON
    SMTPPassword string `env:"SMTP_PASSWORD" json:"-"`
}

// Validate trong main.go:
func main() {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        log.Fatal().Err(err).Msg("invalid configuration")
    }
    // ... start service
}
```

### 11.6 Observability Standards

#### Structured Logging (zerolog)

```go
// Request log
log.Info().
    Str("method", r.Method).
    Str("path", r.URL.Path).
    Str("user_id", userID).
    Int("status", statusCode).
    Dur("duration", elapsed).
    Msg("request")

// Error log
log.Error().
    Err(err).
    Str("handler", "FindingHandler.Create").
    Str("user_id", userID).
    Msg("handler error")

// KHÔNG dùng:
fmt.Println("error:", err)  // không structured
log.Printf("...")           // không zerolog
panic(err)                  // không panic trong handler
```

#### Prometheus Metrics

```go
// Mỗi service expose /metrics với:
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "http_requests_total"},
        []string{"method", "path", "status"},
    )
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{Name: "http_request_duration_seconds"},
        []string{"method", "path"},
    )
    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{Name: "db_query_duration_seconds"},
        []string{"query"},
    )
)
```

### 11.7 Testing Standards

#### Unit Tests (usecase layer)

```go
// Dùng mock repositories để test business logic
func TestFindingUseCase_Create_Duplicate(t *testing.T) {
    mockRepo := &MockFindingRepo{}
    mockRepo.On("FindByHash", mock.Anything, "abc123", uuid.UUID{}).Return(&Finding{ID: uuid.New()}, nil)
    
    uc := NewFindingUseCase(mockRepo, &MockPublisher{}, &MockAudit{})
    _, err := uc.Create(context.Background(), CreateInput{Hash: "abc123"})
    
    assert.ErrorIs(t, err, ErrDuplicate)
}
```

#### Integration Tests (tests/client/)

- Chạy với server thật trên `c12.openledger.vn`
- Mỗi test module: ≥ 80% pass rate
- Kiểm tra cả schema (field presence, type) lẫn value (enum validation)
- Không dùng `time.Sleep` để chờ — dùng retry với timeout

### 11.8 Forbidden Patterns (Anti-Patterns)

```go
// ❌ FORBIDDEN: UseCase rỗng
func (uc *UseCase) Execute(ctx context.Context) error {
    return nil  // không làm gì
}

// ❌ FORBIDDEN: Hardcode trong handler
writeJSON(w, 200, map[string]interface{}{
    "created_at": "2026-06-22T00:00:00Z",  // hardcode
    "policy":     "medium",                  // hardcode
})

// ❌ FORBIDDEN: SQL trong handler
rows, _ := db.Query("SELECT * FROM findings")  // bypass repository

// ❌ FORBIDDEN: Nil handler
router.Handle("/api/v1/import", nil)  // panic when called

// ❌ FORBIDDEN: Global mutable state
var activeConnections int  // data race

// ❌ FORBIDDEN: Credentials trong code
const jwtSecret = "my-super-secret"  // security breach

// ❌ FORBIDDEN: Error suppression
_, _ = repo.Create(ctx, entity)  // hide errors

// ❌ FORBIDDEN: Panic trong handler
panic("not implemented")  // crashes entire service

// ✅ CORRECT alternatives:
// - Return ErrNotImplemented (domain error)
// - Trả 501 với message rõ ràng
// - Log warning và tiếp tục với fallback
```
