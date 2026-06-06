// Package pipeline contains individual vulnerability enrichers for the worker pipeline.
package pipeline

import (
	"context"

	"github.com/osv/pkg/models"
	ecosystem "github.com/osv/pkg/ecosystem/impl"
	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

type EnrichParams struct {
	PathInSource      string
	SourceRepo        *models.SourceRepository
	EcosystemProvider *ecosystem.Provider
	ExistingVuln      *osvschema.Vulnerability
	RelationsStore    models.RelationsStore
}

type Enricher interface {
	Enrich(ctx context.Context, vuln *osvschema.Vulnerability, params *EnrichParams) error
}
