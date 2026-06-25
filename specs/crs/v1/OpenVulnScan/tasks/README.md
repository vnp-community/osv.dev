# OpenVulnScan — Tasks cho AI Execution

> **Phiên bản**: v1  
> **Ngày tạo**: 2026-06-16  
> **Cập nhật**: 2026-06-17  
> **Nguồn**: Được chia nhỏ từ Solutions (`../solutions/`)

---

## Cách Dùng

Mỗi file task trong thư mục này là một **đơn vị công việc độc lập** cho AI agent thực thi. Mỗi task:

1. **Self-contained**: Chứa đủ context để thực thi mà không cần đọc thêm file khác
2. **Verifiable**: Có acceptance criteria rõ ràng để kiểm tra kết quả
3. **Ordered**: Có dependency rõ ràng (task nào cần làm trước)
4. **Scoped**: Chỉ thực hiện một chức năng cụ thể

---

## Index Tasks Theo Service

### 🔐 AUTH SERVICE (identity-service) — CR-OVS-003

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-AUTH-001 | [TASK-AUTH-001-domain-entities.md](TASK-AUTH-001-domain-entities.md) | User/Session/APIKey domain entities | — | ✅ Completed |
| T-AUTH-002 | [TASK-AUTH-002-argon2-jwt.md](TASK-AUTH-002-argon2-jwt.md) | Argon2id hashing + RS256 JWT manager | T-AUTH-001 | ✅ Completed |
| T-AUTH-003 | TASK-AUTH-003 | Redis JTI cache cho ValidateToken | T-AUTH-001 | ✅ Completed |
| T-AUTH-004 | TASK-AUTH-004 | Register + Login use cases | T-AUTH-002 | ✅ Completed |
| T-AUTH-005 | TASK-AUTH-005 | RefreshToken + Logout (token family) | T-AUTH-004 | ✅ Completed |
| T-AUTH-006 | [TASK-AUTH-006-grpc-validate-token.md](TASK-AUTH-006-grpc-validate-token.md) | gRPC ValidateToken hot path | T-AUTH-003 | ✅ Completed |
| T-AUTH-007 | TASK-AUTH-007 | API Key (ovs_ prefix) CRUD + validate | T-AUTH-004 | ✅ Completed |
| T-AUTH-008 | TASK-AUTH-008 | TOTP MFA setup/confirm/disable | T-AUTH-004 | ✅ Completed |
| T-AUTH-009 | TASK-AUTH-009 | Google + GitHub OAuth2 | T-AUTH-004 | ✅ Completed |
| T-AUTH-010 | TASK-AUTH-010 | HTTP REST handlers + routes | T-AUTH-005,007,008,009 | ✅ Completed |

### 🔍 SCAN SERVICE — CR-OVS-001

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-SCAN-001 | [TASK-SCAN-001-nmap-scanner.md](TASK-SCAN-001-nmap-scanner.md) | Nmap subprocess wrapper + XML parser | — | ✅ Completed |
| T-SCAN-002 | TASK-SCAN-002 | OWASP ZAP API client (spider + active) | — | ✅ Completed |
| T-SCAN-003 | TASK-SCAN-003 | ExecuteScan use case + cancel flow | T-SCAN-001,002 | ✅ Completed |
| T-SCAN-004 | TASK-SCAN-004 | SSE progress streaming endpoint | T-SCAN-003 | ✅ Completed |
| T-SCAN-005 | TASK-SCAN-005 | Agent report endpoint + Python script | — | ✅ Completed |

### 🐛 FINDING SERVICE — CR-OVS-002

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-FIND-001 | [TASK-FIND-001-state-machine.md](TASK-FIND-001-state-machine.md) | Finding state machine (Close/Reopen/FP) | — | ✅ Completed |
| T-FIND-002 | [TASK-FIND-002-dedup-hash.md](TASK-FIND-002-dedup-hash.md) | SHA-256 hash deduplication | T-FIND-001 | ✅ Completed |
| T-FIND-003 | TASK-FIND-003 | SLA configuration + breach check cron | T-FIND-001 | ✅ Completed |
| T-FIND-004 | TASK-FIND-004 | Audit trail logging | T-FIND-001 | ✅ Completed |
| T-FIND-005 | TASK-FIND-005 | gRPC FindingService server | T-FIND-001,002,003,004 | ✅ Completed |
| T-FIND-006 | TASK-FIND-006 | NATS scan.scan.completed consumer | T-FIND-002 | ✅ Completed |

### 📦 PRODUCT SERVICE — CR-OVS-004

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-PROD-001 | TASK-PROD-001 | ProductType/Product/Engagement/Test entities | — | ✅ Completed |
| T-PROD-002 | TASK-PROD-002 | CI/CD Orchestrator use case | T-PROD-001 | ✅ Completed |
| T-PROD-003 | TASK-PROD-003 | REST API + orchestrate endpoint | T-PROD-001,002 | ✅ Completed |

