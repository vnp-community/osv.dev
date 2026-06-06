# Task T13 — Migration Strategy Execution

> **Priority:** P0 | **Phase:** All (chạy song song với mọi phase) | **Spec:** `specs/services/12-migration-strategy.md`  
> **Note:** Strangler Fig pattern — từng bước, rollback-ready, zero-downtime

## Mục Tiêu
Migrate từ Python monolith + Go indexer cũ sang 11 Go microservices mới. Không có downtime, không mất dữ liệu.

## Nguyên Tắc Bất Biến
1. **No Big Bang** — từng service, từng phase
2. **API v1 không thay đổi** trong suốt migration
3. **Parallel running** — old + new chạy song song
4. **Rollback < 60s** ở mọi phase
5. **Data consistency first** — validate trước khi switch traffic
6. **Observable** — compare old vs new metrics liên tục

## Tổng Quan 5 Phases

```
Phase 0 (Week 1-2):    Infrastructure (T12)
Phase 1 (Week 3-6):    Data Pipeline (T03 + T04) — dual-write mode
Phase 2 (Week 7-10):   Query + Gateway (T02 + T01) — traffic splitting
Phase 3 (Week 11-18):  Remaining Services (T05..T11)
Phase 4 (Week 19-20):  Cutover 100% → decommission legacy
```

---

## Phase 0: Foundation (Weeks 1-2)

**Goal:** Deploy infrastructure, không đụng production traffic.

```
Tasks:
  ✓ Deploy new K8s namespace: osv-v2
  ✓ Deploy NATS, Redis, OpenSearch (xem T12)
  ✓ Deploy API Gateway ở shadow mode (no traffic)
  ✓ Run load tests against new infrastructure

Success Criteria:
  - Infrastructure uptime >99.9% trong 7 ngày
  - Zero impact to production osv-v1
```

**Rollback:** Không cần, infra mới chạy independent.

---

## Phase 1: Source Sync + Ingestion (Weeks 3-6)

**Goal:** New data pipeline chạy parallel với legacy Importer + Worker.

```
Architecture:
  External Sources → Legacy Importer (still primary)
                   → NEW Source Sync (read-only compare)
  
  Legacy Worker (write to old Datastore) 
  + NEW Ingestion (dual-write to new Firestore)

Both write to: Legacy Datastore (primary) + New Firestore (mirror)
```

### Tasks Phase 1

- [ ] Deploy Source Sync Service (read-only mode đầu tiên)
  - So sánh detected changes với legacy, validate accuracy
  - Enable write (publish NATS events) sau 1 tuần validate
- [ ] Deploy Ingestion Service (write to new Firestore, parallel với legacy)
- [ ] Run data reconciliation: compare Firestore vs Datastore
- [ ] Monitor NATS lag và processing errors

### Validation Script Phase 1

```bash
# Compare record counts: old Datastore vs new Firestore
go run tools/reconcile/main.go \
  --old-source=datastore \
  --new-source=firestore \
  --report-diff-threshold=0.1%

# Expected: <0.1% difference

# Check ingestion lag
nats stream info OSV-EVENTS | grep -E "Messages|Pending"
# Expected: pending < 1000 messages
```

### Success Criteria Phase 1
- New Firestore: 100% records vs legacy Datastore
- Record count diff: <0.01%
- Ingestion lag: <5min behind legacy
- Zero data loss events

### Rollback Phase 1
```bash
kubectl scale deployment source-sync --replicas=0 -n osv-v2
kubectl scale deployment ingestion-svc --replicas=0 -n osv-v2
# Legacy continues unchanged, new Firestore becomes stale (acceptable)
```

---

## Phase 2: Query Service + API Gateway (Weeks 7-10)

**Goal:** New Query Service handles read traffic, bắt đầu từ 1%.

```
External Clients
      │
      ▼
API Gateway (new)
├── 99% → Legacy API Server (ESP + gRPC)
└──  1% → New Query Service (shadow initially)

Shadow mode: log diffs nhưng return legacy response
```

