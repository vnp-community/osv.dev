// Package graphql — schema.go
// GraphQL schema definition for the gateway-service BFF using github.com/graphql-go/graphql.
// S3-GW-01: additive — existing REST handlers unchanged.
//
// Route wired in main.go:
//   POST /graphql → server.Handler()
//   GET  /graphql → GraphiQL playground (dev only, behind DEV_MODE env)
package graphql

import (
	"github.com/graphql-go/graphql"
)

// ── Object Types ─────────────────────────────────────────────────────────────

// EPSSDataType represents EPSS score for a CVE.
var EPSSDataType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EPSSData",
	Fields: graphql.Fields{
		"score":      {Type: graphql.Float},
		"percentile": {Type: graphql.Float},
	},
})

// EnrichmentSummaryType contains AI-generated CVE enrichment.
var EnrichmentSummaryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnrichmentSummary",
	Fields: graphql.Fields{
		"summary_short":     {Type: graphql.String},
		"impact_analysis":   {Type: graphql.String},
		"remediation_guide": {Type: graphql.String},
	},
})

// FindingType represents an active security finding.
var FindingType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Finding",
	Fields: graphql.Fields{
		"id":       {Type: graphql.String},
		"title":    {Type: graphql.String},
		"severity": {Type: graphql.String},
		"state":    {Type: graphql.String},
		"cve":      {Type: graphql.String},
	},
})

// CVEType is the primary CVE aggregate returned by queries.
var CVEType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CVE",
	Fields: graphql.Fields{
		"id":         {Type: graphql.String},
		"summary":    {Type: graphql.String},
		"severity":   {Type: graphql.String},
		"cvss_score": {Type: graphql.Float},
		"is_kev":     {Type: graphql.Boolean},
		"epss":       {Type: EPSSDataType},
		"enrichment": {Type: EnrichmentSummaryType},
		"active_findings": {
			Type: graphql.NewList(FindingType),
		},
	},
})

// CVESearchResultType wraps paginated CVE search results.
var CVESearchResultType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CVESearchResult",
	Fields: graphql.Fields{
		"items": {Type: graphql.NewList(CVEType)},
		"total": {Type: graphql.Int},
		"page":  {Type: graphql.Int},
		"limit": {Type: graphql.Int},
	},
})

// DashboardDataType contains dashboard summary metrics.
var DashboardDataType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DashboardData",
	Fields: graphql.Fields{
		"critical_count": {Type: graphql.Int},
		"high_count":     {Type: graphql.Int},
		"medium_count":   {Type: graphql.Int},
		"low_count":      {Type: graphql.Int},
		"total_findings": {Type: graphql.Int},
	},
})

// BuildSchema constructs and returns the GraphQL schema.
// Resolvers are injected via the Resolver struct.
func BuildSchema(r *Resolver) (graphql.Schema, error) {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"cve": {
				Type:        CVEType,
				Description: "Fetch a single CVE with enrichment and EPSS data",
				Args: graphql.FieldConfigArgument{
					"id": {Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: r.ResolveCVE,
			},
			"searchCVEs": {
				Type:        CVESearchResultType,
				Description: "Search CVEs by keyword with pagination",
				Args: graphql.FieldConfigArgument{
					"query": {Type: graphql.NewNonNull(graphql.String)},
					"page":  {Type: graphql.Int},
					"limit": {Type: graphql.Int},
				},
				Resolve: r.ResolveSearchCVEs,
			},
			"dashboard": {
				Type:        DashboardDataType,
				Description: "Fetch dashboard summary metrics for the authenticated user",
				Resolve:     r.ResolveDashboard,
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
}
