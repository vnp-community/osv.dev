# 05 — Feature Matrix (PRD/SRS/URD → Service Mapping)

> **Mục đích**: Đảm bảo mọi chức năng trong PRD/SRS/URD đều được phủ bởi services
> và accessible qua apps/cli hoặc apps/osv.

---

## 1. PRD Features → Service Coverage

| PRD Feature | Service | CLI Command | OSV App Endpoint |
|-------------|---------|-------------|-----------------|
| **Vulnerability Query API** | gateway-service + data-service | `cmd/query --cve/--package/--commit` | `GET /v1/vulns/{id}`, `POST /v1/query` |
| **Web Interface** | gateway-service (BFF) | — | `GET /` → Next.js frontend |
| **Data Ingestion** | data-service + NATS | `cmd/importer` | `POST /api/v1/ingest` (admin) |
| **Data Dumps** | data-service | `cmd/exporter` | `GET /api/v1/vulns?bulk=true` |
| **Open Schema (OSV)** | data-service (OSV schema storage) | `cmd/query --format osv` | JSON response follows OSV schema |

---

## 2. SRS Functional Requirements → Service Mapping

| FR | Requirement | Service | Implementation |
|----|-------------|---------|----------------|
| **FR-01** | API query by (package,version) and commit | gateway-service → data-service | `POST /v1/query` → `cvedb.LookupCVEs` gRPC |
| **FR-02** | Routine data ingestion | data-service + NATS | `cmd/importer` → NATS `osv.vuln.imported` → data-service consumer |
| **FR-03** | Bisection/impact analysis | data-service + ai-service | `cmd/worker` → `ai-service.EnrichCVE` gRPC |
| **FR-04** | Public web interface | gateway-service (BFF) | `apps/osv` serves HTML via gateway-service |
| **FR-05** | Data export to GCS/bulk | data-service | `cmd/exporter` → `GET /api/v1/vulns` REST |

---

## 3. URD User Requirements → App Coverage

| UR | User | Requirement | CLI | OSV App |
|----|------|-------------|-----|---------|
| **UR-01** | Developer | Search by package+version | `osv-query --package X --version Y` | `POST /v1/query` |
| **UR-02** | Developer | Vulnerability details + remediation | `osv-query --cve CVE-X` | `GET /v1/vulns/{id}` + AI enrichment |
| **UR-03** | Developer | Browse via web UI | — | `GET /` → web interface |
| **UR-04** | Tool Builder | Programmatic API (package, commit) | `osv-query` → JSON output | REST API `/v1/query`, `/v1/querybatch` |
| **UR-05** | Tool Builder | Low-latency API response | gateway-service (<100ms p50) | gateway caches via Redis |
| **UR-06** | Tool Builder | Bulk data dumps | `osv-exporter` | `GET /api/v1/dump` |
| **UR-07** | Tool Builder | Machine-readable OSV schema | `osv-query --format osv` | JSON follows OSV schema |
| **UR-08** | Security Analyst | Standardized vuln description | — | OSV schema compliance |
| **UR-09** | Security Analyst | Contribute vuln data | `osv-import` | `POST /api/v1/vulns` (authenticated) |

---

## 4. Python `osv/` → Go Service Feature Parity

### osv/bug.py → data-service

| Python function | Go equivalent | Service |
|-----------------|---------------|---------|
| `normalize_tag(tag)` | Version normalization in ingest | data-service/usecase/ingest |
| `normalize_tags(tags)` | Batch version normalization | data-service/usecase/ingest |
| `populate_indices(bug)` | Search index population | search-service/usecase |
| `BugStatus` enum | Finding state machine | finding-service/domain/finding/state_machine.go |

### osv/ecosystems/ → search-service

| Python module | Go equivalent | Service |
|---------------|---------------|---------|
| `ecosystems/pypi.py` | `ecosystem_impl/pypi.go` | search-service (pkg ecosystem) |
| `ecosystems/maven.py` | `ecosystem_impl/maven.go` | search-service |
| `ecosystems/alpine.py` | `ecosystem_impl/alpine.go` | search-service |
| `ecosystems/debian.py` | `ecosystem_impl/debian.go` | search-service |
| `ecosystems/_ecosystems.py` (registry) | `ecosystem/impl/provider.go` | shared/pkg/ecosystem |
| `ecosystems/semver_ecosystem_helper.py` | `ecosystem/semver.go` | shared/pkg/ecosystem |
| **Missing**: NuGet, RubyGems, Packagist | **TODO**: Add Go implementations | search-service |