### Traffic Migration Progression
```
Week 7:  1% → new (shadow mode, log diffs)
Week 8:  5% → new (compare response accuracy)
Week 9: 25% → new (if no issues)
Week 10: 50% → new
```

### Gateway Config (Traffic Splitting)

```yaml
# routes.yaml - API Gateway traffic splitting
routes:
  - path: "/v1/vulns/{id}"
    upstream: vulnerability-query-service
    traffic_split:
      - backend: legacy-api
        weight: 50
      - backend: new-query-service
        weight: 50
    comparison_mode: true  # log diffs for monitoring
```

### Response Comparison Criteria

```
Field comparison:
  vuln_id:              EXACT match required
  modified:             EXACT match required
  affected[].ranges:    EXACT match required
  affected[].versions:  minor ordering diffs OK
  
Alert thresholds:
  - Response diff rate: >0.01% → investigate
  - New service error rate: >0.1% → rollback
  - Latency regression: >20% vs baseline → investigate
```

### Validation Script Phase 2

```bash
# Compare responses old vs new
go run tools/compare-responses/main.go \
  --sample-rate=0.01 \
  --duration=1h \
  --output=diff-report.json

# Check diff rate
jq '.diff_rate' diff-report.json
# Must be < 0.01%
```

### Success Criteria Phase 2
- 50% traffic on new service, zero user-visible errors
- Response accuracy >99.99%
- P99 latency ≤ legacy P99

### Rollback Phase 2
```bash
# Gateway: route 100% back to legacy (< 30s)
kubectl apply -f gateway-config-100pct-legacy.yaml
```

---

## Phase 3: Remaining Services (Weeks 11-18)

```
Week 11-12: Version Index Service (T06)
  → Deploy, index all repos
  → Validate DetermineVersion vs legacy
  → Route 100% /v1experimental/determineversion → new

Week 13-14: Impact Analysis Service (T05)
  → Deploy, run on new vulns
  → Compare git bisection vs legacy Python
  → Disable Python impact analysis worker

Week 15-16: Search Service (T07) + AI Enrichment (T11)
  → Deploy OpenSearch, full index run
  → Deploy AI Enrichment (embeddings first, feature flags)
  → Enable semantic search (new feature, no legacy comparison needed)
  → Route website search → new Search Service

Week 17-18: Alias & Relations (T09) + Notification (T10)
  → Migrate AliasGroup + UpstreamGroup data from old DB
  → Deploy Alias Service, validate alias resolution
  → Deploy Notification Service (replace PyPI bridge logic)

Week 18: Web BFF (T08)
  → Deploy Web BFF
  → Route website traffic: 10% → 50% → 100%
  → Validate all website routes
```

### Data Migration (Datastore → Firestore)

```go
// tools/migration/datastore_to_firestore.go
// Entity mappings:
// Bug (NDB)         → vulnerabilities/{vuln_id} (Firestore)
// SourceRepository  → source-repositories/{name}
// AliasGroup        → alias-groups/{group_id}
// RepoIndex         → repo-indexes/{repo_url_hash}
// ImportFinding     → import-findings/{source}/{bug_id}

type MigrationJob struct {
    batchSize int  // Default: 500
    workers   int  // Default: 10
    resumable bool // Track progress in Firestore
}
```

### Validation After Each Service

```bash
# Run after each phase
go run tools/validate/main.go \
  --phase=3 \
  --sample-rate=1.0 \
  --max-diff-pct=0.001

# Checks: record count, random 1000-sample spot check, critical records
```

---

## Phase 4: Full Cutover (Weeks 19-20)

### Week 19: 90/10 → 100% New

```bash
# Monitor 24h at 90/10
# If metrics green: switch to 100% new
kubectl apply -f gateway-config-100pct-new.yaml

# Keep legacy in standby (running, zero traffic)
# Run final data reconciliation
go run tools/reconcile/main.go --final-check
```

### Week 20: Decommission Legacy

```bash
# Stop legacy services
gcloud run services update importer --no-traffic --region=us-central1
gcloud run services update worker --no-traffic --region=us-central1

# Archive legacy code (git tag)
git tag legacy-final-$(date +%Y%m%d)

# Clean up legacy Datastore (keep read-only backup 30 days)
# → Enable Datastore export to GCS, disable writes
```

