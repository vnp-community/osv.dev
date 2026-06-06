# tools/deprecated/

> **Status:** Archived — Các file trong thư mục này không còn được dùng trong production

Thư mục này chứa các tools đã bị thay thế hoặc không còn cần thiết.  
Được giữ lại cho mục đích tham khảo lịch sử trước khi xóa hoàn toàn.

## Danh sách

| Tool | Lý do archive | Replaced by |
|------|--------------|-------------|
| `source-sync/` | Python script cũ | `services/source-sync/` (Go microservice) |
| `migrate/` | One-time migration Datastore→Firestore đã hoàn thành | N/A |
| `datastore-remover/` | Datastore không còn dùng | N/A |
| `sourcerepo-sync/` | Tương tự source-sync, đã replaced | `services/source-sync/` |
| `datafix/` | Ad-hoc Python scripts một lần | N/A |

## Xóa khỏi codebase

Các tool này sẽ được xóa khỏi codebase vào 2026-07-01.  
Nếu bạn cần một tool nào đó, hãy port sang Go trước khi xóa.
