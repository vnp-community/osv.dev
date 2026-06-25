package severity

import (
    "context"
    "fmt"
    "strings"

    "github.com/rs/zerolog"
)

// SeverityLevel is the classification output.
type SeverityLevel string

const (
    SeverityCritical SeverityLevel = "CRITICAL"
    SeverityHigh     SeverityLevel = "HIGH"
    SeverityMedium   SeverityLevel = "MEDIUM"
    SeverityLow      SeverityLevel = "LOW"
    SeverityInfo     SeverityLevel = "INFO"
)

// CVSSSeverity holds a CVSS score with its version.
type CVSSSeverity struct {
    Score float64
    Type  string // "CVSS_V3" | "CVSS_V2"
}

// SeverityPrediction is the result of severity classification.
type SeverityPrediction struct {
    Severity   SeverityLevel
    Confidence float32 // 0.0-1.0
    Reasoning  string
    Source     string // "cvss_v3" | "cvss_v2" | "llm"
}

// LLMProvider generates text completions.
type LLMProvider interface {
    Generate(ctx context.Context, prompt string) (string, error)
}

// Classifier classifies CVE severity using CVSS first, then LLM fallback.
type Classifier struct {
    llm    LLMProvider
    logger zerolog.Logger
}

// New creates a SeverityClassifier.
func New(llm LLMProvider, logger zerolog.Logger) *Classifier {
    return &Classifier{llm: llm, logger: logger}
}

// Classify returns the severity for a CVE. Priority: CVSS_V3 → CVSS_V2 → LLM.
func (c *Classifier) Classify(
    ctx context.Context,
    summary, details string,
    existingCVSS []CVSSSeverity,
) (*SeverityPrediction, error) {
    // 1. CVSS_V3 (deterministic, confidence=1.0)
    for _, cvss := range existingCVSS {
        if cvss.Type == "CVSS_V3" {
            return &SeverityPrediction{
                Severity:   cvssScoreToLevel(cvss.Score),
                Confidence: 1.0,
                Reasoning:  fmt.Sprintf("CVSSv3 score: %.1f", cvss.Score),
                Source:     "cvss_v3",
            }, nil
        }
    }

    // 2. CVSS_V2 (fallback, confidence=0.9)
    for _, cvss := range existingCVSS {
        if cvss.Type == "CVSS_V2" {
            return &SeverityPrediction{
                Severity:   cvssScoreToLevel(cvss.Score),
                Confidence: 0.9,
                Reasoning:  fmt.Sprintf("CVSSv2 score: %.1f", cvss.Score),
                Source:     "cvss_v2",
            }, nil
        }
    }

    // 3. LLM fallback (confidence varies)
    return c.classifyWithLLM(ctx, summary, details)
}

// classifyWithLLM uses an LLM to classify severity when no CVSS data is available.
func (c *Classifier) classifyWithLLM(ctx context.Context, summary, details string) (*SeverityPrediction, error) {
    prompt := fmt.Sprintf(`You are a cybersecurity expert. Classify the severity of this vulnerability.
Respond with ONLY one word: CRITICAL, HIGH, MEDIUM, LOW, or INFO.

Summary: %s

Details: %s

Severity:`, truncate(summary, 500), truncate(details, 1000))

    response, err := c.llm.Generate(ctx, prompt)
    if err != nil {
        // Fallback to MEDIUM when LLM fails
        c.logger.Warn().Err(err).Msg("LLM severity classification failed, defaulting to MEDIUM")
        return &SeverityPrediction{
            Severity:   SeverityMedium,
            Confidence: 0.0,
            Reasoning:  "LLM unavailable, defaulting to MEDIUM",
            Source:     "default",
        }, nil
    }

    severity := parseLLMSeverity(strings.TrimSpace(response))
    return &SeverityPrediction{
        Severity:   severity,
        Confidence: 0.7,
        Reasoning:  fmt.Sprintf("LLM classification: %s", response),
        Source:     "llm",
    }, nil
}

// cvssScoreToLevel maps a CVSS numeric score to a severity level.
func cvssScoreToLevel(score float64) SeverityLevel {
    switch {
    case score >= 9.0:
        return SeverityCritical
    case score >= 7.0:
        return SeverityHigh
    case score >= 4.0:
        return SeverityMedium
    case score > 0.0:
        return SeverityLow
    default:
        return SeverityInfo
    }
}

// parseLLMSeverity parses the LLM response into a SeverityLevel.
func parseLLMSeverity(s string) SeverityLevel {
    switch strings.ToUpper(strings.TrimSpace(s)) {
    case "CRITICAL":
        return SeverityCritical
    case "HIGH":
        return SeverityHigh
    case "LOW":
        return SeverityLow
    case "INFO", "INFORMATIONAL":
        return SeverityInfo
    default:
        return SeverityMedium
    }
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}
