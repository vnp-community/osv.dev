package triage

import "github.com/google/uuid"

// TriageAction is the recommended action for a finding
type TriageAction string

const (
	TriageActionFixNow   TriageAction = "FIX_NOW"
	TriageActionSchedule TriageAction = "SCHEDULE"
	TriageActionMonitor  TriageAction = "MONITOR"
	TriageActionAccept   TriageAction = "ACCEPT"
)

// TriageRecommendation is the AI-generated triage for a finding
type TriageRecommendation struct {
	FindingID      uuid.UUID
	Priority       int      // 1-10 (10 = most urgent)
	Rationale      string   // Human-readable reasoning
	Suggestion     TriageAction
	ContextFactors []string // ["kev_listed", "exploit_available", "asset_critical"]
	Confidence     float64  // 0.0 - 1.0
}
