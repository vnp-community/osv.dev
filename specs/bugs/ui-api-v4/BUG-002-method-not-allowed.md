# BUG-002: Các Endpoints Lỗi Method Not Allowed (405)

## Overview
Kết quả test API từ `test_all_endpoints.py` cho thấy có 6 endpoints nhận được phản hồi `405 Method Not Allowed`.
Trạng thái HTTP 405 có nghĩa là: Gateway đã forward chính xác request đến upstream service tương ứng (Route Path có tồn tại trên upstream service router), nhưng **HTTP Method mà test script gửi đi không được hỗ trợ** trên endpoint đó. 

## Các API Bị Ảnh Hưởng (405)

- `POST /api/v1/scans/import`: Có route này nhưng không cho phép POST, hoặc script expect POST nhưng API expect method khác (vd: PUT).
- `GET /api/v1/findings/{id}/notes`: Route tồn tại nhưng không hỗ trợ GET (có thể chỉ hỗ trợ POST hoặc không có logic cho GET notes list).
- `PUT /api/v1/sla/config`: Có thể API expect POST hoặc PATCH.
- `PATCH /api/v1/products/{id}`: Có thể API expect PUT.
- `GET /api/v1/webhooks/stats`: Route tồn tại nhưng bị từ chối Method GET.
- `GET /api/v1/admin/users/{id}`: Tồn tại route nhưng không hỗ trợ GET.

## Phân tích & Giải pháp
Cần kiểm tra lại mã nguồn của các router trong từng service (Scan, Finding, Asset, Gateway/Admin, Webhook) và đối chiếu với test script `test_all_endpoints.py`:
1. Nếu API backend đang định nghĩa sai method (ví dụ update product lại dùng PUT thay vì PATCH như specs API), cần sửa router backend.
2. Nếu Test Script sử dụng sai HTTP method so với thiết kế chuẩn của API backend, cần cập nhật Test Script.
