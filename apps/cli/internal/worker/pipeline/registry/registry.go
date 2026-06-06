// Package registry contains all the enrichers that are used in the worker pipeline.
package registry

import (
	"github.com/osv/apps/cli/internal/worker/pipeline"
	"github.com/osv/apps/cli/internal/worker/pipeline/enumerateversions"
	"github.com/osv/apps/cli/internal/worker/pipeline/filterecosystem"
	"github.com/osv/apps/cli/internal/worker/pipeline/makesemver"
	"github.com/osv/apps/cli/internal/worker/pipeline/namenormalize"
	"github.com/osv/apps/cli/internal/worker/pipeline/published"
	"github.com/osv/apps/cli/internal/worker/pipeline/purl"
	"github.com/osv/apps/cli/internal/worker/pipeline/relations"
	"github.com/osv/apps/cli/internal/worker/pipeline/schemaversion"
	"github.com/osv/apps/cli/internal/worker/pipeline/sourcelink"
)

// List is the list of all enrichers used in the worker pipeline.
var List = []pipeline.Enricher{
	&namenormalize.Enricher{},
	&filterecosystem.Enricher{},
	&makesemver.Enricher{},
	&enumerateversions.Enricher{},
	&schemaversion.Enricher{},
	&purl.Enricher{},
	&sourcelink.Enricher{},
	&published.Enricher{},
	&relations.Enricher{},
}
