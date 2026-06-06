// Package mitretagger provides LLM-based MITRE ATT&CK technique tagging.
// TASK-05-04d: Use an LLM to map CVE descriptions to ATT&CK techniques and tactics.
package mitretagger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ATT&CK tactics (enterprise framework).
const (
	TacticInitialAccess     = "Initial Access"
	TacticExecution         = "Execution"
	TacticPersistence       = "Persistence"
	TacticPrivilegeEscalation = "Privilege Escalation"
	TacticDefenseEvasion    = "Defense Evasion"
	TacticCredentialAccess  = "Credential Access"
	TacticDiscovery         = "Discovery"
	TacticLateralMovement   = "Lateral Movement"
	TacticCollection        = "Collection"
	TacticCommandAndControl = "Command and Control"
	TacticExfiltration      = "Exfiltration"
	TacticImpact            = "Impact"
)

// ATTCKTag is a MITRE ATT&CK technique or tactic annotation.
type ATTCKTag struct {
	TechniqueID   string // e.g. "T1190"
	TechniqueName string // e.g. "Exploit Public-Facing Application"
	Tactic        string // e.g. "Initial Access"
	Confidence    float64 // 0.0–1.0
	Source        string  // "llm" or "rule"
}

// LLMClient is the interface for calling an LLM API.
type LLMClient interface {
	// Complete sends a prompt and returns the completion.
	Complete(ctx context.Context, prompt string) (string, error)
}

// Tagger maps CVE descriptions to ATT&CK techniques using LLM + rule-based fallback.
type Tagger struct {
	llm    LLMClient
	rules  []ruleMapping
}

// ruleMapping is a keyword-to-technique mapping for the rule-based fallback.
type ruleMapping struct {
	keywords  []string
	techniqueID string
	techniqueName string
	tactic    string
}

// defaultRules provides common CVE → ATT&CK mappings for fast/offline tagging.
var defaultRules = []ruleMapping{
	{[]string{"sql injection", "sqli"}, "T1190", "Exploit Public-Facing Application", TacticInitialAccess},
	{[]string{"remote code execution", "rce", "code execution"}, "T1190", "Exploit Public-Facing Application", TacticInitialAccess},
	{[]string{"command injection", "os command"}, "T1059", "Command and Scripting Interpreter", TacticExecution},
	{[]string{"path traversal", "directory traversal"}, "T1083", "File and Directory Discovery", TacticDiscovery},
	{[]string{"privilege escalation", "privesc", "elevated privileges"}, "T1068", "Exploitation for Privilege Escalation", TacticPrivilegeEscalation},
	{[]string{"authentication bypass", "auth bypass", "unauthorized access"}, "T1078", "Valid Accounts", TacticDefenseEvasion},
	{[]string{"credential", "password", "brute force"}, "T1110", "Brute Force", TacticCredentialAccess},
	{[]string{"cross-site scripting", "xss"}, "T1189", "Drive-by Compromise", TacticInitialAccess},
	{[]string{"ssrf", "server-side request forgery"}, "T1090", "Proxy", TacticCommandAndControl},
	{[]string{"deserialization"}, "T1059", "Command and Scripting Interpreter", TacticExecution},
	{[]string{"denial of service", "dos", "resource exhaustion"}, "T1499", "Endpoint Denial of Service", TacticImpact},
	{[]string{"information disclosure", "data exposure", "sensitive information"}, "T1213", "Data from Information Repositories", TacticCollection},
	{[]string{"cross-site request forgery", "csrf"}, "T1185", "Browser Session Hijacking", TacticCollection},
	{[]string{"open redirect"}, "T1189", "Drive-by Compromise", TacticInitialAccess},
	{[]string{"buffer overflow", "heap overflow", "stack overflow"}, "T1203", "Exploitation for Client Execution", TacticExecution},
	{[]string{"man-in-the-middle", "mitm", "certificate validation"}, "T1557", "Adversary-in-the-Middle", TacticCredentialAccess},
	{[]string{"persistence", "startup", "autorun"}, "T1547", "Boot or Logon Autostart Execution", TacticPersistence},
	{[]string{"lateral movement", "pass-the-hash", "pass-the-ticket"}, "T1550", "Use Alternate Authentication Material", TacticLateralMovement},
	{[]string{"exfiltration", "data theft"}, "T1041", "Exfiltration Over C2 Channel", TacticExfiltration},
	{[]string{"file upload", "arbitrary file"}, "T1505", "Server Software Component", TacticPersistence},
}

