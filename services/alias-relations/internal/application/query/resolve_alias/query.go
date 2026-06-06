// application/query/resolve_alias/query.go
package resolvealias

// Query requests alias resolution: given any vuln ID, find the canonical group.
type Query struct {
	VulnID string
}

// Result is the alias resolution result.
type Result struct {
	CanonicalID string
	AllIDs      []string
	GroupID     string
	LastModified string
	Found       bool
}
