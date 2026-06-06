# Python Codebase Cleanup Plan
# TASK-10-04: Mark deprecated Python code and prepare for archival

## Deprecation Strategy

Following the Strangler Fig pattern, Python code is deprecated in phases
as Go equivalents reach production parity.

## Phase 1 — Already Deprecated / Removed ✅

| File/Directory | Replaced By | Status |
|----------------|------------|--------|
| `tools/` (legacy scripts) | `services/cvectl/` | ✅ Archived |
| `external/` (sync scripts) | `services/source-sync/` | ✅ Removed |
| `bindings/go/` | `services/pkg/clients/` | ✅ Merged |
| `gcp/` old Cloud Functions | Go services + GKE | ✅ Removed |

## Phase 2 — Deprecated (Go Parity Reached) 🔄

These files have Go equivalents but Python is still running in production.

### `osv/sources.py`
- **Replaced by**: `services/source-sync/internal/application/sourcesloader/`
- **Deprecation marker**: Add `# DEPRECATED: Use source-sync service` to all public functions
- **Timeline**: Remove after 30-day traffic validation

### `vulnfeeds/` (CLI commands)
- **Replaced by**: `services/converter/` + `services/cvectl/ convert`
- **Deprecation marker**: Add deprecation warnings to CLI entry points
- **Timeline**: Archive after 60 days

### `osv/impact.py` (version enumeration only)
- **Replaced by**: `services/impact-analysis/internal/domain/service/analyzer/`
- **Still needed**: Git bisection logic (osv/impact.py::RepoAnalyzer)
- **Timeline**: Partial deprecation; bisection remains in Python

## Phase 3 — Target Deprecations 📋

| File | Go Replacement | ETA |
|------|---------------|-----|
| `osv/sources.py` | source-sync/sourcesloader | Q2 2027 |
| `osv/impact.py` (version enum) | impact-analysis/analyzer | Q2 2027 |
| `osv/ecosystems.py` | pkg/ecosystem/impl | Q3 2027 |
| `osv/models.py` (Datastore) | pkg/database/firestore | Q3 2027 |
| `osv/server.py` | api-gateway | ✅ Done |

## Deprecation Markers

Add the following header to all deprecated Python files:

```python
# DEPRECATED: This module has been ported to Go.
# Go replacement: services/<service>/internal/...
# This file will be removed after <date>.
# See: https://github.com/google/osv.dev/issues/XXX
```

## Cleanup Checklist

- [ ] Add deprecation headers to `osv/sources.py`
- [ ] Add deprecation headers to `vulnfeeds/*.py`
- [ ] Add deprecation warnings to `osv/impact.py` version enumeration functions
- [ ] Update Python CI to warn (not fail) on deprecated modules
- [ ] Remove `gcp/` old Cloud Function code
- [ ] Archive `tools/` directory
- [ ] Update README.md to reflect new architecture
- [ ] Notify downstream users of deprecation timeline