### osv/impact.py → data-service + ai-service

| Python function | Go equivalent | Service |
|-----------------|---------------|---------|
| `analyze_impact()` | Ingest pipeline enrichment | data-service/usecase/ingest |
| Commit bisection | `usecase/bisect/` (TODO) | data-service |
| Version enumeration | `ecosystem/impl/*.EnumerateVersions()` | shared/pkg/ecosystem |

### osv/models.py → data-service

| Python class | Go equivalent | Service |
|--------------|---------------|---------|
| `Bug` (Firestore model) | `domain/vuln/entity.go` | data-service |
| `SourceRepository` | `domain/source/source_repo.go` | data-service |
| `AliasGroup` | `infra/persistence/postgres/alias_group_repo.go` | data-service |

### osv/gcs.py → data-service infra

| Python function | Go equivalent | Service |
|-----------------|---------------|---------|
| `upload_osv_to_gcs()` | `infra/storage/gcs/gcs_store.go` | data-service |
| `download_from_gcs()` | `infra/storage/gcs/gcs_source.go` | data-service |

### osv/pubsub.py → NATS (shared/pkg/nats)

| Python function | Go equivalent | Description |
|-----------------|---------------|-------------|
| `publish_message()` | `shared/pkg/nats/publisher.go` | Publish vuln events |
| `subscribe()` | `shared/pkg/nats/subscriber.go` | Consume events |

---

## 5. Missing Features (Gap Analysis)

### 5.1 Features cần implement (chưa có trong services)

| Feature | PRD/URD | Service cần implement | Priority |
|---------|---------|----------------------|----------|
| Commit-based query | URD UR-04 | data-service: `LookupByCommit` RPC | P1 |
| Ecosystem enumeration | SRS FR-03 | search-service: port Python ecosystems | P1 |
| Sitemap generation | — | gateway-service: `/sitemap.xml` endpoint | P2 |
| Custom metrics | — | Prometheus metrics in each service | P2 |
| OSV linting | — | data-service: schema validation endpoint | P2 |
| Web interface (HTML) | PRD, URD UR-03 | gateway-service: serve Next.js | P1 |
| GCS data dump export | URD UR-06 | data-service: `/api/v1/dump` bulk export | P1 |
| Impact analysis (bisect) | SRS FR-03 | data-service: commit bisection | P3 |
| Record consistency check | — | data-service: `/admin/check-records` | P2 |

### 5.2 Features đã có đầy đủ

| Feature | Service | Status |
|---------|---------|--------|
| CVE lookup by ID | data-service (cvedb) | ✅ via LookupCVEs |
| AI enrichment (summary, impact) | ai-service | ✅ EnrichCVE gRPC |
| EPSS scoring | ai-service | ✅ GetEPSS gRPC |
| Full-text search | search-service | ✅ osv_handler.go |
| Authentication / JWT | identity-service | ✅ login/register/TOTP |
| Security findings tracking | finding-service | ✅ HTTP + gRPC |
| Container scanning | scan-service (Trivy) | ✅ trivy_client.go |
| Alert notifications | notification-service | ✅ rule + SSE |
| GraphQL BFF | gateway-service | ✅ schema/resolver/server |
| Rate limiting | gateway-service | ✅ middleware/ratelimit.go |

---

## 6. OSV v1 API Compatibility

Apps phải implement đầy đủ OSV v1 API spec tại gateway-service:

```
POST /v1/query          → UR-01, UR-04
GET  /v1/vulns/{id}     → UR-02, UR-04, UR-07
POST /v1/querybatch     → UR-06, UR-04
GET  /v1/vulns/{id}/determineversion → UR-04 (version determination)
```

Hiện trạng `gateway-service/cmd/server/main.go`:
- ✅ `GET /v1/vulns/{id}` — stub (proxied to data-service)
- ❌ `POST /v1/query` — chưa implement
- ❌ `POST /v1/querybatch` — chưa implement  
- ❌ `determineversion` — chưa implement

→ **Giải pháp**: Hoàn thiện `gateway-service` OSV router với gRPC forwarding đến data-service.
