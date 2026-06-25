// Package product defines the Product domain entity.
package product

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNameRequired = errors.New("product name is required")

// BusinessCriticality classifies how critical a product is to the business.
type BusinessCriticality string

const (
	BCVeryHigh BusinessCriticality = "very high"
	BCHigh     BusinessCriticality = "high"
	BCMedium   BusinessCriticality = "medium"
	BCLow      BusinessCriticality = "low"
	BCVeryLow  BusinessCriticality = "very low"
	BCNone     BusinessCriticality = "none"
)

// Platform describes the deployment platform of the product.
type Platform string

const (
	PlatformWeb     Platform = "web"
	PlatformMobile  Platform = "mobile"
	PlatformDesktop Platform = "desktop"
	PlatformAPI     Platform = "api"
	PlatformIoT     Platform = "iot"
)

// Origin describes the origin/ownership type of the product.
type Origin string

const (
	OriginInternal   Origin = "internal"
	OriginContractor Origin = "contractor"
	OriginOutsourced Origin = "outsourced"
	OriginOpenSource Origin = "open source"
	OriginPurchased  Origin = "purchased"
)

// Lifecycle describes the current lifecycle stage of the product.
type Lifecycle string

const (
	LCConstruction Lifecycle = "construction"
	LCProduction   Lifecycle = "production"
	LCRetirement   Lifecycle = "retirement"
)

// Product represents a DefectDojo product — a logical unit of software under test.
type Product struct {
	ID                         uuid.UUID
	ProductTypeID              uuid.UUID
	Name                       string
	Description                string
	ProdNumericGrade           int
	BusinessCriticality        BusinessCriticality
	Platform                   Platform
	Lifecycle                  Lifecycle
	Origin                     Origin
	ExternalAudience           bool
	InternetAccessible         bool
	EnableFullRiskAcceptance   bool
	EnableSimpleRiskAcceptance bool
	EnableProductTagInheritance bool
	SLAConfigurationID         *uuid.UUID
	Tags                       []string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// New creates and validates a new Product.
func New(productTypeID uuid.UUID, name, description string) (*Product, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	return &Product{
		ID:            uuid.New(),
		ProductTypeID: productTypeID,
		Name:          name,
		Description:   description,
		Tags:          []string{},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, nil
}