// LLMPromptTemplate is the prompt template for ATT&CK classification.
const LLMPromptTemplate = `You are a cybersecurity expert specializing in MITRE ATT&CK framework mapping.
Given the following CVE description, identify the most relevant ATT&CK techniques.

CVE ID: %s
Description: %s

Respond with a JSON array of ATT&CK technique annotations. Each item must have:
- technique_id: string (e.g. "T1190")
- technique_name: string (e.g. "Exploit Public-Facing Application") 
- tactic: string (one of: Initial Access, Execution, Persistence, Privilege Escalation, 
  Defense Evasion, Credential Access, Discovery, Lateral Movement, Collection, 
  Command and Control, Exfiltration, Impact)
- confidence: number between 0 and 1

Return only valid JSON, no commentary. Maximum 3 techniques.
Example: [{"technique_id":"T1190","technique_name":"Exploit Public-Facing Application","tactic":"Initial Access","confidence":0.9}]`

// NewTagger creates a new ATT&CK tagger.
// If llmClient is nil, only rule-based tagging is used.
func NewTagger(llmClient LLMClient) *Tagger {
	return &Tagger{
		llm:   llmClient,
		rules: defaultRules,
	}
}

// Tag assigns ATT&CK tags to a CVE.
// It first tries the LLM (if available) and falls back to rule-based matching.
func (t *Tagger) Tag(ctx context.Context, cveID, description string) ([]ATTCKTag, error) {
	var tags []ATTCKTag
	var llmErr error

	if t.llm != nil {
		tags, llmErr = t.tagWithLLM(ctx, cveID, description)
		if llmErr == nil && len(tags) > 0 {
			return tags, nil
		}
	}

	// Rule-based fallback
	tags = t.tagWithRules(description)

	if len(tags) == 0 && llmErr != nil {
		return nil, fmt.Errorf("llm tagging failed: %w; no rule matches found", llmErr)
	}
	return tags, nil
}

// tagWithLLM sends a prompt to the LLM and parses the JSON response.
func (t *Tagger) tagWithLLM(ctx context.Context, cveID, description string) ([]ATTCKTag, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(LLMPromptTemplate, cveID, description)
	completion, err := t.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Extract JSON from completion
	jsonStr := extractJSON(completion)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	var raw []struct {
		TechniqueID   string  `json:"technique_id"`
		TechniqueName string  `json:"technique_name"`
		Tactic        string  `json:"tactic"`
		Confidence    float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	var tags []ATTCKTag
	for _, r := range raw {
		if r.TechniqueID == "" {
			continue
		}
		tags = append(tags, ATTCKTag{
			TechniqueID:   r.TechniqueID,
			TechniqueName: r.TechniqueName,
			Tactic:        r.Tactic,
			Confidence:    r.Confidence,
			Source:        "llm",
		})
	}
	return tags, nil
}

// tagWithRules uses keyword matching against the defaultRules table.
func (t *Tagger) tagWithRules(description string) []ATTCKTag {
	lower := strings.ToLower(description)
	seen := make(map[string]bool)
	var tags []ATTCKTag

	for _, rule := range t.rules {
		matched := false
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if matched && !seen[rule.techniqueID] {
			seen[rule.techniqueID] = true
			tags = append(tags, ATTCKTag{
				TechniqueID:   rule.techniqueID,
				TechniqueName: rule.techniqueName,
				Tactic:        rule.tactic,
				Confidence:    0.7, // rule-based = medium confidence
				Source:        "rule",
			})
		}
	}
	return tags
}

// extractJSON finds the first JSON array in a string.
func extractJSON(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

// ---- Null LLM (for testing without an API) ----

// NullLLMClient always returns an error — use for offline/test mode.
type NullLLMClient struct{}

func (n *NullLLMClient) Complete(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("LLM not configured")
}
