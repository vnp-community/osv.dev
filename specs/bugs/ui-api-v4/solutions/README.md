# V4 Solutions Directory

Thư mục này chứa các giải pháp thiết kế (`SOL-xxx`) để khắc phục triệt để các bugs trong `ui-api-v4` (đặc biệt là lỗi 404 và 405 phát sinh sau khi tích hợp vào Modular Monolith).

## Danh sách Giải pháp

- [SOL-001: Giải quyết bẫy Embedded Routers (Lỗi 404)](SOL-001-embedded-routers.md)
- [SOL-002: Giải quyết Lỗi Method Not Allowed (Lỗi 405)](SOL-002-method-mismatches.md)
- [SOL-003: Giải quyết Lỗi Auth MFA Method Mismatch](SOL-003-auth-mfa.md)

Các developer sẽ dựa vào các tài liệu này để refactor lại kiến trúc Embedded Services và Gateway Routing nhằm vượt qua 100% test cases của `test_all_endpoints.py`.
