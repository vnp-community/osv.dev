// Package triage provides the AI triage use case for DefectDojo findings.
package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TriageFindingInput contains finding data needed to build the triage prompt.
type TriageFindingInput struct {
	FindingID   string
	Title       string
	Description string
	Severity    string
	CVE         string
	CVSSv3Score *float64
}

// TriageSuggestion is the structured AI-generated triage output.
type TriageSuggestion struct {
	ID          uuid.UUID
	FindingID   string
	Exploitable bool
	RiskRating  string   // "Critical" | "High" | "Medium" | "Low" | "Informational"
	Summary     string   // Brief AI-written risk summary
	Remediation string   // Specific remediation steps
	References  []string // Relevant CVE/CWE/patch links
	Model       string   // LLM model used
	GeneratedAt time.Time
}

// LLMClient is the interface for any LLM backend.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// TriageFindingUseCase uses an LLM to generate triage suggestions.
type TriageFindingUseCase struct {
	llm LLMClient
}

func New(llm LLMClient) *TriageFindingUseCase {
	return &TriageFindingUseCase{llm: llm}
}

// Execute builds a prompt, calls the LLM, and returns a structured suggestion.
func (uc *TriageFindingUseCase) Execute(ctx context.Context, in TriageFindingInput) (*TriageSuggestion, error) {
	prompt := buildTriagePrompt(in)
	response, err := uc.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	suggestion, err := parseTriageResponse(response, in.FindingID)
	if err != nil {
		// If parsing fails, return raw summary
		return &TriageSuggestion{
			ID:          uuid.New(),
			FindingID:   in.FindingID,
			Summary:     response,
			GeneratedAt: time.Now().UTC(),
		}, nil
	}
	return suggestion, nil
}

// buildTriagePrompt constructs the LLM prompt for triage analysis.
func buildTriagePrompt(in TriageFindingInput) string {
	cvss := ""
	if in.CVSSv3Score != nil {
		cvss = fmt.Sprintf("CVSS v3 Score: %.1f\n", *in.CVSSv3Score)
	}
	return fmt.Sprintf(`You are a security expert performing vulnerability triage.

Analyze the following vulnerability finding and provide a structured JSON response.

Finding Details:
- Title: %s
- Severity: %s
- CVE: %s
%s- Description: %s

Respond with JSON in this exact format:
{
  "exploitable": true|false,
  "risk_rating": "Critical|High|Medium|Low|Informational",
  "summary": "<2-3 sentence risk summary>",
  "remediation": "<specific remediation steps>",
  "references": ["<url1>", "<url2>"]
}`, in.Title, in.Severity, in.CVE, cvss, in.Description)
}

// parseTriageResponse parses the LLM JSON response into a TriageSuggestion.
func parseTriageResponse(response, findingID string) (*TriageSuggestion, error) {
	var parsed struct {
		Exploitable bool     `json:"exploitable"`
		RiskRating  string   `json:"risk_rating"`
		Summary     string   `json:"summary"`
		Remediation string   `json:"remediation"`
		References  []string `json:"references"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, err
	}
	return &TriageSuggestion{
		ID:          uuid.New(),
		FindingID:   findingID,
		Exploitable: parsed.Exploitable,
		RiskRating:  parsed.RiskRating,
		Summary:     parsed.Summary,
		Remediation: parsed.Remediation,
		References:  parsed.References,
		GeneratedAt: time.Now().UTC(),
	}, nil
}
