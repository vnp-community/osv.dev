# Migration Strategy — Monolith to Microservices

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P0  
> **Pattern:** Strangler Fig + Blue-Green Deployment  
> **Risk Level:** High — production system with millions of records

---

## 1. Nguyên Tắc Migration

### 1.1 Core Principles

| Principle | Description |
|-----------|-------------|
| **No Big Bang** | Không chuyển đổi toàn bộ cùng một lúc. Từng service, từng phase |
| **Backward Compatible** | API v1 không thay đổi trong suốt migration |
| **Parallel Running** | Old và new systems chạy song song, traffic splitting dần |
| **Rollback Ready** | Mọi phase đều có rollback plan rõ ràng |
| **Data Consistency First** | Đảm bảo data consistency trước khi switch traffic |
| **Observable** | Measure everything — so sánh old vs new metrics liên tục |

### 1.2 Migration Pattern: Strangler Fig

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4
  Legacy    Dual-write  Traffic   Cutover  Legacy
  Only      Mode        Split     90/10    Decommission

Traffic:
  100% Legacy → 100% Legacy + New writes → 50/50 → 10/90 → 100% New
```

---

## 2. Pre-Migration Checklist

### 2.1 Infrastructure Prerequisites

- [ ] Kubernetes cluster provisioned (GKE)
- [ ] NATS cluster deployed (3-node, JetStream enabled)
- [ ] Redis Cluster deployed (HA mode)
- [ ] OpenSearch cluster deployed (3 shards, 1 replica)
- [ ] Firestore collections scoped for new services
- [ ] OpenTelemetry Collector deployed
- [ ] Prometheus + Grafana dashboards ready
- [ ] Jaeger/Tempo tracing backend ready

### 2.2 Shared Libraries Ready

- [ ] `pkg/osvschema` — OSV Schema Go types
- [ ] `pkg/ecosystem` — 30+ ecosystem helpers
- [ ] `pkg/purl` — PURL parsing
- [ ] `pkg/semver` — SemVer normalization
- [ ] `pkg/osv_proto` — Shared protobuf definitions
- [ ] `pkg/middleware` — Shared gRPC/HTTP middleware

### 2.3 Data Baseline

- [ ] Export full Firestore snapshot (baseline)
- [ ] Document current Datastore schema → Firestore mapping
- [ ] Verify record count per ecosystem
- [ ] Benchmark current API latency (P50, P99)

---

## 3. Migration Phases

### Phase 0: Foundation (Weeks 1-2)

**Goal:** Deploy infrastructure without touching production traffic.

```
Tasks:
  ✓ Deploy new Kubernetes namespace: osv-v2
  ✓ Deploy NATS cluster
  ✓ Deploy Redis Cluster
  ✓ Deploy OpenSearch cluster
  ✓ Set up OpenTelemetry collector
  ✓ Set up monitoring dashboards
  ✓ Deploy API Gateway (shadow mode - no traffic)
  ✓ Run load tests against new infrastructure

Success Criteria:
  - Infrastructure 99.9% uptime over 7 days
  - No impact to production osv-v1
```

### Phase 1: Source Sync + Ingestion (Weeks 3-6)

**Goal:** New data pipeline running in parallel with legacy Importer + Worker.

```
                    ┌─────────────────────────────────┐
External Sources ──►│ Legacy Importer (still primary) │
                    └────────────┬────────────────────┘
                                 │ Pub/Sub
                    ┌────────────▼────────────────────┐
                    │ Legacy Worker (still primary)    │
                    │ + NEW: Source Sync Service       │
                    │   (dual-write mode)              │
                    └────────────┬────────────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
              ▼                  ▼                  ▼
        Legacy              New Firestore       NATS Events
        Datastore            (mirror)          (new pipeline)
```

**Tasks:**
- Deploy Source Sync Service (read-only mode, compare with legacy)
- Deploy Ingestion Service (write to new Firestore, parallel with legacy)
- Run data reconciliation jobs: compare new Firestore vs old Datastore
- Monitor Kafka/NATS lag and processing errors

**Validation:**
```bash
# Compare record counts: old vs new
go run tools/reconcile/main.go \
  --old-source=datastore \
  --new-source=firestore \
  --namespace=osv-v2 \
  --report-diff-threshold=0.1%

