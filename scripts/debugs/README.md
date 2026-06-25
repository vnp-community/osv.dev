# OSV Debug Scripts

Python scripts để kiểm tra kết quả fetch dữ liệu CVE trên OSV platform.

## Architecture được test

```
Script → Gateway (apps/osv port 8080)
              ↓
       Data-service (services/data-service port 8082)
              ↓
       ┌──────┴──────────────┐
    MongoDB              PostgreSQL
    (cvedb)              (osv)
    ├── cves             ├── kev_entries
    ├── capec            └── sync_jobs
    └── cwe
```

## Scripts

| Script | Mục đích | Thời gian |
|--------|---------|----------|
| `01_health_check.py` | Kiểm tra health stack (Gateway + Data + Identity) | ~3s |
| `02_verify_kev_data.py` | Verify KEV data từ CISA trong PostgreSQL | ~5s |
| `03_verify_cve_data.py` | Verify CVE data từ NVD/CIRCL trong MongoDB | ~5s |
| `04_verify_scheduler.py` | Trạng thái scheduler + trigger manual sync | ~5s |
| `05_verify_stores.py` | Query trực tiếp PostgreSQL/MongoDB/Redis/NATS qua SSH | ~15s |
| `06_full_pipeline_test.py` | **10 test cases end-to-end** (BUG-002 fix verification) | ~20s |
| `run_all.py` | Chạy tất cả scripts theo thứ tự | ~60s |

## Quick Start

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/scripts/debugs

# 1. Chạy toàn bộ pipeline test
python3 06_full_pipeline_test.py

# 2. Quick health check
python3 01_health_check.py

# 3. Verify KEV data (kết quả sync từ CISA)
python3 02_verify_kev_data.py

# 4. Verify CVE data (kết quả sync từ NVD)
python3 03_verify_cve_data.py --cve CVE-2021-44228

# 5. Query stores trực tiếp qua SSH
python3 05_verify_stores.py --store postgres
python3 05_verify_stores.py --store mongodb

# 6. Trigger manual sync
python3 04_verify_scheduler.py --trigger kev
python3 04_verify_scheduler.py --trigger NVD

# 7. Chạy tất cả
python3 run_all.py
python3 run_all.py --quick   # chỉ health + pipeline
```

## Environment Variables

```bash
export OSV_SERVER=172.20.2.48       # server IP (default)
export ADMIN_EMAIL=admin@...        # admin email
export ADMIN_PASSWORD=Admin@123!    # admin password
export REQUEST_TIMEOUT=15           # HTTP timeout (giây)
```

## Expected Results (sau khi BUG-002 fix)

| Check | Expected |
|-------|---------|
| Gateway health | `{"status": "ok", "service": "gateway-service"}` |
| Data-service health | `{"status": "ok", "service": "data-service", "fetchers": 9}` |
| Fetcher registry | 9 fetchers: NVD, CIRCL, CVE.ORG, EXPLOITDB, JVN, EPSS, CAPEC, CWE, NVD-CPE |
| KEV count (PostgreSQL) | ≥ 1622 entries |
| CVE count (MongoDB) | ≥ 1000 CVEs (sau NVD sync ~10-30 phút) |
| Log4Shell lookup | HTTP 200, severity=CRITICAL |

## Bug Fix Reference

Scripts này được tạo để verify kết quả fix của:
- [BUG-001](../../specs/bugs/f02-cve-data-aggregation/BUG-001-scheduler-not-wired.md) — Scheduler không được wire vào main.go
- [BUG-002](../../specs/bugs/f02-cve-data-aggregation/BUG-002-data-service-placeholder.md) — data-service dùng placeholder adapter
- [BUG-003](../../specs/bugs/f02-cve-data-aggregation/BUG-003-wrong-mongo-db-name.md) — MONGO_DB sai (osv → cvedb)
