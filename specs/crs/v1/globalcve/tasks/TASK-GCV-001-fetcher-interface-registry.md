# TASK-GCV-001 — Fetcher Interface Extension + Registry

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-001 |
| **Service** | `data-service` |
| **CR** | CR-GCV-001 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | — |

## Context

`data-service` hiện có interface `Fetcher` với signature `FetchAndStore(ctx, FetchOptions) (int, error)`.  
Task này mở rộng interface, thêm `IncrementalFetcher`, định nghĩa `SourceName` constants, và tạo `Registry` pattern để các fetcher mới tự đăng ký.

## Reference

- Solution: [SOL-GCV-001](../solutions/SOL-GCV-001-multi-source-fetcher.md)
- CR: [CR-GCV-001](../CR-GCV-001-multi-source-fetcher-pipeline.md)

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/fetcher/fetcher.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/fetcher/registry.go
```

## Implementation Spec

### fetcher.go — ADD SourceName + IncrementalFetcher

```go
// Package fetcher defines the Fetcher interface for data source ingestion.
package fetcher

import (
    "context"
    "time"
)

// SourceName — canonical identifier cho mỗi data source
type SourceName string

const (
    SourceNVD       SourceName = "NVD"
    SourceCIRCL     SourceName = "CIRCL"
    SourceJVN       SourceName = "JVN"
    SourceExploitDB SourceName = "EXPLOITDB"
    SourceCVEOrg    SourceName = "CVE.ORG"
    SourceCNNVD     SourceName = "CNNVD"
    SourceEPSS      SourceName = "EPSS"
    SourceNVDCPE    SourceName = "NVD-CPE"
    SourceCAPEC     SourceName = "CAPEC"
    SourceCWE       SourceName = "CWE"
    // Beta sources (Phase 2)
    SourceAndroid   SourceName = "ANDROID"
    SourceCERTFR    SourceName = "CERT-FR"
)

// Fetcher — giữ nguyên interface hiện tại (backward compatibility)
type Fetcher interface {
    // Name returns the source identifier
    Name() string
    // FetchAndStore fetches data and upserts into DB.
    // Returns count of documents processed.
    FetchAndStore(ctx context.Context, opts FetchOptions) (int, error)
}

// IncrementalFetcher — fetchers hỗ trợ incremental sync
// Implement thêm interface này nếu source hỗ trợ "last modified since"
type IncrementalFetcher interface {
    Fetcher
    // FetchSince chỉ fetch dữ liệu thay đổi kể từ `since`
    FetchSince(ctx context.Context, since time.Time) (int, error)
}

// FetchOptions controls fetch behavior (giữ nguyên)
type FetchOptions struct {
    // ManualDays: 0 = full import, >0 = only fetch last N days
    ManualDays int
    // StartYear: for CVE fetcher, first year to import
    StartYear int
}

// FetchResult — thống kê sau khi fetch (mới)
type FetchResult struct {
    Source    SourceName
    Fetched   int
    Upserted  int
    Skipped   int
    Errors    int
    Duration  time.Duration
    StartedAt time.Time
}
```

### registry.go — NEW

```go
// Package fetcher — Registry for auto-registration of all fetchers.
package fetcher

import "sync"

// Registry holds all registered fetchers, keyed by SourceName.
// Thread-safe for concurrent reads.
type Registry struct {
    fetchers map[SourceName]Fetcher
    mu       sync.RWMutex
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
    return &Registry{fetchers: make(map[SourceName]Fetcher)}
}

// Register adds a fetcher to the registry.
// Panics if a fetcher with the same SourceName is already registered (programming error).
func (r *Registry) Register(source SourceName, f Fetcher) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.fetchers[source]; exists {
        panic("fetcher already registered: " + string(source))
    }
    r.fetchers[source] = f
}

// Get returns the fetcher for a source, or (nil, false) if not found.
func (r *Registry) Get(source SourceName) (Fetcher, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    f, ok := r.fetchers[source]
    return f, ok
}

// All returns a snapshot of all registered fetchers.
func (r *Registry) All() []Fetcher {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]Fetcher, 0, len(r.fetchers))
    for _, f := range r.fetchers {
        result = append(result, f)
    }
    return result
}

// Sources returns all registered source names.
func (r *Registry) Sources() []SourceName {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]SourceName, 0, len(r.fetchers))
    for name := range r.fetchers {
        names = append(names, name)
    }
    return names
}
```

## Acceptance Criteria

- [x] `fetcher.SourceName` type và tất cả constants (NVD, CIRCL, JVN, EXPLOITDB, CVE.ORG, CNNVD, EPSS, NVD-CPE, CAPEC, CWE) được định nghĩa
- [x] `IncrementalFetcher` interface có method `FetchSince(ctx, since) (int, error)`
- [x] `FetchResult` struct tồn tại với đầy đủ fields
- [x] `Registry.Register` panic khi duplicate source
- [x] `Registry.All()` thread-safe (multiple concurrent goroutines không data race)
- [x] Existing code vẫn compile (backward compatible — `Fetcher` interface không thay đổi signature)
- [x] `go build ./...` trong `data-service/` pass không có lỗi
