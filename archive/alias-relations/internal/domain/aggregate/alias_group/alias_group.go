// domain/aggregate/alias_group/alias_group.go
package aliasgroup

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/osv/alias-relations/internal/domain/valueobject"
)

// Event is a domain event marker interface.
type Event interface {
	EventName() string
}

// AliasGroupUpdated is a domain event published when an alias group changes.
type AliasGroupUpdated struct {
	EventID      string    `json:"event_id"`
	GroupID      string    `json:"group_id"`
	BugIDs       []string  `json:"bug_ids"`
	CanonicalID  string    `json:"canonical_id"`
	LastModified time.Time `json:"last_modified"`
	OccurredAt   time.Time `json:"occurred_at"`
}

func (e AliasGroupUpdated) EventName() string { return "osv.alias.group.updated" }

// AliasGroup is the core aggregate: a set of vuln IDs that refer to the same vulnerability.
type AliasGroup struct {
	id              string
	bugIDs          map[string]struct{} // Set of vuln IDs (key = string value)
	lastModified    time.Time
	detectionMethod valueobject.DetectionMethod
	events          []Event
}

// NewAliasGroup creates a new AliasGroup with the given IDs.
func NewAliasGroup(ids []string, method valueobject.DetectionMethod) *AliasGroup {
	g := &AliasGroup{
		id:              uuid.NewString(),
		bugIDs:          make(map[string]struct{}, len(ids)),
		lastModified:    time.Now().UTC(),
		detectionMethod: method,
	}
	for _, id := range ids {
		if id != "" {
			g.bugIDs[id] = struct{}{}
		}
	}
	g.appendUpdatedEvent()
	return g
}

// ReconstitueAliasGroup rebuilds an AliasGroup from persisted state (no events).
func ReconstitueAliasGroup(id string, bugIDs []string, lastModified time.Time, method valueobject.DetectionMethod) *AliasGroup {
	g := &AliasGroup{
		id:              id,
		bugIDs:          make(map[string]struct{}, len(bugIDs)),
		lastModified:    lastModified,
		detectionMethod: method,
	}
	for _, bid := range bugIDs {
		g.bugIDs[bid] = struct{}{}
	}
	return g
}

// ID returns the group's unique identifier.
func (g *AliasGroup) ID() string { return g.id }

// BugIDs returns a sorted list of all vuln IDs in this group.
func (g *AliasGroup) BugIDs() []string {
	ids := make([]string, 0, len(g.bugIDs))
	for id := range g.bugIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// LastModified returns when this group was last changed.
func (g *AliasGroup) LastModified() time.Time { return g.lastModified }

// DetectionMethod returns how the alias was detected.
func (g *AliasGroup) DetectionMethod() valueobject.DetectionMethod { return g.detectionMethod }

// Events returns pending domain events (clears internal queue).
func (g *AliasGroup) Events() []Event {
	evts := g.events
	g.events = nil
	return evts
}

// Contains returns true if the given ID is part of this group.
func (g *AliasGroup) Contains(id string) bool {
	_, ok := g.bugIDs[id]
	return ok
}

// AddID adds an ID to the group. Idempotent: no-op if already exists.
// Returns true if the group was modified.
func (g *AliasGroup) AddID(id string) bool {
	if id == "" {
		return false
	}
	if _, exists := g.bugIDs[id]; exists {
		return false
	}
	g.bugIDs[id] = struct{}{}
	g.lastModified = time.Now().UTC()
	g.appendUpdatedEvent()
	return true
}

// Merge merges another AliasGroup into this one (union of bug IDs).
// The other group should be deleted after merging.
func (g *AliasGroup) Merge(other *AliasGroup) {
	modified := false
	for id := range other.bugIDs {
		if _, exists := g.bugIDs[id]; !exists {
			g.bugIDs[id] = struct{}{}
			modified = true
		}
	}
	if modified {
		g.lastModified = time.Now().UTC()
		g.appendUpdatedEvent()
	}
}

// CanonicalID returns the primary/canonical ID for this group.
// Priority: CVE- > GHSA- > OSV- > alphabetical first.
func (g *AliasGroup) CanonicalID() string {
	prefixes := []string{"CVE-", "GHSA-", "OSV-"}
	sortedIDs := g.BugIDs()

	for _, prefix := range prefixes {
		for _, id := range sortedIDs {
			if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
				return id
			}
		}
	}

	if len(sortedIDs) > 0 {
		return sortedIDs[0]
	}
	return ""
}

// Size returns the number of IDs in this group.
func (g *AliasGroup) Size() int { return len(g.bugIDs) }

func (g *AliasGroup) appendUpdatedEvent() {
	g.events = append(g.events, AliasGroupUpdated{
		EventID:      uuid.NewString(),
		GroupID:      g.id,
		BugIDs:       g.BugIDs(),
		CanonicalID:  g.CanonicalID(),
		LastModified: g.lastModified,
		OccurredAt:   time.Now().UTC(),
	})
}
