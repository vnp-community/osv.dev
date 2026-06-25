// Package query provides the KEV list/get use case.
package query

import (
	"context"
	"fmt"
	"time"

	entity "github.com/osv/data-service/internal/domain/kev"
	"github.com/osv/data-service/internal/domain/repository"
)

// Request holds parameters for listing KEV entries.
type Request struct {
	Page          int
	Limit         int
	Query         string
	VendorProject string
	Since         *time.Time
	IsRansomware  bool // CR-GCV-007
	// CR-007: advanced filters
	DateFrom *time.Time // filter by date_added >= DateFrom
	DateTo   *time.Time // filter by date_added <= DateTo
	SortBy   string     // "date_added_desc" | "date_added_asc" | "vendor_asc"
}

// Response is the paginated response for KEV list queries.
type Response struct {
	Entries []*entity.KEVEntry `json:"entries"`
	Total   int64              `json:"total"`
	Page    int                `json:"page"`
	Limit   int                `json:"limit"`
	HasMore bool               `json:"has_more"`
}

// UseCase handles KEV query operations.
type UseCase struct {
	kevRepo repository.KEVRepository
}

// New creates a query UseCase.
func New(kevRepo repository.KEVRepository) *UseCase {
	return &UseCase{kevRepo: kevRepo}
}

// Execute lists KEV entries matching the request parameters.
func (uc *UseCase) Execute(ctx context.Context, req *Request) (*Response, error) {
	filter := &entity.KEVFilter{
		Query:         req.Query,
		VendorProject: req.VendorProject,
		Since:         req.Since,
		Page:          req.Page,
		Limit:         req.Limit,
	}
	if req.IsRansomware {
		filter.IsRansomware = &req.IsRansomware
	}
	// CR-007: additional filters
	if req.DateFrom != nil {
		filter.DateFrom = req.DateFrom
	}
	if req.DateTo != nil {
		filter.DateTo = req.DateTo
	}
	if req.SortBy != "" {
		filter.SortBy = req.SortBy
	}
	filter.Validate()

	entries, total, err := uc.kevRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("kev query: %w", err)
	}

	return &Response{
		Entries: entries,
		Total:   total,
		Page:    filter.Page,
		Limit:   filter.Limit,
		HasMore: int64((filter.Page+1)*filter.Limit) < total,
	}, nil
}
