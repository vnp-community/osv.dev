// Package models — AliasGroup entity port from Python osv/models.py.
package models

import "time"

// AliasGroup tracks the canonical grouping of aliases for a vulnerability.
// Equivalent to Python's AliasGroup(ndb.Model).
type AliasGroup struct {
	// BugID is the canonical (primary) OSV ID for the group.
	BugID string

	// Members contains all alias IDs (including the canonical BugID).
	// e.g. ["CVE-2023-44487", "GHSA-xxx-yyy-zzz"]
	Members []string

	// LastModified is when the group was last updated.
	LastModified time.Time
}

// AddMember adds an alias to the group if not already present.
func (g *AliasGroup) AddMember(id string) bool {
	for _, m := range g.Members {
		if m == id {
			return false // already exists
		}
	}
	g.Members = append(g.Members, id)
	g.LastModified = time.Now().UTC()
	return true
}

// RemoveMember removes an alias from the group.
func (g *AliasGroup) RemoveMember(id string) bool {
	for i, m := range g.Members {
		if m == id {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)
			g.LastModified = time.Now().UTC()
			return true
		}
	}
	return false
}

// Contains returns true if id is a member of this group.
func (g *AliasGroup) Contains(id string) bool {
	for _, m := range g.Members {
		if m == id {
			return true
		}
	}
	return false
}

// ── SourceRepositoryRef ────────────────────────────────────────────────────────

// SourceRepositoryRef is the lightweight reference entity tracked in Datastore/Firestore.
// Equivalent to osv/models.py SourceRepository relevant fields.
type SourceRepositoryRef struct {
	// Name is the unique identifier for the source (e.g. "ghsa", "debian").
	Name string

	// Link is the URL of the repository or feed.
	Link string

	// LastSyncHash records the last git commit hash or ETag synced.
	LastSyncHash string

	// LastSyncAt is when the source was last successfully synced.
	LastSyncAt time.Time

	// LastEnqueuedAt is when the source was last enqueued for sync.
	LastEnqueuedAt time.Time

	// IgnorePatterns contains file glob patterns to skip during sync.
	IgnorePatterns []string

	// Paused indicates whether the source is paused from syncing.
	Paused bool
}

// IsNew returns true if the source has never been synced.
// Equivalent to Python's SourceRepository.is_new().
func (s *SourceRepositoryRef) IsNew() bool {
	return s.LastSyncHash == "" && s.LastSyncAt.IsZero()
}

// IsChanged returns true if the given hash/ETag differs from the last synced value.
// Equivalent to Python's SourceRepository.is_changed().
func (s *SourceRepositoryRef) IsChanged(newHash string) bool {
	return s.LastSyncHash != newHash
}
