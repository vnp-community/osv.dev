# TASK-UI-V1-SEARCH-001 — Implement Topbar Search

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-V1-SEARCH-001 |
| **Module** | `ui/src/app/components/` |
| **Solution Ref** | [SOL-UI-V1-SEARCH-001](./SOL-UI-V1-SEARCH-001.md) |
| **Priority** | 🔴 P1 |
| **Estimated** | 2h |
| **Status** | 🕒 Pending |

## Context
Thực thi giải pháp đã được thiết kế tại `SOL-UI-V1-SEARCH-001` nhằm biến thanh tìm kiếm tĩnh (mockup) trên Topbar thành một Command Palette (Universal Search) hoạt động trơn tru.

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `ui/src/app/components/Topbar.tsx` |
| MODIFY | `ui/src/app/components/GlobalSearch.tsx` |
| MODIFY | `ui/src/app/App.tsx` |

## Implementation Steps

### Step 1: Refactor GlobalSearch.tsx
- Cập nhật hàm `GlobalSearch` để nhận thêm 2 props: `isOpen: boolean` và `onClose: () => void`.
- Bọc toàn bộ component hiện tại trong một lớp `div` overlay (background có độ trong suốt và `z-index` cao) kết hợp với layout định dạng Modal (căn giữa màn hình).
- Lắng nghe sự kiện `Escape` để gọi hàm `onClose()`.
- Xóa bỏ việc render `GlobalSearch` như một view/route riêng biệt trong tương lai.

### Step 2: Cập nhật Topbar.tsx
- Thêm biến state: `const [isSearchModalOpen, setIsSearchModalOpen] = useState(false);`
- Sửa đổi thẻ `<input>` tìm kiếm:
  - Bổ sung `onClick={() => setIsSearchModalOpen(true)}`
  - Đổi thuộc tính `readOnly={true}` để chặn bàn phím gõ trực tiếp vào ô input này.
- Thêm `useEffect` lắng nghe sự kiện nhấn phím tắt `⌘K` (hoặc `Ctrl+K`) để tự động kích hoạt Modal `GlobalSearch`:
```tsx
useEffect(() => {
  const handleKeyDown = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      setIsSearchModalOpen(true);
    }
  };
  window.addEventListener('keydown', handleKeyDown);
  return () => window.removeEventListener('keydown', handleKeyDown);
}, []);
```
- Gọi (render) `<GlobalSearch isOpen={isSearchModalOpen} onClose={() => setIsSearchModalOpen(false)} onNavigate={navigate} />` ở phía cuối cùng của `Topbar`.

### Step 3: Tích hợp API cho Search Results
- Bỏ bộ dữ liệu giả lập `ALL_RESULTS` trong `GlobalSearch.tsx`.
- Import các React Query endpoint để thực thi global search mỗi khi `query` thay đổi (với 300ms debounce).
- Render dữ liệu trả về từ API theo từng danh mục (CVE, Findings, Asset...).

## Verification
1. Tải lại trang chủ.
2. Click vào ô tìm kiếm ở Topbar -> Modal Global Search hiện ra, ô input trên Modal tự động focus.
3. Thử nghiệm gõ `⌘K` -> Modal hiện ra.
4. Nhấn phím `Esc` -> Modal đóng lại.
5. Gõ từ khóa tìm kiếm vào Modal -> Cần có spinner loading và hiện dữ liệu đúng từ Backend.
