# SOL-UI-V1-EXPORT-001 — Backend-Side PDF Export (Server Generation)

## 1. Context
Chức năng "Export PDF" trên màn hình Dashboard hiện đang sử dụng giải pháp tạm thời ở phía Frontend (Client-side) thông qua `window.print()` hoặc thư viện JS nội bộ. Điều này bộc lộ các yếu điểm như:
- Format file không đồng nhất giữa các trình duyệt.
- Thiếu các thành phần chuẩn hóa của báo cáo doanh nghiệp (Header, Footer, Watermark, chữ ký số).
- Việc xử lý tốn tài nguyên máy khách nếu lượng biểu đồ lớn.

## 2. Goals
- Dịch chuyển tiến trình khởi tạo PDF sang phía Backend (Server-side Generation).
- Chuẩn hóa layout báo cáo theo định dạng A4 chuyên nghiệp.
- Cho phép xuất báo cáo theo chu kỳ (Period) đang được chọn trên giao diện.

## 3. Architecture & Approach
Sử dụng API `reportApi.download(id, format)` đã được cấu trúc trong dự án.

### Quy trình hoạt động:
1. Người dùng bấm "Export PDF".
2. Frontend gọi API `dashboardApi.export(period)` (hoặc `reportApi.generate({ type: 'dashboard', period })`).
3. Backend sử dụng Puppeteer hoặc Go-PDF để render server-side dựa vào data hiện tại.
4. Backend trả về một File Stream (Blob) với định dạng `application/pdf`.
5. Frontend hứng luồng Blob này, sinh ra object URL và kích hoạt sự kiện tự động `download` trên trình duyệt.

## 4. Dependencies
- Endpoint Backend (Ví dụ: `GET /api/v1/dashboard/export?period=30d`).
- `FileSaver.js` hoặc cơ chế tạo thẻ `<a>` ẩn để trigger download Blob.
