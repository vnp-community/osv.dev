# OSV.dev — Specs Index

> **Project:** Open Source Vulnerabilities (OSV.dev)  
> **Repository:** https://github.com/google/osv.dev  
> **Last Updated:** 2026-06-03

---

## Danh Sách Tài Liệu

| # | Tài liệu | Mô tả |
|---|----------|-------|
| 01 | [Architecture Document](./01-architecture.md) | Kiến trúc tổng thể hệ thống: data flow, các services, hạ tầng GCP, data sources |
| 02 | [Technical Design Document](./02-technical-design.md) | Chi tiết kỹ thuật từng component: algorithms, schemas, error handling, testing |
| 03 | [Deployment Model](./03-deployment-model.md) | Mô hình triển khai, quản lý nguồn CVE, phân loại CVE, quản lý kết nối, rollout plan |
| — | **[Develop/](./develop/README.md)** | **Đề xuất phát triển codebase: tổ chức lại, deprecation, roadmap, migration** |

---

## Tóm Tắt Hệ Thống

**OSV.dev** là nền tảng cơ sở dữ liệu lỗ hổng bảo mật mã nguồn mở (Open Source Vulnerabilities) do Google phát triển. Hệ thống:

- **Tổng hợp** dữ liệu lỗ hổng từ **30+ nguồn** (GitHub, NVD, OSS-Fuzz, Debian, Ubuntu, Red Hat, ...)
- **Chuẩn hóa** theo OSV Schema (v1.7.5)
- **Phân phối** qua REST/gRPC API công khai tại `api.osv.dev`
- **Hiển thị** qua web interface tại `osv.dev`
- **Export** data dumps tới `gs://osv-vulnerabilities/`

### Các Components Chính

```
osv/              → Core Python library
gcp/api/          → gRPC + REST API Server
gcp/workers/      → Importer, Worker, Alias Workers
gcp/indexer/      → Version Indexer (Go)
gcp/website/      → Web Frontend (Flask + Hugo)
vulnfeeds/        → CVE/Alpine/Debian converters (Go)
go/               → Shared Go library
```

### Công Nghệ Sử Dụng

| Layer | Technology |
|-------|-----------|
| Languages | Python 3.13, Go |
| Cloud | Google Cloud Platform (GCP) |
| Database | Cloud Datastore (Firestore NDB) |
| Storage | Cloud Storage (GCS) |
| Messaging | Cloud Pub/Sub |
| API Framework | gRPC + Protocol Buffers |
| Web Framework | Flask (Python) |
| Frontend | Hugo + modern JS |
| Infrastructure | Terraform, Cloud Build, Cloud Deploy |
| Caching | Redis (Cloud Memorystore) |
