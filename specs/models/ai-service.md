# Data Models — ai-service

> **Service**: `services/ai-service`  
> **Mô tả**: Làm giàu dữ liệu CVE bằng AI: tạo narrative tóm tắt, gán MITRE ATT&CK tags, phân loại severity bằng ML/LLM, tính EPSS scores và tạo vector embeddings.  
> **Storage**: MongoDB (enrichment results), Redis (EPSS cache, embeddings), PostgreSQL (vector via pgvector)  
> **Go packages**:
> - `services/ai-service/internal/domain/enrichment` — EnrichmentResult, EPSSSnapshot, MITRETag
> - `services/ai-service/internal/domain/epss` — EPSSScore (FIRST.org client)
> - `services/ai-service/internal/domain/triage` — TriageDecision, FindingInput, TriageResult
> - `services/ai-service/internal/domain/severity` — SeverityLevel, SeverityPrediction

---

## 1. EnrichmentResult

Aggregation entity tổng hợp toàn bộ kết quả AI enrichment cho một CVE.  
Package: `enrichment` (file: `domain/enrichment/entity.go`, package name: `service`)

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `cve_id` | string | No | CVE ID, ví dụ `CVE-2021-44228` |
| `enriched_at` | timestamp | No | Thời điểm enrichment hoàn thành |
| `provider` | string | No | AI provider thành công (openai, gemini, v.v.) |
| `model_version` | string | No | Phiên bản model được sử dụng |
| `summary_short` | string | Yes | Tóm tắt ngắn do AI tạo |
| `summary_long` | string | Yes | Mô tả chi tiết do AI tạo |
| `impact_analysis` | string | Yes | Phân tích tác động |
| `remediation_guide` | string | Yes | Hướng dẫn khắc phục do AI tạo |
| `attack_vector` | string | Yes | Vector tấn công mô tả |
| `epss` | *EPSSSnapshot | Yes | Snapshot điểm EPSS tại thời điểm enrichment |
| `mitre_tags` | []MITRETag | Yes | MITRE ATT&CK techniques liên quan |
| `severity_ml` | string | Yes | Severity do ML/LLM phân loại |
| `severity_confidence` | float64 | No | Độ tin cậy phân loại severity (0–1) |
| `exploit_available` | bool | No | Có public exploit |
| `has_embedding` | bool | No | Vector embedding đã được tạo |
| `embedding_model` | string | Yes | Model tạo embedding |

> **Repository interface**: `EnrichmentRepository`  
> - `Save(ctx, *EnrichmentResult) error`  
> - `FindByCVEID(ctx, id string) (*EnrichmentResult, error)`  
> - `FindBatch(ctx, []string) ([]*EnrichmentResult, error)`  
> - `ListUnenriched(ctx, limit int) ([]string, error)`

---

## 2. EPSSSnapshot

Điểm EPSS tại một thời điểm cụ thể (embedded trong EnrichmentResult).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `score` | float64 | No | EPSS probability 0–1 (xác suất khai thác trong 30 ngày tới) |
| `percentile` | float64 | No | Vị trí phần trăm so với tất cả CVEs |
| `fetched_at` | timestamp | No | Thời điểm lấy dữ liệu |

---

## 3. EPSSScore

Điểm EPSS lấy từ FIRST.org API (standalone cache entity).  
Package: `epss` (file: `domain/epss/client.go`)

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `cve_id` | string | No | CVE ID |
| `score` | float64 | No | EPSS probability 0–1 |
| `percentile` | float64 | No | Percentile 0–100 |
| `date` | timestamp | No | Ngày lấy dữ liệu |

> **Cache**: In-memory `sync.Map` với TTL 24 giờ. Batch fetch tối đa 100 CVE/request theo giới hạn FIRST.org API.

---

## 4. MITRETag

Ánh xạ CVE với MITRE ATT&CK technique (embedded trong EnrichmentResult).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `technique_id` | string | No | ATT&CK technique ID, ví dụ `T1190` |
| `technique_name` | string | No | Tên technique |
| `tactic_id` | string | No | Tactic ID, ví dụ `TA0001` |
| `confidence` | float64 | No | Độ tin cậy mapping (0–1) |

