# TASK-003: Kiểm tra và Refactor Mock Embedded Servers

## Mục tiêu
Loại bỏ lỗi 404 do Orchestrator đang nhúng các Mock Router đối với Scan, Finding, Asset, SLA, Audit.

## Các Endpoints Bị Ảnh Hưởng
Tất cả các API của các service chưa được wire đầy đủ trong `embedded.go`.

## Hướng dẫn thực thi

1. **Rà soát `apps/osv/internal/config/wire.go`**:
   - Kiểm tra xem orchestration gọi hàm `WireEmbedded` của các service (Scan, Finding, Asset, SLA) đã chuẩn chưa.

2. **Kiểm tra file `embedded.go` của từng Service**:
   - Service: `scan-service`, `finding-service`, `asset-service`, `product-service`, `sla-service`.
   - Xem hàm `WireEmbedded` của các service này có thực sự gọi `mux.Handle("/", router)` với router thật (chi.Router) không, hay chỉ trả về dummy `http.NewServeMux` có `/health`.

3. **Cập nhật Logic Nhúng**:
   - Chuyển tất cả Mock Router thành Real Router. Nếu thiếu dependencies (DB/Cache), hãy inject đầy đủ thông qua `pool *pgxpool.Pool` có sẵn trong hàm `WireEmbedded`.

## Acceptance Criteria (AC)
- [x] Không còn Mock Router trong `embedded.go` của Scan, Finding, Asset, SLA.
- [x] Compile thành công.
