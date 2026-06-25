# V4 API BUG TRACKING (`ui-api-v4`)

Thư mục này chứa danh sách các bug liên quan đến API Backend (Gateway và các Microservices) được phát hiện trong quá trình chạy automation tests (`test_all_endpoints.py`) trên môi trường dev (Deploy server `172.20.2.48`, proxy qua `c12.openledger.vn`).

## Test Results Summary
- **Total Endpoints Tested**: 120
- **Passed**: 95 (Đã hoạt động tốt)
- **Failed**: 25 (Lỗi `404 Not Found` và `405 Method Not Allowed`)

## Phân Tích Bug Hậu Deploy

| ID | Bug Name | Lỗi | Số Endpoints | Trạng Thái |
|---|---|---|---|---|
| BUG-001 | [Embedded Mock Routers](BUG-001-embedded-mock-routers.md) | `404 Not Found` do các service khi nhúng qua `cmd/server/embed.go` đều dùng `http.NewServeMux` ảo chỉ có `/health` mà không chạy router logic thật. | 17 | `TODO` |
| BUG-002 | [Method Not Allowed](BUG-002-method-not-allowed.md) | `405 Method Not Allowed` do HTTP Method định nghĩa ở Router upstream không khớp với Method mà Test Script mong đợi. | 6 | `TODO` |
| BUG-003 | [Auth MFA Mismatch](BUG-003-auth-mfa-method-mismatch.md) | `404 Not Found` do Gateway Forward Proxy không đổi Method (VD: Expect GET nhưng upstream là POST) hoặc thiếu khai báo mapping cho MFA confirm. | 2 | `TODO` |

## Giải Quyết
Các developer sẽ tiếp nhận xử lý theo thứ tự: ưu tiên sửa BUG-001 (Embedded Mock Routers) vì lỗi này ngăn chặn luồng hoạt động của hơn 17 endpoints thuộc nhiều services khác nhau, sau đó mới đến các bug mismatch method.
