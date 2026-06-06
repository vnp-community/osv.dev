# OSS-Fuzz Python Code — Isolation Package

> **Status:** TASK-10-03 — Isolate OSS-Fuzz Python code into `osv/ossfuzz/` package.  
> **Date:** 2026-06-03

## Purpose

This package contains Python code that is **kept exclusively for OSS-Fuzz (ClusterFuzz) integration**.  
All other Python functionality has been migrated to Go microservices.

## What lives here

| Class / Module | Original location | Purpose |
|---|---|---|
| `RegressResult` | `osv/models.py` | Stores bisection regression results for OSS-Fuzz |
| `FixResult` | `osv/models.py` | Stores bisection fix results for OSS-Fuzz |
| `IDCounter` | `osv/models.py` | Counter for generating OSV IDs (OSS-Fuzz specific) |
| Worker tasks | `gcp/workers/worker/` | Impact/bisection tasks driven by ClusterFuzz |

## What has been migrated to Go

| Python module | Go replacement |
|---|---|
| `osv/impact.py` | `services/impact-analysis/` |
| `osv/models.py` (Bug, SourceRepository, AliasGroup) | `services/pkg/models/` |
| `osv/sources.py` | `services/ingestion/` |
| `osv/ecosystems/` | `services/pkg/ecosystem/impl/` |
| `osv/semver_index.py` | `services/pkg/ecosystem/impl/semver.go` |
| `osv/purl_helpers.py` | `services/pkg/purl/` |

## Deprecation Plan

After Go equivalents are stable for ≥ 2 weeks:

- **Phase 1** (after `services/impact-analysis/` stable): Deprecate `osv/impact.py`, `osv/repos.py`
- **Phase 2** (after ingestion migration): Deprecate `osv/sources.py`, `osv/models.py` (non-OSS-Fuzz)  
- **Phase 3** (final cleanup): Deprecate `osv/ecosystems/`, `osv/semver_index.py`, `osv/purl_helpers.py`

## Pre-deletion Checklist (for each Python file)

Before removing any file, verify:
- [ ] `grep -r "from osv import [module]" . -- zero results`
- [ ] Go replacement stable ≥ 2 weeks in production
- [ ] All test cases covered by Go replacement tests
- [ ] Architecture diagram updated
- [ ] Team notified via PR description

## OSS-Fuzz Workflow (stays Python)

```
ClusterFuzz detects crash
  → Triggers OSS-Fuzz bisection task (Python worker)
  → Uses RegressResult / FixResult (this package)
  → Result stored in Firestore
  → Go ingestion-service picks up and creates OSV record
```

This workflow is **not yet migrated to Go** due to deep ClusterFuzz integration dependencies.
Migration is tracked as optional post-Sprint-10 work (P3).
