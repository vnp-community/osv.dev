package finding

import "errors"

// ErrInvalidTransition is returned when a state change is not allowed.
var ErrInvalidTransition = errors.New("invalid state transition")

// FindingState is the logical state of a finding derived from its boolean flags.
type FindingState string

const (
	StateActive        FindingState = "active"
	StateMitigated     FindingState = "mitigated"
	StateFalsePositive FindingState = "false_positive"
	StateRiskAccepted  FindingState = "risk_accepted"
	StateOutOfScope    FindingState = "out_of_scope"
	StateDuplicate     FindingState = "duplicate"
)

// CurrentState derives the logical state from the finding's boolean flags.
// Priority: Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
func (f *Finding) CurrentState() FindingState {
	switch {
	case f.Duplicate:
		return StateDuplicate
	case f.FalsePositive:
		return StateFalsePositive
	case f.OutOfScope:
		return StateOutOfScope
	case f.RiskAccepted:
		return StateRiskAccepted
	case f.IsMitigated:
		return StateMitigated
	default:
		return StateActive
	}
}

// ValidTransitions defines which state transitions are allowed.
var ValidTransitions = map[FindingState]map[FindingState]bool{
	StateActive:        {StateMitigated: true, StateFalsePositive: true, StateRiskAccepted: true, StateOutOfScope: true},
	StateMitigated:     {StateActive: true},
	StateFalsePositive: {StateActive: true},
	StateRiskAccepted:  {StateActive: true},
	StateOutOfScope:    {StateActive: true},
	StateDuplicate:     {}, // duplicates cannot be manually transitioned
}

// CanTransitionTo checks if the transition to next is valid.
func (f *Finding) CanTransitionTo(next FindingState) bool {
	return ValidTransitions[f.CurrentState()][next]
}

// Close marks the finding as mitigated (Active → Mitigated).
func (f *Finding) Close(mitigatedByID *string) error {
	if !f.CanTransitionTo(StateMitigated) {
		return ErrInvalidTransition
	}
	now := nowUTC()
	f.Active = false
	f.IsMitigated = true
	f.MitigatedAt = &now
	if mitigatedByID != nil {
		id := mustParseUUID(*mitigatedByID)
		f.MitigatedByID = &id
	}
	f.LastStatusUpdate = &now
	f.UpdatedAt = now
	return nil
}

// Reopen moves a mitigated/false-positive/risk-accepted finding back to Active.
func (f *Finding) Reopen() error {
	if !f.CanTransitionTo(StateActive) {
		return ErrInvalidTransition
	}
	now := nowUTC()
	f.Active = true
	f.IsMitigated = false
	f.FalsePositive = false
	f.RiskAccepted = false
	f.OutOfScope = false
	f.MitigatedAt = nil
	f.MitigatedByID = nil
	f.LastStatusUpdate = &now
	f.UpdatedAt = now
	return nil
}

// MarkFalsePositive transitions Active → FalsePositive.
func (f *Finding) MarkFalsePositive() error {
	if !f.CanTransitionTo(StateFalsePositive) {
		return ErrInvalidTransition
	}
	now := nowUTC()
	f.FalsePositive = true
	f.Active = false
	f.LastStatusUpdate = &now
	f.UpdatedAt = now
	return nil
}

// AcceptRisk transitions Active → RiskAccepted.
func (f *Finding) AcceptRisk() error {
	if !f.CanTransitionTo(StateRiskAccepted) {
		return ErrInvalidTransition
	}
	now := nowUTC()
	f.RiskAccepted = true
	f.Active = false
	f.LastStatusUpdate = &now
	f.UpdatedAt = now
	return nil
}

// MarkOutOfScope transitions Active → OutOfScope.
func (f *Finding) MarkOutOfScope() error {
	if !f.CanTransitionTo(StateOutOfScope) {
		return ErrInvalidTransition
	}
	now := nowUTC()
	f.OutOfScope = true
	f.Active = false
	f.LastStatusUpdate = &now
	f.UpdatedAt = now
	return nil
}

// MarkDuplicate marks this finding as a duplicate of originalID.
// Duplicates cannot be manually transitioned; only the dedup engine sets this.
func (f *Finding) MarkDuplicate(originalID *string) {
	now := nowUTC()
	f.Duplicate = true
	f.Active = false
	if originalID != nil {
		id := mustParseUUID(*originalID)
		f.DuplicateFindingID = &id
	}
	f.LastStatusUpdate = &now
	f.UpdatedAt = now
}

// ApplyTransition is a generic state transition dispatcher.
// It delegates to the appropriate method and validates the transition.
func (f *Finding) ApplyTransition(target FindingState) error {
	switch target {
	case StateMitigated:
		// Use Close with no mitigated-by — caller should use Close() for full audit
		if !f.CanTransitionTo(StateMitigated) {
			return ErrInvalidTransition
		}
		now := nowUTC()
		f.IsMitigated = true
		f.Active = false
		f.MitigatedAt = &now
		f.LastStatusUpdate = &now
		f.UpdatedAt = now
		return nil
	case StateActive:
		return f.Reopen()
	case StateFalsePositive:
		return f.MarkFalsePositive()
	case StateOutOfScope:
		return f.MarkOutOfScope()
	case StateRiskAccepted:
		return f.AcceptRisk()
	default:
		return ErrInvalidTransition
	}
}
