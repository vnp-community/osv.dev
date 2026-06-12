// Package cwe provides a CWE (Common Weakness Enumeration) database for
// enrichment and classification of vulnerabilities.
//
// Data is embedded from the NVD CWE list (subset of most common CWEs).
// Full list: https://cwe.mitre.org/data/published/cwe_latest.xml.zip
package cwe

import (
	"strings"
)

// Category represents a top-level CWE category grouping.
type Category string

const (
	CategoryMemory         Category = "Memory Safety"
	CategoryInjection      Category = "Injection"
	CategoryCryptography   Category = "Cryptography"
	CategoryAuthentication Category = "Authentication"
	CategoryAuthorization  Category = "Authorization"
	CategoryConcurrency    Category = "Concurrency"
	CategoryFileIO         Category = "File/IO"
	CategoryNetwork        Category = "Network"
	CategoryResourceMgmt   Category = "Resource Management"
	CategoryDesign         Category = "Design"
	CategoryOther          Category = "Other"
)

// Entry represents a CWE database entry.
type Entry struct {
	ID          string   // e.g. "CWE-79"
	Name        string   // e.g. "Improper Neutralization of Input..."
	Abstraction string   // Class, Base, Variant, Compound
	Category    Category // High-level category
	Parents     []string // Parent CWE IDs
	Tags        []string // Suggested OSV tags derived from this CWE
}

// database maps CWE ID → Entry (populated from cweDB below).
var database map[string]*Entry

func init() {
	database = make(map[string]*Entry, len(cweDB))
	for i := range cweDB {
		database[cweDB[i].ID] = &cweDB[i]
	}
}

// Get returns a CWE entry by ID (e.g. "CWE-79" or "79").
// Returns nil if the CWE is not found.
func Get(id string) *Entry {
	id = normalize(id)
	return database[id]
}

// Tags returns the recommended OSV tags for a given CWE ID.
// Returns empty slice if CWE not found.
func Tags(id string) []string {
	e := Get(id)
	if e == nil {
		return nil
	}
	return e.Tags
}

// Category returns the category for a given CWE ID.
func GetCategory(id string) Category {
	e := Get(id)
	if e == nil {
		return CategoryOther
	}
	return e.Category
}

// TagsForAll returns deduplicated tags from multiple CWE IDs.
func TagsForAll(ids []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, id := range ids {
		for _, t := range Tags(id) {
			if !seen[t] {
				seen[t] = true
				result = append(result, t)
			}
		}
	}
	return result
}

// normalize ensures the CWE ID has the "CWE-" prefix.
func normalize(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ToUpper(id)
	if !strings.HasPrefix(id, "CWE-") {
		id = "CWE-" + strings.TrimPrefix(id, "CWE-")
	}
	return id
}

// IsKnown returns true if the CWE ID is in the database.
func IsKnown(id string) bool {
	return database[normalize(id)] != nil
}

// List returns all CWE entries.
func List() []*Entry {
	result := make([]*Entry, 0, len(database))
	for _, e := range database {
		result = append(result, e)
	}
	return result
}

// ByCategory returns all CWEs for a given category.
func ByCategory(cat Category) []*Entry {
	var result []*Entry
	for _, e := range database {
		if e.Category == cat {
			result = append(result, e)
		}
	}
	return result
}
