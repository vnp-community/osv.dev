// domain/aggregate/upstream_group/upstream_group.go
package upstreamgroup

import (
	"time"
)

// UpstreamGroup tracks directional relationships for a vulnerability.
// Upstream: vulnerabilities this one is derived from.
// Downstream: vulnerabilities derived from this one.
type UpstreamGroup struct {
	vulnID       string
	upstream     map[string]struct{} // IDs this vuln is downstream of
	downstream   map[string]struct{} // IDs this vuln is upstream of
	lastModified time.Time
}

// NewUpstreamGroup creates an empty UpstreamGroup for a given vulnID.
func NewUpstreamGroup(vulnID string) *UpstreamGroup {
	return &UpstreamGroup{
		vulnID:       vulnID,
		upstream:     make(map[string]struct{}),
		downstream:   make(map[string]struct{}),
		lastModified: time.Now().UTC(),
	}
}

// Reconstitute rebuilds from persisted state.
func Reconstitute(vulnID string, upstream, downstream []string, lastModified time.Time) *UpstreamGroup {
	g := &UpstreamGroup{
		vulnID:       vulnID,
		upstream:     make(map[string]struct{}, len(upstream)),
		downstream:   make(map[string]struct{}, len(downstream)),
		lastModified: lastModified,
	}
	for _, id := range upstream {
		g.upstream[id] = struct{}{}
	}
	for _, id := range downstream {
		g.downstream[id] = struct{}{}
	}
	return g
}

func (g *UpstreamGroup) VulnID() string     { return g.vulnID }
func (g *UpstreamGroup) LastModified() time.Time { return g.lastModified }

// AddUpstream adds an upstream dependency (this vuln is downstream of upstreamID).
func (g *UpstreamGroup) AddUpstream(upstreamID string) bool {
	if _, exists := g.upstream[upstreamID]; exists {
		return false
	}
	g.upstream[upstreamID] = struct{}{}
	g.lastModified = time.Now().UTC()
	return true
}

// AddDownstream adds a downstream dependent (downstreamID depends on this vuln).
func (g *UpstreamGroup) AddDownstream(downstreamID string) bool {
	if _, exists := g.downstream[downstreamID]; exists {
		return false
	}
	g.downstream[downstreamID] = struct{}{}
	g.lastModified = time.Now().UTC()
	return true
}

// Upstream returns all upstream dependency IDs.
func (g *UpstreamGroup) Upstream() []string {
	ids := make([]string, 0, len(g.upstream))
	for id := range g.upstream {
		ids = append(ids, id)
	}
	return ids
}

// Downstream returns all downstream dependent IDs.
func (g *UpstreamGroup) Downstream() []string {
	ids := make([]string, 0, len(g.downstream))
	for id := range g.downstream {
		ids = append(ids, id)
	}
	return ids
}