---

## 5. SeverityLevel & SeverityPrediction

Phân loại severity bằng CVSS ưu tiên, fallback sang LLM.  
Package: `severity` (file: `domain/severity/`)

**SeverityLevel** (enum):

| Giá trị | Mô tả |
|---------|-------|
| `CRITICAL` | CVSS ≥ 9.0 |
| `HIGH` | CVSS 7.0–8.9 |
| `MEDIUM` | CVSS 4.0–6.9 |
| `LOW` | CVSS > 0 và < 4.0 |
| `INFO` | CVSS = 0 |

**CVSSSeverity** (input):

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `score` | float64 | CVSS score |
| `type` | string | `CVSS_V3` \| `CVSS_V2` |

**SeverityPrediction** (output):

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `severity` | SeverityLevel | Kết quả phân loại |
| `confidence` | float32 | 1.0 = CVSS_V3, 0.9 = CVSS_V2, 0.7 = LLM, 0.0 = default fallback |
| `reasoning` | string | Giải thích lý do |
| `source` | string | `cvss_v3` \| `cvss_v2` \| `llm` \| `default` |

---

## 6. FindingInput & TriageResult

AI triage recommendation cho một finding.  
Package: `triage` (file: `domain/triage/service.go`)

**TriageDecision** (enum):

| Giá trị | Mô tả |
|---------|-------|
| `Confirmed` | Lỗ hổng được xác nhận là thực tế |
| `FalsePositive` | False alarm từ tool |
| `NotAffected` | Không bị ảnh hưởng |
| `NeedsReview` | Cần xem xét thêm (default) |

**FindingInput** (input cho triage):

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `title` | string | No | Tiêu đề finding |
| `description` | string | Yes | Mô tả chi tiết |
| `mitigation` | string | Yes | Hướng dẫn khắc phục |
| `cve` | string | Yes | CVE ID |
| `cvss_v3_score` | *float64 | Yes | CVSS v3 score |
| `component_name` | string | Yes | Tên component |
| `component_version` | string | Yes | Phiên bản component |
| `environment` | string | Yes | `production` \| `staging` \| `dev` |
| `asset_criticality` | string | Yes | `critical` \| `high` \| `medium` \| `low` |

**TriageResult** (output từ LLM):

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `decision` | TriageDecision | Quyết định triage |
| `confidence` | float32 | Độ tin cậy 0.0–1.0 |
| `reasoning` | string | Lý do quyết định |
| `suggestion` | string | Hành động đề xuất tiếp theo |
| `ai_cost` | float64 | Chi phí API LLM |

---

## 7. EmbeddingService

Tạo và cache vector embeddings cho vulnerabilities.  
Package: `enrichment` (file: `domain/enrichment/embedding_service.go`)

| Thông số | Giá trị |
|---------|---------|
| Cache backend | Redis |
| Cache key | `osv:embed:{vulnID}` |
| Cache TTL | 7 ngày |
| Max text length | 8,000 ký tự |
| Vector format | `[]float32` (binary little-endian encoding) |

---

## 8. Relationships

```
CVE (data-service) ──────── EnrichmentResult (1:1, theo cve_id)
EnrichmentResult ─────────── EPSSSnapshot (embedded)
EnrichmentResult ─────────── MITRETag (1:N, embedded array)
EPSSScore ────────────────── (standalone, cached từ FIRST.org API)
FindingInput → TriageResult (1:1, AI triage service)
SeverityPrediction ──────── EnrichmentResult.severity_ml (1:1)
```

---

## 9. AI Enrichment Pipeline

```
CVE ID
  │
  ├─→ EPSS Client (FIRST.org API) → EPSSSnapshot
  ├─→ AI Provider (OpenAI/Gemini) → narrative fields
  ├─→ MITRE Tagger → []MITRETag
  ├─→ Severity Classifier → SeverityPrediction (CVSS first → LLM fallback)
  ├─→ Exploit Detector → exploit_available
  └─→ Embedding Generator → []float32 → Redis (osv:embed:{id})
        │
        └─→ EnrichmentResult (aggregated)
              │
              └─→ MongoDB enrichment collection
```
