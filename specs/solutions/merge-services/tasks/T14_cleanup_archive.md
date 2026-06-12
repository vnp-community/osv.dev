# T14 — Cleanup: Delete Archive & Old Services

**Phase**: 14 (FINAL)
**Depends on**: T13
**Estimated effort**: 15 minutes

> ⚠️ **KHÔNG THỰC HIỆN BƯỚC NÀY cho đến khi TẤT CẢ điều kiện prerequisite hoàn thành.**
> Đây là bước không thể hoàn tác. Sau khi xoá, không thể khôi phục nếu không có git.

---

## Mục tiêu

1. Xoá thư mục `archive/` (toàn bộ 45+ services đã deprecated)
2. Xoá các services cũ còn sót lại trong `services/` (nếu có)
3. Commit final cleanup

---

## Pre-conditions Checklist (PHẢI PASS trước khi xoá)

Chạy script kiểm tra sau:

```bash
#!/bin/bash
echo "=== PRE-CLEANUP VERIFICATION ==="
PASS=1
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
ARCHIVE="/Users/binhnt/Lab/sec/cve/osv.dev/archive"

# 1. Kiểm tra 8 services tồn tại
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  if [ -d "$SVC_ROOT/$svc" ]; then
    echo "✓ $svc exists"
  else
    echo "✗ $svc MISSING"
    PASS=0
  fi
done

# 2. Kiểm tra tất cả build pass
for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  cd "$SVC_ROOT/$svc" 2>/dev/null || { echo "✗ Cannot cd to $svc"; PASS=0; continue; }
  if go build ./... 2>/dev/null; then
    echo "✓ $svc builds OK"
  else
    echo "✗ $svc BUILD FAILED"
    PASS=0
  fi
done

# 3. Kiểm tra services cũ đã được xoá
OLD_SVCS=(
  auth-service vulnerability-service ingestion-service
  query-service dd-search schedule-service product-service
  report-service integration-service unified-gateway impact-service
)
for old in "${OLD_SVCS[@]}"; do
  if [ -d "$SVC_ROOT/$old" ]; then
    echo "⚠ $old vẫn còn trong services/ — nên xoá trước"
  fi
done

# 4. Kiểm tra git clean
cd /Users/binhnt/Lab/sec/cve/osv.dev
git status --porcelain | head -5

# 5. Kết quả
echo ""
if [ $PASS -eq 1 ]; then
  echo "✅ ALL CHECKS PASSED — Safe to proceed with cleanup"
else
  echo "❌ SOME CHECKS FAILED — DO NOT DELETE ARCHIVE YET"
  exit 1
fi
```

---

## Tác vụ chi tiết

### Bước 1: Git commit trước khi xoá

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
git add -A
git commit -m "feat: complete 17→8 service merge - all builds pass"
git tag merge-complete
```

### Bước 2: Xoá services cũ còn sót trong services/

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

# Các services đã được merge vào core services
OLD_SERVICES=(
  auth-service          # → identity-service
  vulnerability-service # → data-service
  ingestion-service     # → data-service
  query-service         # → search-service
  dd-search             # → search-service
  schedule-service      # → scan-service
  impact-service        # → absorbed by data-service + scan-service
  product-service       # → finding-service
  report-service        # → finding-service
  integration-service   # → notification-service
  unified-gateway       # → gateway-service
)

for old in "${OLD_SERVICES[@]}"; do
  if [ -d "$SVC_ROOT/$old" ]; then
    rm -rf "$SVC_ROOT/$old"
    echo "Deleted: services/$old"
  else
    echo "Already gone: services/$old"
  fi
done
```

### Bước 3: Xoá toàn bộ archive/

```bash
ARCHIVE="/Users/binhnt/Lab/sec/cve/osv.dev/archive"

# Đếm trước khi xoá
echo "Archive contents to delete:"
ls "$ARCHIVE/" | wc -l
echo "services/subdirs"
du -sh "$ARCHIVE"

# XOÁ
rm -rf "$ARCHIVE"
echo "✅ Deleted: archive/ ($(du -sh "$ARCHIVE" 2>/dev/null || echo 'gone'))"
```

### Bước 4: Kiểm tra cấu trúc sau cleanup

```bash
echo "=== services/ AFTER CLEANUP ==="
ls -la /Users/binhnt/Lab/sec/cve/osv.dev/services/

# Phải chỉ thấy:
# identity-service/
# data-service/
# search-service/
# scan-service/
# finding-service/
# ai-service/
# notification-service/
# gateway-service/
# shared/

echo "=== archive/ ==="
ls /Users/binhnt/Lab/sec/cve/osv.dev/archive/ 2>/dev/null || echo "(deleted ✓)"
```

### Bước 5: Final build verification

```bash
FAIL=0
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  cd "$SVC_ROOT/$svc"
  if go build ./...; then
    echo "✓ $svc OK"
  else
    echo "✗ $svc FAILED"
    FAIL=1
  fi
done

[ $FAIL -eq 0 ] && echo "🎉 ALL 8 SERVICES BUILD SUCCESSFULLY AFTER CLEANUP"
```

### Bước 6: Final git commit

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
git add -A
git commit -m "chore: cleanup - delete archive/ and old services

BREAKING CHANGE: Removed 11 old services and 45 archive services.

Before: 17 active services + 45 archive = 62 total
After:  8 core services + shared layer = 9 total

Deleted services:
  services/: auth-service, vulnerability-service, ingestion-service,
              query-service, dd-search, schedule-service, impact-service,
              product-service, report-service, integration-service,
              unified-gateway
  archive/: (all 45 services)

New core services:
  identity-service, data-service, search-service, scan-service,
  finding-service, ai-service, notification-service, gateway-service"

git tag v1.0.0-merged
echo "🏁 Merge project complete!"
```

---

## Kết quả cuối cùng

```
Trước:                          Sau:
────────────────────────        ────────────────────────
services/  (17 services)        services/  (8 services)
archive/   (45 services)        shared/    (unchanged)
────────────────────────        ────────────────────────
62 modules total                9 modules total
```

---

## Điều kiện hoàn thành CUỐI CÙNG

- [ ] Pre-condition script pass (tất cả 8 services build OK)
- [ ] Git tag `merge-complete` tạo thành công trước khi xoá
- [ ] `services/` chỉ còn 8 core services + `shared/`
- [ ] `archive/` KHÔNG còn tồn tại
- [ ] Tất cả 8 services build pass sau cleanup
- [ ] Git commit final với message mô tả đầy đủ
- [ ] Git tag `v1.0.0-merged`

---

## Rollback (nếu cần)

```bash
# Nếu có vấn đề sau cleanup, rollback về snapshot:
git checkout pre-merge-snapshot

# Hoặc chỉ restore archive từ git:
git checkout pre-merge-snapshot -- archive/
```
