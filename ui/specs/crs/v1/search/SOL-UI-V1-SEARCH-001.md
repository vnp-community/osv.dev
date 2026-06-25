# SOL-UI-V1-SEARCH-001 — Topbar Global Search Integration

## 1. Context
Hiện tại ô Search Box trên `Topbar.tsx` chỉ là một khung HTML tĩnh (mockup component). Người dùng gõ dữ liệu không hiển thị suggestion và nhấn Enter không có kết quả do thẻ `<input>` thiếu state management, event handlers, và chưa được kết nối API.

Đồng thời, hệ thống đang có sẵn file `GlobalSearch.tsx` đóng vai trò là một trang tìm kiếm độc lập (Universal Search / Command Palette), nhưng chưa được tích hợp vào `Topbar.tsx`.

## 2. Goals
- Làm cho thanh tìm kiếm ở Topbar hoạt động trơn tru.
- Cung cấp trải nghiệm UX liền mạch bằng cách hiển thị tìm kiếm dưới dạng Command Palette (Modal) hoặc điều hướng trực tiếp sang trang tìm kiếm tập trung.
- Kết nối dữ liệu thực tế (CVEs, Findings, Assets) vào bộ tìm kiếm.

## 3. Architecture & Approach
Sẽ áp dụng phương án **Command Palette Overlay (Modal)** để mang lại trải nghiệm tốt nhất (tương tự MacOS Spotlight).

### Các thay đổi kiến trúc:
1. **Refactor GlobalSearch:** 
   Chuyển `GlobalSearch.tsx` từ một view component (hiển thị chiếm toàn màn hình) thành một Modal/Dialog component. Bổ sung các props điều khiển trạng thái như `isOpen` và `onClose`.
2. **Quản lý State ở Topbar:**
   Thêm biến trạng thái `isSearchModalOpen` vào `Topbar.tsx`.
3. **Event Binding:**
   - Thay đổi thuộc tính của ô `<input>` trên Topbar thành `readOnly`.
   - Gắn sự kiện `onClick` vào ô tìm kiếm để mở Modal.
   - Lắng nghe sự kiện bàn phím (Global Keyboard Listener) để mở Modal khi nhấn tổ hợp phím `⌘K` (hoặc `Ctrl+K`).
4. **Data Fetching:**
   Trong `GlobalSearch.tsx`, thay thế bộ dữ liệu `ALL_RESULTS` (hiện đang fix cứng) bằng các React Query hooks thực tế (như `useCVESearch`, `useFindings`, `useAssets`) khi người dùng gõ từ khóa, kết hợp với debounce để tối ưu hiệu năng.

## 4. Dependencies
- React (`useState`, `useEffect`) để xử lý đóng/mở và debounce.
- `@tanstack/react-query` để lấy dữ liệu search realtime.
- Lớp UI Modal/Dialog (có sẵn trong design system hoặc tự xây dựng overlay css).