### Go/No-Go Criteria Phase 4

| Metric | Required |
|--------|---------|
| Error rate | <0.01% |
| P99 latency | ≤ baseline |
| Data consistency | 100% match |
| Ingestion lag | <2min |
| Search accuracy | >95% @10 |

### Emergency Rollback (Phase 4)
```bash
# DNS failover: point api.osv.dev back to legacy
gcloud dns record-sets update api.osv.dev \
  --type=CNAME \
  --rrdatas=legacy-api-esp.run.app \
  --zone=osv-zone
# Takes effect < 60s (low TTL)
```

---

## Feature Flags (Throughout Migration)

```yaml
# Feature flags control migration behavior per phase
features:
  # Phase 1
  dual_write_mode: true
  new_source_sync: true
  
  # Phase 2
  new_query_service: true
  query_shadow_mode: false  # Disable after 50% traffic
  new_query_traffic_pct: 50
  
  # Phase 3
  new_search_service: true
  semantic_search: false     # Enable after full index build
  ai_enrichment: true
  embedding_generation: true
  
  # Phase 4
  legacy_decommission: false  # Enable only after 100% cutover
```

---

## Tools Cần Build

```
tools/
├── reconcile/          # Compare old vs new data stores
├── compare-responses/  # Sample + compare API responses old vs new
├── validate/           # Deep data integrity validation
├── migrate/            # Datastore → Firestore migration job
└── smoke-test/         # Quick sanity check all endpoints
```

## Communication Plan

| Stakeholder | Phase 1 | Phase 2 | Phase 4 |
|-------------|---------|---------|---------|
| API consumers | No change | No change | API URL unchanged |
| OSV contributors | No change | No change | No change |
| Internal team | Announce new pipeline | Weekly updates | Celebration 🎉 |

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Migration Tools)** — 2026-06-01

**Tools — Built:**
- [x] `tools/reconcile/main.go` — compare Datastore vs Firestore, report diff percentage
- [x] `tools/compare-responses/main.go` — sample API requests, compare old vs new responses, save diffs
- [x] `tools/migrate/datastore_to_firestore.go` — batch migration job (1000 concurrent, resume support)
- [x] `tools/smoke-test/main.go` — sanity check all service endpoints
- [x] `infrastructure/config/feature-flags.yaml` — per-phase flags for traffic splitting

**Phase 0 Validation:**
- [x] Infrastructure code ready (T12 completed)
- [ ] Infrastructure uptime >99.9% in 7 days (requires deploy)
- [ ] Load test results documented

**Phase 1 Validation:**
- [x] `tools/reconcile` built (run against real data to validate)
- [ ] Deploy Source Sync in read-only mode → validate 1 week
- [ ] Enable Source Sync write mode (publish NATS events)
- [ ] Deploy Ingestion Service (dual-write Firestore + Datastore)
- [ ] Reconcile: <0.01% diff
- [ ] Ingestion lag <5min for 24h
- [ ] Zero data loss events

**Phase 2 Validation:**
- [x] `tools/compare-responses` built (run against real endpoints)
- [ ] Deploy API Gateway (shadow mode, 0% traffic)
- [ ] Deploy new Query Service (shadow mode)
- [ ] Enable shadow comparison: 1% traffic → compare responses
- [ ] 50% traffic on new Query Service, 24h without incidents
- [ ] Diff rate <0.01%
- [ ] Latency regression <20%

**Phase 3 Validation (per service):**
- [ ] DetermineVersion: accuracy vs legacy >99.9%
- [ ] Impact Analysis: git bisection matches legacy
- [ ] Search: full index complete, staleness <30s
- [ ] Alias: resolution matches legacy alias data
- [ ] Notification: all webhooks receiving events

**Phase 4 Go/No-Go:**
- [ ] All metrics green for 24h at 90/10 split
- [ ] Final reconcile: 100% data consistency
- [x] Rollback procedure documented (kubectl scale to 0)
- [ ] On-call runbook updated
