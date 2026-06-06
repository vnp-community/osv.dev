// domain/service/severity_classifier.go
package service

import (
	"context"
	"fmt"

	"github.com/osv/ai-enrichment/internal/application/port"
	"github.com/rs/zerolog"
)

// Severity levels matching OSV severity field.
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "CRITICAL"
	SeverityHigh     SeverityLevel = "HIGH"
	SeverityMedium   SeverityLevel = "MEDIUM"
	SeverityLow      SeverityLevel = "LOW"
)

// SeverityPrediction holds the LLM classification result.
type SeverityPrediction struct {
	Severity   SeverityLevel
	Confidence float32
	Reasoning  string
	Source     string // "cvss" | "llm"
}

// CVSSSeverity is a lightweight representation of an existing CVSS score.
type CVSSSeverity struct {
	Score float64
	Type  string // "CVSS_V3" | "CVSS_V2"
}

// SeverityClassifier classifies vulnerability severity.
// Priority 1: derive from existing CVSS (deterministic).
// Priority 2: LLM classification (expensive).
type SeverityClassifier struct {
	llm port.LLMProvider
	log zerolog.Logger
}

// NewSeverityClassifier creates a SeverityClassifier.
func NewSeverityClassifier(llm port.LLMProvider, log zerolog.Logger) *SeverityClassifier {
	return &SeverityClassifier{llm: llm, log: log}
}

// Classify determines the severity of a vulnerability.
func (c *SeverityClassifier) Classify(ctx context.Context, summary, details string, existingCVSS []CVSSSeverity) (*SeverityPrediction, error) {
	// Priority 1: derive from existing CVSS score (deterministic, no LLM)
	for _, cvss := range existingCVSS {
		if cvss.Type == "CVSS_V3" {
			return deriveSeverityFromCVSS(cvss.Score), nil
		}
	}
	// Fall back to CVSS v2 if no v3
	for _, cvss := range existingCVSS {
		if cvss.Type == "CVSS_V2" {
			return deriveSeverityFromCVSS(cvss.Score), nil
		}
	}

	// Priority 2: LLM classification
	return c.classifyWithLLM(ctx, summary, details)
}

// deriveSeverityFromCVSS converts a numeric CVSS score to a severity level.
func deriveSeverityFromCVSS(score float64) *SeverityPrediction {
	var level SeverityLevel
	switch {
	case score >= 9.0:
		level = SeverityCritical
	case score >= 7.0:
		level = SeverityHigh
	case score >= 4.0:
		level = SeverityMedium
	default:
		level = SeverityLow
	}
	return &SeverityPrediction{
		Severity:   level,
		Confidence: 1.0,
		Reasoning:  fmt.Sprintf("Derived from CVSS score %.1f", score),
		Source:     "cvss",
	}
}

func (c *SeverityClassifier) classifyWithLLM(ctx context.Context, summary, details string) (*SeverityPrediction, error) {
	prompt := fmt.Sprintf(`You are a security expert. Classify the severity of this vulnerability.
Summary: %s
Details: %s

Respond ONLY with valid JSON in this exact format:
{"severity": "CRITICAL|HIGH|MEDIUM|LOW", "confidence": 0.0-1.0, "reasoning": "brief explanation"}`,
		truncate(summary, 500), truncate(details, 1500))

	var result struct {
		Severity   string  `json:"severity"`
		Confidence float32 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := c.llm.GenerateStructured(ctx, prompt, &result); err != nil {
		return nil, fmt.Errorf("LLM severity classification failed: %w", err)
	}

	c.log.Debug().
		Str("severity", result.Severity).
		Float32("confidence", result.Confidence).
		Msg("LLM severity classification complete")

	return &SeverityPrediction{
		Severity:   SeverityLevel(result.Severity),
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
		Source:     "llm",
	}, nil
}
