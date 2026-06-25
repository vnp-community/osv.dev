package triage

import (
    "context"
    "fmt"
    "strings"

    "github.com/rs/zerolog"
)

// TriageDecision represents the AI recommendation.
type TriageDecision string

const (
    DecisionConfirmed    TriageDecision = "Confirmed"
    DecisionFalsePositive TriageDecision = "FalsePositive"
    DecisionNotAffected  TriageDecision = "NotAffected"
    DecisionNeedsReview  TriageDecision = "NeedsReview"
)

// FindingInput is the finding data passed for triage.
type FindingInput struct {
    Title            string
    Description      string
    Mitigation       string
    CVE              string
    CVSSv3Score      *float64
    ComponentName    string
    ComponentVersion string
    Environment      string // "production" | "staging" | "dev"
    AssetCriticality string // "critical" | "high" | "medium" | "low"
}

// TriageResult is the AI triage recommendation.
type TriageResult struct {
    Decision    TriageDecision
    Confidence  float32
    Reasoning   string
    Suggestion  string // Suggested next action
    AICost      float64
}

// LLMProvider defines the LLM interface for triage.
type LLMProvider interface {
    Generate(ctx context.Context, prompt string) (string, error)
}

// Service provides AI-assisted finding triage recommendations.
type Service struct {
    llm    LLMProvider
    logger zerolog.Logger
}

// New creates a TriageService.
func New(llm LLMProvider, logger zerolog.Logger) *Service {
    return &Service{llm: llm, logger: logger}
}

// Triage provides an AI recommendation for a security finding.
func (s *Service) Triage(ctx context.Context, in FindingInput) (*TriageResult, error) {
    prompt := s.buildPrompt(in)

    response, err := s.llm.Generate(ctx, prompt)
    if err != nil {
        return nil, fmt.Errorf("LLM triage generation: %w", err)
    }

    result := parseTriageResponse(response)

    s.logger.Info().
        Str("cve", in.CVE).
        Str("component", in.ComponentName).
        Str("decision", string(result.Decision)).
        Float32("confidence", result.Confidence).
        Msg("AI triage completed")

    return result, nil
}

// buildPrompt constructs the triage prompt for the LLM.
func (s *Service) buildPrompt(in FindingInput) string {
    cvssStr := "Unknown"
    if in.CVSSv3Score != nil {
        cvssStr = fmt.Sprintf("%.1f", *in.CVSSv3Score)
    }

    return fmt.Sprintf(`You are a security analyst performing finding triage.

FINDING DETAILS:
- Title: %s
- CVE: %s
- CVSSv3 Score: %s
- Component: %s %s
- Environment: %s
- Asset Criticality: %s

DESCRIPTION:
%s

SUGGESTED MITIGATION:
%s

Based on the above, provide a triage decision in this exact format:
DECISION: [Confirmed|FalsePositive|NotAffected|NeedsReview]
CONFIDENCE: [0.0-1.0]
REASONING: [1-2 sentences explaining the decision]
SUGGESTION: [Recommended next action]`,
        in.Title,
        in.CVE,
        cvssStr,
        in.ComponentName,
        in.ComponentVersion,
        in.Environment,
        in.AssetCriticality,
        truncate(in.Description, 500),
        truncate(in.Mitigation, 300),
    )
}

// parseTriageResponse parses the structured LLM response.
func parseTriageResponse(response string) *TriageResult {
    result := &TriageResult{
        Decision:   DecisionNeedsReview,
        Confidence: 0.5,
        Reasoning:  response,
    }

    lines := strings.Split(response, "\n")
    for _, line := range lines {
        parts := strings.SplitN(line, ": ", 2)
        if len(parts) != 2 {
            continue
        }
        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])

        switch key {
        case "DECISION":
            switch value {
            case "Confirmed":
                result.Decision = DecisionConfirmed
            case "FalsePositive":
                result.Decision = DecisionFalsePositive
            case "NotAffected":
                result.Decision = DecisionNotAffected
            default:
                result.Decision = DecisionNeedsReview
            }
        case "CONFIDENCE":
            var conf float32
            fmt.Sscanf(value, "%f", &conf)
            result.Confidence = conf
        case "REASONING":
            result.Reasoning = value
        case "SUGGESTION":
            result.Suggestion = value
        }
    }

    return result
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}
