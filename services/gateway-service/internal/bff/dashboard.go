package bff

// DashboardData aggregates data from multiple services for the main dashboard
type DashboardData struct {
	Findings FindingsSummary
	Scans    ScansSummary
	KEV      KEVSummary
	AI       AISummary
}

// FindingsSummary contains finding counts by severity
type FindingsSummary struct {
	Total    int
	Critical int
	High     int
	Medium   int
	Low      int
	Open     int
	Resolved int
}

// ScansSummary contains scan job statistics
type ScansSummary struct {
	Total     int
	Running   int
	Completed int
	Failed    int
}

// KEVSummary contains KEV (Known Exploited Vulnerabilities) stats
type KEVSummary struct {
	Total         int
	AffectedAssets int
}

// AISummary contains AI enrichment coverage stats
type AISummary struct {
	EnrichedCVEs int
	TotalCVEs    int
}

// DashboardAggregator aggregates data from multiple services for dashboard
type DashboardAggregator struct{}

// GetDashboard fetches and combines data for the main dashboard view
// Calls finding-service, scan-service, data-service in parallel
func (a *DashboardAggregator) GetDashboard() (*DashboardData, error) {
	// TODO: parallel gRPC calls to all services
	return &DashboardData{}, nil
}
