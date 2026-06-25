# TASK-GCV-015 — KEV Sync Diff Detection + NATS Publish

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-015 |
| **Service** | `data-service` |
| **CR** | CR-GCV-007 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-014 |

## Context

Nâng cấp KEV sync use case để: (1) map `shortDescription`, `requiredAction`, `knownRansomwareCampaignUse` từ CISA API, (2) detect new KEV entries (diff với existing), (3) publish `kev.new` event tới NATS JetStream cho mỗi entry mới (nếu NATS configured). NATS failure là non-fatal.

## Reference

- Solution: [SOL-GCV-007](../solutions/SOL-GCV-007-kev-enhancement.md) §2.2, §2.3
- CR: [CR-GCV-007](../CR-GCV-007-kev-service-enhancement.md)

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/usecase/sync/ (kev sync)
        (đọc cấu trúc, tìm sync use case file)
CREATE hoặc MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/fetcher/publisher_hook.go
        (NATS publisher — kiểm tra nếu đã tồn tại)
```

**Đọc trước**: KEV sync usecase hiện có để biết flow và data structures.

## Implementation Spec

### KEV Sync UseCase — MODIFY

Trong sync usecase (`sync.go` hoặc tương đương):

```go
// SyncResult — extended để track new entries
type SyncResult struct {
    Inserted   int
    Updated    int
    Total      int
    NewEntries []*kev.KEVEntry // entries newly added to CISA KEV
}

// Trong Execute/Sync method:
func (uc *UseCase) Sync(ctx context.Context) (*SyncResult, error) {
    // 1. Fetch CISA KEV catalog
    catalog, err := uc.fetchCatalog(ctx)
    if err != nil {
        return nil, fmt.Errorf("kev: fetch catalog: %w", err)
    }

    // 2. Get existing CVE IDs from DB (for diff detection)
    existingIDs, err := uc.kevRepo.GetAllCVEIDs(ctx)
    if err != nil {
        return nil, fmt.Errorf("kev: get existing ids: %w", err)
    }
    existingSet := toStringSet(existingIDs)

    // 3. Map catalog entries → entities (now includes new fields)
    entries := make([]*kev.KEVEntry, 0, len(catalog.Vulnerabilities))
    var newEntries []*kev.KEVEntry

    for _, v := range catalog.Vulnerabilities {
        entry := mapCISAToKEV(v) // map new fields
        entries = append(entries, entry)
        if !existingSet[entry.CVEID] {
            newEntries = append(newEntries, entry)
        }
    }

    // 4. Upsert all entries
    result := &SyncResult{NewEntries: newEntries}
    for _, entry := range entries {
        if err := uc.kevRepo.Upsert(ctx, entry); err != nil {
            uc.log.Error().Err(err).Str("cve_id", entry.CVEID).Msg("kev: upsert failed")
            continue
        }
        if !existingSet[entry.CVEID] {
            result.Inserted++
        } else {
            result.Updated++
        }
        result.Total++
    }

    // 5. Publish NATS events for new entries (non-blocking, non-fatal)
    if len(newEntries) > 0 && uc.publisher != nil {
        go func() {
            pubCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            if err := uc.publisher.PublishNewKEVBatch(pubCtx, newEntries); err != nil {
                uc.log.Warn().Err(err).Int("count", len(newEntries)).Msg("kev: NATS publish failed (non-fatal)")
            }
        }()
    }

    uc.log.Info().
        Int("inserted", result.Inserted).
        Int("updated", result.Updated).
        Int("new_kev_entries", len(newEntries)).
        Msg("KEV sync complete")

    return result, nil
}

