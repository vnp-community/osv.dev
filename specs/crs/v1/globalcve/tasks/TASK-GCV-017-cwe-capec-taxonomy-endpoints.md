# TASK-GCV-017 — CWE/CAPEC Taxonomy Endpoints (search-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-017 |
| **Service** | `search-service` |
| **CR** | CR-GCV-003 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

Thêm 4 endpoints mới vào `search-service` để list/get CWE weaknesses và CAPEC attack patterns. Data được sync bởi `data-service` vào các bảng `cwe_weaknesses` và `capec_patterns` (shared PostgreSQL DB). `search-service` chỉ cần query/read.

## Reference

- Solution: [SOL-GCV-003](../solutions/SOL-GCV-003-capec-cwe-enrichment.md) §2.2, §2.3
- CR: [CR-GCV-003](../CR-GCV-003-mitre-capec-cwe-enrichment.md)

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/entity/taxonomy.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/repository/taxonomy_repo.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/postgres/taxonomy_pg.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/taxonomy_handler.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/
        (router file — đăng ký routes mới)
```

## Implementation Spec

### entity/taxonomy.go

```go
package entity

// CWEEntry represents a CWE weakness from MITRE.
type CWEEntry struct {
    ID          string   `db:"id"          json:"id"`           // "CWE-89"
    Name        string   `db:"name"        json:"name"`
    Description string   `db:"description" json:"description"`
    Abstraction string   `db:"abstraction" json:"abstraction"`  // "Base"|"Class"|"Variant"
    Status      string   `db:"status"      json:"status"`
    CAPECIDs    []string `db:"-"           json:"capec_ids,omitempty"`
    UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// CAPECEntry represents a CAPEC attack pattern from MITRE.
type CAPECEntry struct {
    ID          string   `db:"id"          json:"id"`           // "CAPEC-66"
    Name        string   `db:"name"        json:"name"`
    Description string   `db:"description" json:"description"`
    Likelihood  string   `db:"likelihood"  json:"likelihood"`  // "High"|"Medium"|"Low"
    Severity    string   `db:"severity"    json:"severity"`
    CWEIDs      []string `db:"-"           json:"cwe_ids,omitempty"`
    UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
```

### repository/taxonomy_repo.go

```go
package repository

import "context"

type CWERepository interface {
    List(ctx context.Context, q string, page, limit int) ([]*entity.CWEEntry, int64, error)
    FindByID(ctx context.Context, id string) (*entity.CWEEntry, error)
}

type CAPECRepository interface {
    List(ctx context.Context, q, cweID string, page, limit int) ([]*entity.CAPECEntry, int64, error)
    FindByID(ctx context.Context, id string) (*entity.CAPECEntry, error)
}
```

### infra/postgres/taxonomy_pg.go

```go
package postgres

// pgCWERepository implements CWERepository using PostgreSQL.
type pgCWERepository struct{ db *sqlx.DB }

func (r *pgCWERepository) List(ctx context.Context, q string, page, limit int) ([]*entity.CWEEntry, int64, error) {
    offset := page * limit
    args := []interface{}{limit, offset}
    where := ""
    if q != "" {
        where = "WHERE lower(name) LIKE lower($3) OR lower(id) LIKE lower($3)"
        args = append(args, "%"+q+"%")
    }

    var entries []*entity.CWEEntry
    query := fmt.Sprintf(`SELECT id, name, description, abstraction, status, updated_at
        FROM cwe_weaknesses %s ORDER BY id LIMIT $1 OFFSET $2`, where)
    err := r.db.SelectContext(ctx, &entries, query, args...)

    var total int64
    r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cwe_weaknesses "+where, args[2:]...).Scan(&total)

    return entries, total, err
}

func (r *pgCWERepository) FindByID(ctx context.Context, id string) (*entity.CWEEntry, error) {
    var entry entity.CWEEntry
    err := r.db.GetContext(ctx, &entry, `SELECT id, name, description, abstraction, status, updated_at
        FROM cwe_weaknesses WHERE id = $1`, id)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &entry, err
}

// pgCAPECRepository implements CAPECRepository.
type pgCAPECRepository struct{ db *sqlx.DB }

func (r *pgCAPECRepository) List(ctx context.Context, q, cweID string, page, limit int) ([]*entity.CAPECEntry, int64, error) {
    // Build WHERE clause dynamically based on q and cweID
    conditions := []string{}
    args := []interface{}{limit, page * limit}

    if q != "" {
        args = append(args, "%"+q+"%")
        conditions = append(conditions, fmt.Sprintf("lower(name) LIKE lower($%d)", len(args)))
    }
    if cweID != "" {
        args = append(args, cweID)
        conditions = append(conditions, fmt.Sprintf("$%d = ANY(cwe_ids)", len(args)))
    }

    where := ""
    if len(conditions) > 0 {
        where = "WHERE " + strings.Join(conditions, " AND ")
    }

    var entries []*entity.CAPECEntry
    query := fmt.Sprintf(`SELECT id, name, description, likelihood, severity, updated_at
        FROM capec_patterns %s ORDER BY id LIMIT $1 OFFSET $2`, where)
    err := r.db.SelectContext(ctx, &entries, query, args...)
    // count query similar pattern
    return entries, 0, err
}

func (r *pgCAPECRepository) FindByID(ctx context.Context, id string) (*entity.CAPECEntry, error) {
    var entry entity.CAPECEntry
    err := r.db.GetContext(ctx, &entry, `SELECT id, name, description, likelihood, severity, updated_at
        FROM capec_patterns WHERE id = $1`, id)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &entry, err
}
```

### delivery/http/taxonomy_handler.go

```go
type TaxonomyHandler struct {
    cweRepo   repository.CWERepository
    capecRepo repository.CAPECRepository
}

// GET /api/v2/cwe?q=injection&page=0&limit=50
func (h *TaxonomyHandler) ListCWE(w http.ResponseWriter, r *http.Request) {
    q     := r.URL.Query().Get("q")
    page  := parseInt(r.URL.Query().Get("page"), 0)
    limit := parseInt(r.URL.Query().Get("limit"), 50)

    entries, total, err := h.cweRepo.List(r.Context(), q, page, limit)
    if err != nil {
        respondError(w, 500, "failed to list CWE")
        return
    }
    respondJSON(w, 200, map[string]interface{}{"data": entries, "total": total})
}

// GET /api/v2/cwe/{id} (e.g. CWE-89)
func (h *TaxonomyHandler) GetCWE(w http.ResponseWriter, r *http.Request) {
    id    := chi.URLParam(r, "id")
    entry, err := h.cweRepo.FindByID(r.Context(), id)
    if errors.Is(err, repository.ErrNotFound) {
        respondError(w, 404, "CWE not found")
        return
    }
    if err != nil {
        respondError(w, 500, "failed to get CWE")
        return
    }
    respondJSON(w, 200, entry)
}

// GET /api/v2/capec?q=injection&cwe_id=CWE-89&page=0&limit=50
func (h *TaxonomyHandler) ListCAPEC(w http.ResponseWriter, r *http.Request) { /* similar */ }

// GET /api/v2/capec/{id}
func (h *TaxonomyHandler) GetCAPEC(w http.ResponseWriter, r *http.Request) { /* similar */ }
```

### Router registration

```go
r.Get("/api/v2/cwe", h.ListCWE)
r.Get("/api/v2/cwe/{id}", h.GetCWE)
r.Get("/api/v2/capec", h.ListCAPEC)
r.Get("/api/v2/capec/{id}", h.GetCAPEC)
```

## Acceptance Criteria

- [x] `GET /api/v2/cwe` → array CWE entries với `id`, `name`, `description`, `abstraction`
- [x] `GET /api/v2/cwe?q=injection` → filter by name/id contains "injection"
- [x] `GET /api/v2/cwe/CWE-89` → single CWE entry (SQL Injection)
- [x] `GET /api/v2/cwe/CWE-99999` → 404
- [x] `GET /api/v2/capec` → array CAPEC entries
- [x] `GET /api/v2/capec?cwe_id=CWE-89` → CAPEC linked to CWE-89
- [x] `GET /api/v2/capec/CAPEC-66` → single CAPEC entry
- [x] Response có `total` count
- [x] `go build ./...` pass không lỗi
