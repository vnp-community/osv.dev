// Package product_type defines the ProductType domain entity.
package product_type

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNameRequired = errors.New("product type name is required")

// ProductType groups related Products under a common category (e.g., "Web Application", "Mobile App").
type ProductType struct {
	ID                         uuid.UUID
	Name                       string
	Description                string
	CriticalProduct            bool
	KeyProduct                 bool
	EnableFullRiskAcceptance   bool
	EnableSimpleRiskAcceptance bool
	Tags                       []string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// New creates and validates a new ProductType.
func New(name, description string) (*ProductType, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	return &ProductType{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Tags:        []string{},
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, nil
}