// mapCISAToKEV maps CISA API response to KEVEntry entity.
// Must map NEW fields: ShortDescription, RequiredAction, KnownRansomwareCampaignUse
func mapCISAToKEV(v CISAVulnerability) *kev.KEVEntry {
    entry := &kev.KEVEntry{
        CVEID:             v.CVEID,
        VendorProject:     v.VendorProject,
        Product:           v.Product,
        VulnerabilityName: v.VulnerabilityName,
        // NEW fields:
        ShortDescription:              v.ShortDescription,
        RequiredAction:                v.RequiredAction,
        KnownRansomwareCampaignUse:   v.KnownRansomwareCampaignUse,
    }
    // Parse dates
    if t, err := time.Parse("2006-01-02", v.DateAdded); err == nil {
        entry.DateAdded = t
    }
    if t, err := time.Parse("2006-01-02", v.DueDate); err == nil {
        entry.DueDate = t
    }
    return entry
}
```

### publisher_hook.go — CREATE hoặc MODIFY

```go
// Package fetcher — NATS JetStream event publisher for KEV sync events.
package fetcher

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"
    "github.com/osv/data-service/internal/domain/kev"
)

// KEVPublisher publishes kev.new events to NATS JetStream.
type KEVPublisher struct {
    js     nats.JetStreamContext
    logger zerolog.Logger
}

func NewKEVPublisher(nc *nats.Conn, log zerolog.Logger) (*KEVPublisher, error) {
    js, err := nc.JetStream()
    if err != nil {
        return nil, fmt.Errorf("kev publisher: jetstream: %w", err)
    }

    // Ensure stream exists (idempotent)
    _, err = js.AddStream(&nats.StreamConfig{
        Name:     "KEV_EVENTS",
        Subjects: []string{"kev.>"},
        MaxMsgs:  10000,
    })
    if err != nil && !isStreamExistsError(err) {
        return nil, fmt.Errorf("kev publisher: add stream: %w", err)
    }

    return &KEVPublisher{js: js, logger: log}, nil
}

// PublishNewKEVBatch publishes kev.new events for each new KEV entry.
// Deduplicates using NATS message ID (CVE ID).
func (p *KEVPublisher) PublishNewKEVBatch(ctx context.Context, entries []*kev.KEVEntry) error {
    for _, entry := range entries {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        payload := map[string]interface{}{
            "event":         "kev.new",
            "cve_id":        entry.CVEID,
            "product":       entry.Product,
            "vendor":        entry.VendorProject,
            "date_added":    entry.DateAdded.Format("2006-01-02"),
            "is_ransomware": entry.IsKnownRansomware,
        }
        data, _ := json.Marshal(payload)

        _, err := p.js.Publish("kev.new", data,
            nats.Context(ctx),
            nats.MsgId(entry.CVEID), // deduplication key
        )
        if err != nil {
            p.logger.Warn().Err(err).Str("cve_id", entry.CVEID).Msg("NATS publish failed")
        }
    }
    return nil
}

func isStreamExistsError(err error) bool {
    return err != nil && err.Error() == nats.ErrStreamNameAlreadyInUse.Error()
}
```

### GetAllCVEIDs — ADD to KEV repository

Thêm method vào KEV repository interface:

```go
// GetAllCVEIDs returns all CVE IDs currently in the KEV catalog.
// Used for diff detection during sync.
GetAllCVEIDs(ctx context.Context) ([]string, error)
```

Implementation (PostgreSQL):
```go
func (r *pgKEVRepo) GetAllCVEIDs(ctx context.Context) ([]string, error) {
    var ids []string
    err := r.db.SelectContext(ctx, &ids, "SELECT cve_id FROM kev_entries")
    return ids, err
}
```

## Acceptance Criteria

- [x] Sau sync, `SyncResult.NewEntries` chứa đúng các CVE IDs **lần đầu** xuất hiện trong KEV
- [x] Existing CVE IDs không được coi là "new" khi re-sync
- [x] `ShortDescription`, `RequiredAction`, `KnownRansomwareCampaignUse` được map từ CISA API response
- [x] NATS configured → `kev.new` event published với `cve_id`, `is_ransomware` fields
- [x] NATS not configured (nil publisher) → sync hoàn thành bình thường, không panic
- [x] NATS down → sync hoàn thành, NATS error logged non-fatal
- [x] NATS publish deduplication: same CVE ID không publish 2 lần (MsgId dedup)
- [x] Context timeout: NATS publish timeout sau 30s, không block sync
- [x] `go build ./...` pass không lỗi
