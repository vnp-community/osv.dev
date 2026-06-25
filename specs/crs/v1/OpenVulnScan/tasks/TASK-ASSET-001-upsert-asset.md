# TASK-ASSET-001 — Asset Entity + UpsertAsset Use Case

| Field | Value |
|-------|-------|
| **Task ID** | T-ASSET-001 |
| **Service** | `asset-service` (new service, via scan-service extension) |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-007 §2 Asset Service |
| **Priority** | 🟡 Medium |
| **Depends On** | — |
| **Estimated** | 3h |

---

## Context

Asset Management được implement trong `scan-service` vì domain liên quan chặt chẽ (scan → upsert assets). `scan-service/internal/domain/asset/` đã tồn tại. Task này bổ sung:

1. `Asset` domain entity với risk scoring
2. `UpsertAsset` use case — find-or-create by IP address
3. NATS consumer trigger từ `scan.scan.completed`

---

## Goal

Implement asset registry core:
- `Asset` entity với IP, hostname, OS, services, risk score, tags
- `UpsertAsset` use case (idempotent: same IP = update existing)
- Risk score computation: 0.0–10.0 based on finding severity

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/scan-service/internal/domain/asset/entity.go` |
| CREATE | `services/scan-service/internal/domain/asset/errors.go` |
| CREATE | `services/scan-service/internal/domain/asset/risk.go` |
| CREATE | `services/scan-service/internal/usecase/asset/upsert.go` |
| CREATE | `services/scan-service/internal/usecase/asset/upsert_test.go` |

---

## Implementation

### File 1: `services/scan-service/internal/domain/asset/entity.go`

```go
package asset

import (
    "encoding/json"
    "net"
    "strings"
    "time"

    "github.com/google/uuid"
)

// AssetStatus tracks the operational state of an asset
type AssetStatus string

const (
    StatusActive   AssetStatus = "active"
    StatusInactive AssetStatus = "inactive"
    StatusUnknown  AssetStatus = "unknown"
)

// NetworkService represents a detected network service on an asset
type NetworkService struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // "tcp" | "udp"
    Name     string `json:"name"`     // e.g., "http", "ssh", "mysql"
    Product  string `json:"product"`  // e.g., "Apache httpd"
    Version  string `json:"version"`  // e.g., "2.4.51"
    Banner   string `json:"banner,omitempty"`
}

// WebTechnology represents a web technology detected on an asset
type WebTechnology struct {
    Name       string   `json:"name"`
    Version    string   `json:"version,omitempty"`
    Categories []string `json:"categories"` // e.g., ["CMS", "PHP"]
}

