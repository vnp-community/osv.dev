// Package http — pagination.go
// Single source of truth for all pagination constants in finding-service HTTP delivery.
// [FIX BUG-008] Previously: magic numbers 20/50/200/500 scattered across handlers.
package http

const (
	// DefaultPageSize là page_size mặc định khi không có query param.
	// Áp dụng cho: FindingHandler.List
	DefaultPageSize = 20

	// DefaultProductPageSize là page_size mặc định cho ProductHandler.List.
	// Product list thường ít item hơn, default 25 là hợp lý.
	DefaultProductPageSize = 25

	// MaxPageSize là page_size tối đa được phép cho các handler (trừ internal).
	// Giúp tránh query quá lớn làm overload DB.
	MaxPageSize = 200

	// MaxInternalPageSize là giới hạn cho internal handler.
	// Internal calls (finding count by CVE IDs, etc.) có limit riêng.
	MaxInternalPageSize = 50

	// MaxBulkFindingsPerRequest là số finding tối đa trong một bulk import request.
	// Được dùng bởi findingbulk usecase.
	MaxBulkFindingsPerRequest = 500
)
