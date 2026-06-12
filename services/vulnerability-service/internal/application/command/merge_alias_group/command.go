// application/command/merge_alias_group/command.go
package mergealiasgroup

// Command requests merging of declared aliases for a vulnerability.
type Command struct {
	// VulnID is the primary vulnerability identifier.
	VulnID string
	// DeclaredAliases are the aliases[] from the OSV record.
	DeclaredAliases []string
}

// Result is returned after successful processing.
type Result struct {
	GroupID     string
	CanonicalID string
	AllIDs      []string
}
