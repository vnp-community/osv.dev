// application/command/detect_new_aliases/command.go
package detectnewaliases

// Command triggers AI-based alias detection for a vulnerability.
type Command struct {
	VulnID    string
	Embedding []float32
}

// Result contains potential alias candidates found.
type Result struct {
	VulnID     string
	Candidates []string // IDs that may be aliases (score >= threshold)
	Merged     bool     // Whether any were merged
}
