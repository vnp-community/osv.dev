# ⚠️ DEPRECATED: bindings/go

> **Status:** DEPRECATED — Do not use for new code  
> **Migration:** Use `services/pkg/clients/` instead  
> **Timeline:** Will be removed after 2 sprints (approximately 2026-07-01)

---

## Migration Guide

The Go bindings in this directory have been merged into `services/pkg/clients/` for consistency with the rest of the Go service stack.

### Replace imports

| Old import | New import |
|-----------|-----------|
| `osv.dev/bindings/go/osvdev` | `github.com/osv/pkg/clients/osvdev` |
| `osv.dev/bindings/go/osvdevexperimental` | `github.com/osv/pkg/clients/osvdevexperimental` |
| `osv.dev/bindings/go/api` | `github.com/osv/pkg/clients/api` |

### Steps

1. Update your `go.mod` to use `github.com/osv/pkg` instead of `osv.dev/bindings/go`
2. Replace imports in your code using the table above
3. Run `go build ./...` to verify

---

## Contents (for reference only)

- `osvdev/` — OSV.dev API client (v1 + experimental)  
- `api/` — Generated proto bindings for OSV API  