### 🤖 AI SERVICE — CR-OVS-005

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-AI-001 | [TASK-AI-001-provider-chain.md](TASK-AI-001-provider-chain.md) | Ollama + OpenAI providers + failover chain | — | ✅ Completed |
| T-AI-002 | TASK-AI-002 | pgvector storage + Redis embedding cache | — | ✅ Completed |
| T-AI-003 | TASK-AI-003 | CVSS-first severity classification + LLM | T-AI-001 | ✅ Completed |
| T-AI-004 | TASK-AI-004 | EPSS FIRST.org API + midnight cache | — | ✅ Completed |
| T-AI-005 | TASK-AI-005 | AI triage finding use case + LLM prompt | T-AI-001 | ✅ Completed |
| T-AI-006 | TASK-AI-006 | EnrichCVE parallel orchestration + NATS | T-AI-001,002,003,004 | ✅ Completed |

### 📊 REPORT SERVICE — CR-OVS-006

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-RPT-001 | [TASK-RPT-001-html-formatter.md](TASK-RPT-001-html-formatter.md) | Bootstrap HTML formatter + template | — | ✅ Completed |
| T-RPT-002 | TASK-RPT-002 | PDF via chromedp headless Chrome | T-RPT-001 | ✅ Completed |
| T-RPT-003 | TASK-RPT-003 | Excel (DefectDojo) + CSV formatters | — | ✅ Completed |
| T-RPT-004 | TASK-RPT-004 | MinIO/S3 storage + presigned URLs | — | ✅ Completed |
| T-RPT-005 | TASK-RPT-005 | GenerateReport async use case | T-RPT-001,002,003,004 | ✅ Completed |
| T-RPT-006 | TASK-RPT-006 | REST API + download + CI/CD exit code | T-RPT-005 | ✅ Completed |

### 🖥️ ASSET SERVICE — CR-OVS-007

| Task | File | Mô tả | Phụ thuộc | Trạng thái |
|------|------|-------|-----------|-----------|
| T-ASSET-001 | [TASK-ASSET-001-upsert-asset.md](TASK-ASSET-001-upsert-asset.md) | Asset entity + UpsertAsset use case | — | ✅ Completed |
| T-ASSET-002 | TASK-ASSET-002 | Asset tagging + risk score computation | T-ASSET-001 | ✅ Completed |
| T-ASSET-003 | TASK-ASSET-003 | REST API (list, tags, history, findings) | T-ASSET-001,002 | ✅ Completed |
| T-ASSET-004 | [TASK-ASSET-004-scheduled-scans.md](TASK-ASSET-004-scheduled-scans.md) | ScheduledScan entity + scheduler goroutine | — | ✅ Completed |

---

## 🎉 Tổng Tiến Độ

**40 / 40 tasks hoàn thành (100%)**

```
✅ ALL COMPLETED (40/40)
  AUTH:  T-AUTH-001..010  (10/10)
  SCAN:  T-SCAN-001..005  (5/5)
  FIND:  T-FIND-001..006  (6/6)
  PROD:  T-PROD-001..003  (3/3)
  AI:    T-AI-001..006    (6/6)
  RPT:   T-RPT-001..006   (6/6)
  ASSET: T-ASSET-001..004 (4/4)
```

---

## Thứ Tự Thực Thi Đề Xuất

```
Sprint 1: T-AUTH-001 → T-AUTH-002 → T-AUTH-003 → T-AUTH-004 → T-AUTH-006  ✅
Sprint 2: T-AUTH-005 → T-AUTH-007 → T-SCAN-001 → T-SCAN-002 → T-SCAN-003  ✅
Sprint 3: T-AUTH-008 → T-AUTH-009 → T-AUTH-010 → T-FIND-001 → T-FIND-002  ✅
Sprint 4: T-FIND-003 → T-FIND-004 → T-FIND-005 → T-FIND-006 → T-SCAN-004  ✅
Sprint 5: T-PROD-001 → T-PROD-002 → T-PROD-003 → T-SCAN-005               ✅
Sprint 6: T-RPT-001 → T-RPT-002 → T-RPT-003 → T-RPT-004 → T-RPT-005 → T-RPT-006  ✅
Sprint 7: T-AI-001 → T-AI-002 → T-AI-003 → T-AI-004 → T-AI-005 → T-AI-006  ✅
Sprint 8: T-ASSET-001 → T-ASSET-002 → T-ASSET-003 → T-ASSET-004           ✅
```

---

## Trạng Thái Thực Thi

