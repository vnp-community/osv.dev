package repository

import (
	"context"
	"errors"

	"github.com/osv/search-service/internal/domain/entity"
)

var ErrNotFound = errors.New("not found")

type CWERepository interface {
	List(ctx context.Context, q string, page, limit int) ([]*entity.CWEEntry, int64, error)
	FindByID(ctx context.Context, id string) (*entity.CWEEntry, error)
}

type CAPECRepository interface {
	List(ctx context.Context, q, cweID string, page, limit int) ([]*entity.CAPECEntry, int64, error)
	FindByID(ctx context.Context, id string) (*entity.CAPECEntry, error)
}