# Expected: < 0.1% difference in record counts
```

**Success Criteria:**
- New Firestore has 100% of records from legacy Datastore
- Record counts match within 0.01%
- Ingestion lag < 5 minutes behind legacy
- Zero data loss events

**Rollback:** Stop new services; legacy continues unchanged.

---

### Phase 2: Query Service + API Gateway (Weeks 7-10)

**Goal:** New Query Service handles read traffic, starting at 1%.

```
External Clients
       │
       ▼
  API Gateway (new)
  ├── 99% → Legacy API Server (ESP + gRPC)
  └──  1% → New Query Service (shadow)

Both paths write to same Datastore/Firestore
Shadow responses logged but not returned to client
```

**Tasks:**
- Deploy Vulnerability Query Service
- Deploy API Gateway with shadow mode (1% → new, 99% → legacy)
- Compare responses: new vs legacy for same requests
- Fix any discrepancies
- Gradually increase new traffic: 1% → 5% → 10% → 25% → 50%

**Traffic Migration Script:**
```yaml
# Gateway config: gradual traffic shift
routing:
  vulnerability-query:
    backends:
      - name: legacy-api
        weight: 50      # Reduce gradually over weeks
      - name: new-query-service
        weight: 50      # Increase gradually
    comparison_mode: true   # Log diffs for monitoring
```

**Validation:**
```
Response comparison criteria:
  - vuln_id: must match exactly
  - modified: must match exactly
  - affected[].versions: may have minor ordering differences (acceptable)
  - affected[].ranges: must match exactly
  
Monitor dashboard:
  - Response diff rate: target < 0.01%
  - New service error rate: < 0.1%
  - Latency regression: < 20% vs baseline
```

**Success Criteria:**
- 50% traffic on new service, zero user-visible errors
- Response accuracy > 99.99%
- P99 latency ≤ legacy P99 latency

---

### Phase 3: Remaining Services (Weeks 11-18)

Deploy remaining services in priority order:

```
Week 11-12: Version Index Service
  - Deploy, index all repos
  - Validate DetermineVersion responses vs legacy
  - Route 100% of /v1experimental/determineversion to new service

Week 13-14: Impact Analysis Service
  - Deploy, run analysis on new vulns
  - Compare git bisection results vs legacy
  - Replace legacy Python impact analysis

Week 15-16: Search Service
  - Deploy OpenSearch, run full index
  - Deploy AI Enrichment Service (embeddings first)
  - Enable semantic search (new feature)
  - Route website search to new Search Service

Week 17-18: Alias & Relations Service
  - Migrate AliasGroup + UpstreamGroup data
  - Deploy service
  - Validate alias resolution matches legacy

Week 18: Notification Service + Web BFF
  - Deploy Notification Service (replace PyPI bridge logic)
  - Deploy Web BFF (replace Flask website)
  - Test all website routes
```

---

### Phase 4: Full Cutover (Weeks 19-20)

**Goal:** 100% traffic on new services, legacy decommission begins.

```
Week 19: 90/10 → 100% new
  - Monitor 24h at 90/10 split
  - Switch to 100% new if metrics green
  - Keep legacy in standby (no traffic, but running)
  - Run final data reconciliation

Week 20: Legacy decommission
  - Stop legacy importer + worker
  - Stop legacy API server
  - Archive legacy code
  - Clean up legacy Datastore (keep as read-only backup 30 days)
```

**Go/No-Go Criteria:**
| Metric | Required |
|--------|---------|
| Error rate | < 0.01% |
| P99 latency | ≤ baseline |
| Data consistency | 100% match |
| Ingestion lag | < 2min |
| Search accuracy | > 95% @10 |

---

## 4. Data Migration

### 4.1 Firestore Schema Migration

```go
// tools/migration/datastore_to_firestore.go

// Migration maps Python NDB entities → Go Firestore documents.
// Runs as a one-time job with resumability.

type MigrationJob struct {
    oldDS   *datastore.Client   // Old GCP Datastore
    newFS   *firestore.Client   // New Firestore
    batchSize int               // Default: 500
    workers  int                // Default: 10
}

// Entity mapping:
// Bug (NDB) → vulnerabilities/{vuln_id} (Firestore)
// SourceRepository → source-repositories/{name}
// AliasGroup → alias-groups/{group_id}
// RepoIndex → repo-indexes/{repo_url_hash}
// ImportFinding → import-findings/{source}/{bug_id}
```

### 4.2 Migration Validation

```bash
# Run after each phase to verify data integrity
go run tools/validate/main.go \
  --phase=2 \
  --sample-rate=1.0 \   # 100% check during migration
  --max-diff-pct=0.001