| Task | Trạng thái | Ngày hoàn thành | File được tạo |
|------|-----------|-----------------|---------------|
| T-AUTH-001 | ✅ | 2026-06-16 | `identity-service/internal/domain/entity/` |
| T-AUTH-002 | ✅ | 2026-06-16 | `identity-service/internal/crypto/` |
| T-AUTH-003 | ✅ | 2026-06-17 | `identity-service/internal/cache/redis/jti_cache.go` |
| T-AUTH-004 | ✅ | 2026-06-17 | `identity-service/internal/usecase/register/` + `login/` |
| T-AUTH-005 | ✅ | 2026-06-17 | `identity-service/internal/usecase/refresh/` + `logout/` |
| T-AUTH-006 | ✅ | 2026-06-16 | `identity-service/internal/delivery/grpc/` |
| T-AUTH-007 | ✅ | 2026-06-17 | `identity-service/internal/usecase/apikey/usecase.go` |
| T-AUTH-008 | ✅ | 2026-06-17 | `identity-service/internal/usecase/totp/usecase.go` |
| T-AUTH-009 | ✅ | 2026-06-17 | `identity-service/internal/usecase/oauth2/usecase.go` |
| T-AUTH-010 | ✅ | 2026-06-17 | `identity-service/internal/delivery/http/handlers.go` |
| T-SCAN-001 | ✅ | 2026-06-16 | `scan-service/internal/scanner/nmap/` |
| T-SCAN-002 | ✅ | 2026-06-17 | `scan-service/internal/scanner/zap/scanner.go` |
| T-SCAN-003 | ✅ | 2026-06-17 | `scan-service/internal/usecase/execute_scan.go` |
| T-SCAN-004 | ✅ | 2026-06-17 | `scan-service/internal/delivery/sse/hub.go` |
| T-SCAN-005 | ✅ | 2026-06-17 | `scan-service/agent/agent.py` |
| T-FIND-001 | ✅ | 2026-06-16 | `finding-service/internal/domain/finding/entity.go` |
| T-FIND-002 | ✅ | 2026-06-16 | `finding-service/internal/usecase/dedup/service.go` |
| T-FIND-003 | ✅ | 2026-06-17 | `finding-service/internal/domain/sla/service.go` |
| T-FIND-004 | ✅ | 2026-06-17 | `finding-service/internal/domain/audit/logger.go` |
| T-FIND-005 | ✅ | 2026-06-17 | `finding-service/internal/delivery/grpc/server.go` |
| T-FIND-006 | ✅ | 2026-06-17 | `finding-service/internal/delivery/nats/consumer.go` |
| T-PROD-001 | ✅ | 2026-06-17 | `product-service/internal/domain/entity/entities.go` |
| T-PROD-002 | ✅ | 2026-06-17 | `product-service/internal/usecase/orchestrator/cicd.go` |
| T-PROD-003 | ✅ | 2026-06-17 | `product-service/internal/delivery/http/handlers.go` |
| T-AI-001 | ✅ | 2026-06-16 | `ai-service/internal/provider/` |
| T-AI-002 | ✅ | 2026-06-17 | `ai-service/internal/domain/embedding/service.go` |
| T-AI-003 | ✅ | 2026-06-17 | `ai-service/internal/domain/severity/classifier.go` |
| T-AI-004 | ✅ | 2026-06-17 | `ai-service/internal/domain/epss/client.go` |
| T-AI-005 | ✅ | 2026-06-17 | `ai-service/internal/domain/triage/service.go` |
| T-AI-006 | ✅ | 2026-06-17 | `ai-service/internal/usecase/enrich/usecase.go` |
| T-RPT-001 | ✅ | 2026-06-16 | `report-service/internal/formatters/html.go` |
| T-RPT-002 | ✅ | 2026-06-17 | `report-service/internal/formatters/pdf.go` |
| T-RPT-003 | ✅ | 2026-06-17 | `report-service/internal/formatters/csv_json_excel.go` |
| T-RPT-004 | ✅ | 2026-06-17 | `report-service/internal/storage/minio.go` |
| T-RPT-005 | ✅ | 2026-06-17 | `report-service/internal/usecase/generate.go` |
| T-RPT-006 | ✅ | 2026-06-17 | `report-service/internal/delivery/http/handlers.go` |
| T-ASSET-001 | ✅ | 2026-06-16 | `asset-service/internal/usecase/asset/upsert.go` |
| T-ASSET-002 | ✅ | 2026-06-17 | `asset-service/internal/usecase/asset/tagging_risk.go` |
| T-ASSET-003 | ✅ | 2026-06-17 | `asset-service/internal/delivery/http/handlers.go` |
| T-ASSET-004 | ✅ | 2026-06-16 | `scan-service/internal/domain/schedule/entity.go` |

---

## Convention Cho Mỗi Task File

```markdown
## Context
## Goal  
## Target Files (create/modify)
## Implementation
## Verification
## Dependencies
```
