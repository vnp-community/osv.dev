// Package graphql — resolver.go
// Resolver delegates GraphQL queries to existing gRPC clients in gateway-service.
// S3-GW-01: additive BFF, existing REST handlers unchanged.
package graphql

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/rs/zerolog"

	"github.com/osv/gateway-service/internal/adapter/grpcclient"
	pb "github.com/osv/shared/proto/gen/go/cvedb/v1"
)

// Resolver holds upstream gRPC clients for GraphQL field resolution.
type Resolver struct {
	aiClient    *grpcclient.AIClient
	cvedbClient *grpcclient.CVEDBClient
	log         zerolog.Logger
}

// NewResolver creates a new Resolver.
func NewResolver(aiClient *grpcclient.AIClient, cvedbClient *grpcclient.CVEDBClient, log zerolog.Logger) *Resolver {
	return &Resolver{aiClient: aiClient, cvedbClient: cvedbClient, log: log}
}

// ResolveCVE handles the cve(id: String!): CVE query.
// Calls AI enrichment in parallel with a CVE lookup stub.
func (res *Resolver) ResolveCVE(p graphql.ResolveParams) (interface{}, error) {
	id, _ := p.Args["id"].(string)
	if id == "" {
		return nil, nil
	}

	ctx, ok := p.Context.(context.Context)
	if !ok {
		ctx = context.Background()
	}

	result := map[string]interface{}{
		"id": id,
	}

	// Fetch EPSS score
	epssResp, err := res.aiClient.GetEPSS(ctx, id)
	if err == nil && epssResp != nil {
		result["epss"] = map[string]interface{}{
			"score":      epssResp.GetScore(),
			"percentile": epssResp.GetPercentile(),
		}
	}

	// Fetch enrichment (AI summary)
	enrichResp, err := res.aiClient.GetEnrichment(ctx, id)
	if err == nil && enrichResp != nil {
		result["enrichment"] = map[string]interface{}{
			"summary_short":     enrichResp.GetSummaryShort(),
			"impact_analysis":   enrichResp.GetImpactAnalysis(),
			"remediation_guide": enrichResp.GetRemediationGuide(),
		}
		result["summary"]   = enrichResp.GetSummaryShort()
		result["severity"]  = enrichResp.GetSeverityMl()
		result["cvss_score"] = enrichResp.GetEpssScore()
	}

	return result, nil
}

// ResolveSearchCVEs handles the searchCVEs(query, page, limit) query.
// Delegates to CVEDBClient.LookupCVEs using the query as product name.
func (res *Resolver) ResolveSearchCVEs(p graphql.ResolveParams) (interface{}, error) {
	query, _ := p.Args["query"].(string)
	page, _ := p.Args["page"].(int)
	limit, _ := p.Args["limit"].(int)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page < 0 {
		page = 0
	}

	ctx, ok := p.Context.(context.Context)
	if !ok {
		ctx = context.Background()
	}

	// Build a ProductInfo lookup using the query as product name
	lookupResult, err := res.cvedbClient.LookupCVEs(ctx, grpcclient.LookupRequest{
		Products: []*pb.ProductInfo{{Product: query}},
	})
	if err != nil {
		res.log.Warn().Err(err).Str("query", query).Msg("GraphQL searchCVEs: lookup failed")
		return map[string]interface{}{
			"items": []interface{}{},
			"total": 0,
			"page":  page,
			"limit": limit,
		}, nil
	}

	items := make([]map[string]interface{}, 0)
	for _, productCVEs := range lookupResult.Results {
		for _, cve := range productCVEs.GetCves() {
			items = append(items, map[string]interface{}{
				"id":         cve.GetCveNumber(),
				"summary":    cve.GetDescription(),
				"severity":   cve.GetSeverity(),
				"cvss_score": float64(cve.GetScore()),
				"is_kev":     cve.GetIsExploit(),
			})
		}
	}

	return map[string]interface{}{
		"items": items,
		"total": len(items),
		"page":  page,
		"limit": limit,
	}, nil
}


// ResolveDashboard handles the dashboard: DashboardData query.
// Returns placeholder data — full wiring with finding-service done in S4.
func (res *Resolver) ResolveDashboard(p graphql.ResolveParams) (interface{}, error) {
	return map[string]interface{}{
		"critical_count": 0,
		"high_count":     0,
		"medium_count":   0,
		"low_count":      0,
		"total_findings": 0,
	}, nil
}