# Checks:
# 1. Record count matches
# 2. Random 1000-sample spot check (deep equality)
# 3. Critical records (high CVSS) always checked
# 4. Recently modified records checked
```

---

## 5. Rollback Procedures

### 5.1 Phase 1 Rollback
```bash
# Stop new Source Sync and Ingestion services
kubectl scale deployment source-sync --replicas=0 -n osv-v2
kubectl scale deployment ingestion-svc --replicas=0 -n osv-v2
# Legacy continues; new Firestore data becomes stale (acceptable)
```

### 5.2 Phase 2 Rollback
```bash
# Gateway: route 100% back to legacy
kubectl apply -f gateway-config-100pct-legacy.yaml
# Takes effect in < 30s
```

### 5.3 Full Rollback (Emergency)
```bash
# DNS failover: point api.osv.dev to legacy Cloud Run
gcloud dns record-sets update api.osv.dev \
  --type=CNAME \
  --rrdatas=legacy-api-esp.run.app \
  --zone=osv-zone
# Takes effect in < 60s (low TTL)
```

---

## 6. Feature Flags

```yaml
# Feature flags control migration behavior
features:
  # Phase 1
  dual_write_mode: true          # Write to both old + new datastores
  new_source_sync: true          # Run new Source Sync Service
  
  # Phase 2
  new_query_service: true        # Enable new Query Service
  query_shadow_mode: true        # Log diffs but use legacy response
  new_query_traffic_pct: 50      # 50% traffic to new query service
  
  # Phase 3
  new_search_service: true
  semantic_search: false         # Enable only after full index
  ai_enrichment: true
  embedding_generation: true
  
  # Phase 4
  legacy_decommission: false     # Enable only after 100% cutover
```

---

## 7. Communication Plan

| Stakeholder | Phase 1 | Phase 2 | Phase 4 |
|-------------|---------|---------|---------|
| API consumers | No change | No change | API URL unchanged |
| OSV contributors | No change | No change | No change |
| Data consumers | No change | No change | No change |
| Internal team | Announce | Weekly updates | Celebration 🎉 |

---

## 8. Timeline Summary

| Phase | Duration | Risk | Rollback Time |
|-------|----------|------|--------------|
| 0: Foundation | 2 weeks | Low | N/A |
| 1: Data Pipeline | 4 weeks | Medium | < 1 minute |
| 2: Query + Gateway | 4 weeks | High | < 30 seconds |
| 3: Remaining Services | 8 weeks | Medium | < 1 minute |
| 4: Cutover | 2 weeks | High | < 60 seconds |
| **Total** | **~20 weeks** | | |

---

## 9. Implementation Status

> **Status:** ✅ Migration Tools Built | **Updated:** 2026-06-01

### Implemented
- [x] `tools/reconcile/main.go` — Compare Datastore vs Firestore record counts, report diff percentage
- [x] `tools/compare-responses/main.go` — Sample API requests, compare old vs new JSON responses, save diffs
- [x] `tools/migrate/datastore_to_firestore.go` — Batch migration job (1000 concurrent workers, resume support via cursor)
- [x] `tools/smoke-test/main.go` — Sanity check all service endpoints (health + basic data)
- [x] `infrastructure/config/feature-flags.yaml` — Per-phase flags for traffic splitting control
- [x] Rollback procedures documented (kubectl scale to 0 for Phase 1/2)

### Pending (Deployment gates, not code)
- [ ] Phase 0: Apply infrastructure Terraform → verify 7-day uptime
- [ ] Phase 0: Run load tests against new infrastructure  
- [ ] Phase 1: Deploy Source Sync (read-only mode), validate 1 week of data
- [ ] Phase 1: Enable Source Sync write mode (publish NATS events)
- [ ] Phase 1: Deploy Ingestion Service dual-write; run reconcile → <0.01% diff
- [ ] Phase 2: Deploy API Gateway (shadow 0%) + Query Service
- [ ] Phase 2: Ramp traffic 1% → 5% → 10% → 25% → 50%; monitor diffs
- [ ] Phase 3: Deploy remaining services per weekly schedule
- [ ] Phase 4: 90/10 monitoring, full cutover, legacy decommission

### Notes
- All migration tools use Go standard library + official GCP SDK (no external dependencies)
- reconcile tool uses 500-worker batch jobs with Firestore pagination cursors for resume support
