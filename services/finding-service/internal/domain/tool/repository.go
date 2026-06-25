package tool

import (
	"context"

	"github.com/google/uuid"
)

// ToolConfigurationRepository defines persistence operations for tool configurations.
type ToolConfigurationRepository interface {
	Save(ctx context.Context, tool *ToolConfiguration) error
	FindByID(ctx context.Context, id uuid.UUID) (*ToolConfiguration, error)
	List(ctx context.Context) ([]*ToolConfiguration, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