// Asset represents a discovered IT asset in the network
type Asset struct {
    ID            uuid.UUID
    IPAddress     net.IP
    Hostname      string
    OS            string
    OSVersion     string
    MACAddress    string
    Status        AssetStatus
    Tags          []string
    Services      []NetworkService
    WebTech       []WebTechnology
    LastScanID    *uuid.UUID
    LastScannedAt *time.Time
    FindingCount  int     // Number of active findings
    RiskScore     float64 // 0.0 (safe) - 10.0 (critical)
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// NewAsset creates a new Asset from scan discovery data
func NewAsset(ipStr, hostname, os, osVersion string) (*Asset, error) {
    ip := net.ParseIP(strings.TrimSpace(ipStr))
    if ip == nil {
        return nil, ErrInvalidIPAddress
    }

    now := time.Now().UTC()
    return &Asset{
        ID:        uuid.New(),
        IPAddress: ip,
        Hostname:  strings.TrimSpace(hostname),
        OS:        os,
        OSVersion: osVersion,
        Status:    StatusActive,
        Tags:      []string{},
        Services:  []NetworkService{},
        WebTech:   []WebTechnology{},
        CreatedAt: now,
        UpdatedAt: now,
    }, nil
}

// IPString returns the IP address as a string
func (a *Asset) IPString() string {
    if a.IPAddress == nil {
        return ""
    }
    return a.IPAddress.String()
}

// AddTag adds a tag if not already present (case-insensitive dedup)
func (a *Asset) AddTag(tag string) {
    tag = strings.TrimSpace(tag)
    if tag == "" {
        return
    }
    for _, existing := range a.Tags {
        if strings.EqualFold(existing, tag) {
            return
        }
    }
    a.Tags = append(a.Tags, tag)
    a.UpdatedAt = time.Now().UTC()
}

// RemoveTag removes a tag (case-insensitive)
func (a *Asset) RemoveTag(tag string) {
    tag = strings.ToLower(strings.TrimSpace(tag))
    filtered := a.Tags[:0]
    for _, t := range a.Tags {
        if !strings.EqualFold(t, tag) {
            filtered = append(filtered, t)
        }
    }
    a.Tags = filtered
    a.UpdatedAt = time.Now().UTC()
}

// SetTags replaces all tags with the provided list
func (a *Asset) SetTags(tags []string) {
    a.Tags = make([]string, 0, len(tags))
    for _, t := range tags {
        t = strings.TrimSpace(t)
        if t != "" {
            a.Tags = append(a.Tags, t)
        }
    }
    a.UpdatedAt = time.Now().UTC()
}

// UpdateFromScan updates asset data from a new scan result
func (a *Asset) UpdateFromScan(scanID uuid.UUID, hostname, os, osVersion string, services []NetworkService) {
    now := time.Now().UTC()
    if hostname != "" {
        a.Hostname = hostname
    }
    if os != "" {
        a.OS = os
    }
    if osVersion != "" {
        a.OSVersion = osVersion
    }
    if len(services) > 0 {
        a.Services = services
    }
    a.LastScanID = &scanID
    a.LastScannedAt = &now
    a.Status = StatusActive
    a.UpdatedAt = now
}

// UpdateRiskScore recomputes the risk score based on finding stats
func (a *Asset) UpdateRiskScore(stats FindingStats) {
    a.FindingCount = stats.Active
    a.RiskScore = computeRiskScore(stats)
    a.UpdatedAt = time.Now().UTC()
}

// ServicesJSON serializes services for DB storage
func (a *Asset) ServicesJSON() ([]byte, error) {
    return json.Marshal(a.Services)
}

// WebTechJSON serializes web technologies for DB storage
func (a *Asset) WebTechJSON() ([]byte, error) {
    return json.Marshal(a.WebTech)
}
```

### File 2: `services/scan-service/internal/domain/asset/errors.go`

```go
package asset

import "errors"

var (
    ErrInvalidIPAddress = errors.New("invalid IP address format")
    ErrAssetNotFound    = errors.New("asset not found")
)
```

### File 3: `services/scan-service/internal/domain/asset/risk.go`

```go
package asset

// FindingStats contains aggregated counts of active findings by severity
type FindingStats struct {
    Active   int // Total active findings
    Critical int
    High     int
    Medium   int
    Low      int
}

// computeRiskScore calculates a risk score 0.0–10.0 based on finding severity.
//
// Risk scoring:
//   - Any Critical finding → 10.0 (maximum risk)
//   - High findings: 8.0 base + up to 2.0 for volume
//   - Medium findings: 5.0 base + up to 3.0 for volume
//   - Low findings: up to 5.0 based on count
//   - No findings → 0.0
func computeRiskScore(stats FindingStats) float64 {
    if stats.Critical > 0 {
        return 10.0
    }

    if stats.High > 0 {
        extra := float64(min(stats.High, 5)) * 0.4
        return min(10.0, 8.0+extra)
    }

    if stats.Medium > 0 {
        extra := float64(min(stats.Medium, 5)) * 0.6
        return min(10.0, 5.0+extra)
    }

    if stats.Low > 0 {
        return min(5.0, float64(stats.Low)*1.0)
    }

    return 0.0
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

// minInt returns the minimum of two int values
func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

### File 4: `services/scan-service/internal/usecase/asset/upsert.go`

```go
package asset

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    assetdomain "github.com/google/osv.dev/services/scan-service/internal/domain/asset"
)

// AssetRepository defines the storage interface for assets
type AssetRepository interface {
    FindByIP(ctx context.Context, ipAddress string) (*assetdomain.Asset, error)
    Save(ctx context.Context, asset *assetdomain.Asset) error
    Update(ctx context.Context, asset *assetdomain.Asset) error
}

// UpsertInput contains the data from a scan to upsert into the asset registry
type UpsertInput struct {
    ScanID    uuid.UUID
    IPAddress string
    Hostname  string
    OS        string
    OSVersion string
    Services  []assetdomain.NetworkService
    WebTech   []assetdomain.WebTechnology
}

// UpsertResult describes what happened during the upsert
type UpsertResult struct {
    Asset   *assetdomain.Asset
    Created bool   // true if new asset was created; false if existing was updated
    Reason  string // "created" | "updated"
}

// UpsertUseCase handles creating or updating assets from scan results
type UpsertUseCase struct {
    repo   AssetRepository
    logger zerolog.Logger
}

// New creates the UpsertUseCase
func New(repo AssetRepository, logger zerolog.Logger) *UpsertUseCase {
    return &UpsertUseCase{repo: repo, logger: logger}
}

// Execute upserts an asset: creates new if IP is unknown, updates if IP already tracked.
// This is IDEMPOTENT: multiple calls with the same IP update the existing asset.
func (uc *UpsertUseCase) Execute(ctx context.Context, in UpsertInput) (*UpsertResult, error) {
    if in.IPAddress == "" {
        return nil, fmt.Errorf("IP address is required")
    }

    // Try to find existing asset by IP
    existing, err := uc.repo.FindByIP(ctx, in.IPAddress)
    if err != nil {
        return nil, fmt.Errorf("find asset by IP: %w", err)
    }

    if existing != nil {
        // UPDATE existing asset
        existing.UpdateFromScan(in.ScanID, in.Hostname, in.OS, in.OSVersion, in.Services)
        if len(in.WebTech) > 0 {
            existing.WebTech = in.WebTech
        }

        if err := uc.repo.Update(ctx, existing); err != nil {
            return nil, fmt.Errorf("update asset: %w", err)
        }

        uc.logger.Debug().
            Str("ip", in.IPAddress).
            Str("asset_id", existing.ID.String()).
            Str("scan_id", in.ScanID.String()).
            Msg("asset updated from scan")

        return &UpsertResult{Asset: existing, Created: false, Reason: "updated"}, nil
    }

    // CREATE new asset
    a, err := assetdomain.NewAsset(in.IPAddress, in.Hostname, in.OS, in.OSVersion)
    if err != nil {
        return nil, fmt.Errorf("create asset: %w", err)
    }

    a.Services = in.Services
    a.WebTech = in.WebTech
    scanID := in.ScanID
    now := time.Now().UTC()
    a.LastScanID = &scanID
    a.LastScannedAt = &now

    if err := uc.repo.Save(ctx, a); err != nil {
        return nil, fmt.Errorf("save new asset: %w", err)
    }

    uc.logger.Info().
        Str("ip", in.IPAddress).
        Str("asset_id", a.ID.String()).
        Str("scan_id", in.ScanID.String()).
        Msg("new asset registered")

    return &UpsertResult{Asset: a, Created: true, Reason: "created"}, nil
}

// ExecuteBatch upserts multiple assets from a scan (e.g., /24 network scan)
func (uc *UpsertUseCase) ExecuteBatch(ctx context.Context, inputs []UpsertInput) ([]*UpsertResult, error) {
    results := make([]*UpsertResult, 0, len(inputs))

    for _, in := range inputs {
        result, err := uc.Execute(ctx, in)
        if err != nil {
            uc.logger.Error().Err(err).Str("ip", in.IPAddress).Msg("failed to upsert asset")
            continue // Don't fail entire batch for single asset
        }
        results = append(results, result)
    }

    uc.logger.Info().
        Int("total", len(inputs)).
        Int("success", len(results)).
        Msg("batch asset upsert completed")

    return results, nil
}
```

### File 5: `services/scan-service/internal/usecase/asset/upsert_test.go`

```go
package asset

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    assetdomain "github.com/google/osv.dev/services/scan-service/internal/domain/asset"
)

// mockAssetRepo is an in-memory asset store for testing
type mockAssetRepo struct {
    assets  map[string]*assetdomain.Asset // keyed by IP string
    saves   int
    updates int
}

func newMockRepo() *mockAssetRepo {
    return &mockAssetRepo{assets: make(map[string]*assetdomain.Asset)}
}

func (m *mockAssetRepo) FindByIP(_ context.Context, ip string) (*assetdomain.Asset, error) {
    if a, ok := m.assets[ip]; ok {
        return a, nil
    }
    return nil, nil
}

func (m *mockAssetRepo) Save(_ context.Context, a *assetdomain.Asset) error {
    m.assets[a.IPString()] = a
    m.saves++
    return nil
}

func (m *mockAssetRepo) Update(_ context.Context, a *assetdomain.Asset) error {
    m.assets[a.IPString()] = a
    m.updates++
    return nil
}

func TestUpsert_NewAsset(t *testing.T) {
    repo := newMockRepo()
    uc := New(repo, zerolog.Nop())

    result, err := uc.Execute(context.Background(), UpsertInput{
        ScanID:    uuid.New(),
        IPAddress: "192.168.1.10",
        Hostname:  "web-server.local",
        OS:        "Linux",
        OSVersion: "5.15",
        Services: []assetdomain.NetworkService{
            {Port: 80, Protocol: "tcp", Name: "http"},
        },
    })
    if err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if !result.Created {
        t.Error("should be Created=true for new IP")
    }
    if result.Reason != "created" {
        t.Errorf("Reason = %s, want created", result.Reason)
    }
    if repo.saves != 1 || repo.updates != 0 {
        t.Errorf("saves=%d updates=%d, want saves=1 updates=0", repo.saves, repo.updates)
    }
}

func TestUpsert_ExistingAsset(t *testing.T) {
    repo := newMockRepo()
    uc := New(repo, zerolog.Nop())
    scanID := uuid.New()

    // First upsert — creates
    _, _ = uc.Execute(context.Background(), UpsertInput{
        ScanID: scanID, IPAddress: "10.0.0.1", Hostname: "old-hostname",
    })

    // Second upsert — updates
    result, err := uc.Execute(context.Background(), UpsertInput{
        ScanID: uuid.New(), IPAddress: "10.0.0.1", Hostname: "new-hostname",
    })
    if err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if result.Created {
        t.Error("should be Created=false for existing IP")
    }
    if result.Asset.Hostname != "new-hostname" {
        t.Errorf("Hostname = %s, want new-hostname", result.Asset.Hostname)
    }
    if repo.saves != 1 || repo.updates != 1 {
        t.Errorf("saves=%d updates=%d, want saves=1 updates=1", repo.saves, repo.updates)
    }
}

func TestUpsert_EmptyIP(t *testing.T) {
    uc := New(newMockRepo(), zerolog.Nop())
    _, err := uc.Execute(context.Background(), UpsertInput{ScanID: uuid.New()})
    if err == nil {
        t.Error("expected error for empty IP address")
    }
}

func TestUpsert_BatchProcessing(t *testing.T) {
    repo := newMockRepo()
    uc := New(repo, zerolog.Nop())
    scanID := uuid.New()

    inputs := []UpsertInput{
        {ScanID: scanID, IPAddress: "10.0.0.1"},
        {ScanID: scanID, IPAddress: "10.0.0.2"},
        {ScanID: scanID, IPAddress: "10.0.0.3"},
    }

    results, err := uc.ExecuteBatch(context.Background(), inputs)
    if err != nil {
        t.Fatalf("ExecuteBatch: %v", err)
    }
    if len(results) != 3 {
        t.Errorf("results = %d, want 3", len(results))
    }
    if repo.saves != 3 {
        t.Errorf("saves = %d, want 3", repo.saves)
    }
}

func TestRiskScore_Critical(t *testing.T) {
    stats := assetdomain.FindingStats{Active: 1, Critical: 1}
    a, _ := assetdomain.NewAsset("192.168.1.1", "", "", "")
    a.UpdateRiskScore(stats)
    if a.RiskScore != 10.0 {
        t.Errorf("Critical should give risk_score=10.0, got %.1f", a.RiskScore)
    }
}

func TestRiskScore_NoFindings(t *testing.T) {
    a, _ := assetdomain.NewAsset("192.168.1.1", "", "", "")
    a.UpdateRiskScore(assetdomain.FindingStats{})
    if a.RiskScore != 0.0 {
        t.Errorf("No findings should give risk_score=0.0, got %.1f", a.RiskScore)
    }
}

func TestAsset_Tags(t *testing.T) {
    a, _ := assetdomain.NewAsset("192.168.1.1", "", "", "")
    a.AddTag("production")
    a.AddTag("web")
    a.AddTag("PRODUCTION") // Duplicate (case-insensitive)

    if len(a.Tags) != 2 {
        t.Errorf("Tags len = %d, want 2 (dedup)", len(a.Tags))
    }

    a.RemoveTag("production")
    if len(a.Tags) != 1 {
        t.Errorf("After remove, Tags len = %d, want 1", len(a.Tags))
    }
}
```

---

## Verification

```bash
cd services/scan-service
go build ./internal/domain/asset/... ./internal/usecase/asset/...
go test ./internal/domain/asset/... ./internal/usecase/asset/... -v
```

**Expected**:
```
--- PASS: TestUpsert_NewAsset
--- PASS: TestUpsert_ExistingAsset
--- PASS: TestUpsert_EmptyIP
--- PASS: TestUpsert_BatchProcessing
--- PASS: TestRiskScore_Critical
--- PASS: TestRiskScore_NoFindings
--- PASS: TestAsset_Tags
```

### Checklist

- [x] `NewAsset("invalid")` → `ErrInvalidIPAddress`
- [x] `NewAsset("192.168.1.1", ...)` → valid Asset with `Status=active`
- [x] `Execute(new IP)` → `Created=true`, `repo.Save()` called
- [x] `Execute(existing IP)` → `Created=false`, `repo.Update()` called, hostname updated
- [x] `Execute("")` → error
- [x] `ExecuteBatch([3 IPs])` → 3 saves, all returned
- [x] `AddTag("production")` twice (case-insensitive) → only 1 tag stored
- [x] `RemoveTag` → case-insensitive removal
- [x] `RiskScore = 10.0` when Critical > 0
- [x] `RiskScore = 0.0` when no findings

## Notes for AI

Check existing `services/scan-service/internal/domain/asset/` and `services/scan-service/internal/usecase/asset/` directories — some code may already exist. If so, review and extend rather than overwrite.
