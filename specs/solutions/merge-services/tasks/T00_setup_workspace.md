# T00 — Setup Workspace

**Phase**: 0 — Prerequisites
**Depends on**: nothing
**Estimated effort**: 15 minutes

---

## Mục tiêu

Chuẩn bị workspace sạch trước khi bắt đầu merge. Tạo cấu trúc thư mục cho 8 core services mới trong `services/`.

---

## Bối cảnh

- **Root**: `/Users/binhnt/Lab/sec/cve/osv.dev/`
- **Services dir**: `services/` — nơi chứa services hiện tại và services mới sau merge
- **Archive dir**: `archive/` — sẽ bị xoá ở T14 sau khi merge xong
- **Spec dir**: `specs/services/` — kiến trúc chi tiết từng service

---

## Tác vụ chi tiết

### 1. Backup trạng thái hiện tại (git)

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
git add -A
git commit -m "chore: snapshot before merge - $(date +%Y%m%d)"
git tag pre-merge-snapshot
```

### 2. Tạo thư mục skeleton cho 8 core services

```bash
SERVICES_DIR="/Users/binhnt/Lab/sec/cve/osv.dev/services"

for svc in identity-service data-service search-service scan-service finding-service ai-service notification-service gateway-service; do
  mkdir -p "$SERVICES_DIR/$svc/cmd/server"
  mkdir -p "$SERVICES_DIR/$svc/internal/domain"
  mkdir -p "$SERVICES_DIR/$svc/internal/usecase"
  mkdir -p "$SERVICES_DIR/$svc/internal/delivery/grpc"
  mkdir -p "$SERVICES_DIR/$svc/internal/delivery/http"
  mkdir -p "$SERVICES_DIR/$svc/internal/infra"
  mkdir -p "$SERVICES_DIR/$svc/migrations"
  echo "Created $svc skeleton"
done
```

### 3. Tạo .gitkeep cho thư mục rỗng

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services -type d -empty -exec touch {}/.gitkeep \;
```

### 4. Kiểm tra cấu trúc

```bash
ls -la /Users/binhnt/Lab/sec/cve/osv.dev/services/
# Phải thấy: identity-service/ data-service/ search-service/ scan-service/
#            finding-service/ ai-service/ notification-service/ gateway-service/
#            + các services cũ (chưa xoá)
```

---

## Điều kiện hoàn thành

- [ ] Git commit snapshot thành công
- [ ] 8 thư mục skeleton được tạo
- [ ] Mỗi thư mục có đủ `cmd/server/`, `internal/`, `migrations/`
- [ ] `git status` sạch

---

## Lưu ý

> **KHÔNG** xoá bất kỳ service cũ nào ở bước này. Chỉ tạo thư mục mới. Các service cũ sẽ được xoá dần khi merge hoàn tất từng cái.
