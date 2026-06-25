# OSV.dev — Service Inventory

> **Mục tiêu**: Tài liệu mô tả chi tiết tất cả services từ `services/` (active) và `archive/` (deprecated/merged).
> Cơ sở để merge và tái cấu trúc thành ≤ 10 services cốt lõi với Clean Architecture.

---

## Thư mục này chứa

| File | Mô tả |
|------|-------|
| [01_active_services.md](./01_active_services.md) | Chi tiết 17 services đang active trong `services/` |
| [02_archive_services.md](./02_archive_services.md) | Chi tiết 45 services trong `archive/` (cũ/deprecated) |
| [03_shared_layer.md](./03_shared_layer.md) | Shared packages và proto definitions |
| [04_merge_proposal.md](./04_merge_proposal.md) | Đề xuất merge thành ≤ 10 services cốt lõi |

---

## Tổng quan hiện trạng

### Active Services (services/)
```
17 services | Go 1.26.3 | Clean Architecture (domain/usecase/infra/delivery)
```

| # | Service | Chức năng chính |
|---|---------|-----------------|
| 1 | auth-service | Xác thực, JWT, OAuth2, API Key |
| 2 | ai-service | AI enrichment, EPSS, MITRE tagging, severity |
| 3 | vulnerability-service | Quản lý CVE/vulnerability data (cve, kev, alias, taxonomy) |
| 4 | ingestion-service | Thu thập, sync dữ liệu từ NVD/OSV/GHSA |
| 5 | finding-service | Quản lý findings, audit, SLA |
| 6 | scan-service | Điều phối scan, agent, asset, schedule |
| 7 | schedule-service | Cron schedule, recurring scan triggers |
| 8 | product-service | Quản lý product, engagement, test |
| 9 | impact-service | Phân tích impact, version index |
| 10 | notification-service | Alert, webhook, rule, subscription |
| 11 | report-service | Tạo báo cáo (PDF/JSON/Excel) |
| 12 | search-service | Full-text search CVE |
| 13 | query-service | Query/filter vulnerabilities |
| 14 | integration-service | Tích hợp Jira |
| 15 | unified-gateway | API Gateway, auth proxy, BFF, rate-limit |
| 16 | dd-search | DefectDojo search adapter |
| 17 | shared | Shared libraries & proto definitions |

### Archive Services (archive/)
```
45 services — đã deprecated, merged hoặc refactored
```

> Xem chi tiết tại [02_archive_services.md](./02_archive_services.md)

---

## Tech Stack chung

- **Language**: Go 1.26.3
- **Architecture**: Clean Architecture (domain → usecase → infra/delivery)
- **Messaging**: NATS
- **Databases**: PostgreSQL (pgx/v5), MongoDB, Redis, Firestore
- **API**: gRPC + HTTP (chi router)
- **Observability**: OpenTelemetry, Zerolog
- **Proto**: buf + protobuf
