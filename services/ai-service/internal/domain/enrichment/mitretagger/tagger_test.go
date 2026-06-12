package mitretagger_test

import (
	"context"
	"strings"
	"testing"

	"github.com/osv/ai-service/internal/domain/enrichment/mitretagger"
)

func TestRuleBasedTagging_SQLInjection(t *testing.T) {
	tagger := mitretagger.NewTagger(nil) // rule-based only
	tags, err := tagger.Tag(context.Background(), "CVE-2024-1234",
		"A SQL injection vulnerability allows an attacker to execute arbitrary SQL commands")
	if err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if len(tags) == 0 {
		t.Fatal("expected at least one tag for SQL injection")
	}
	found := false
	for _, tag := range tags {
		if strings.Contains(tag.TechniqueID, "T1190") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected T1190 for SQL injection, got: %+v", tags)
	}
}

func TestRuleBasedTagging_RCE(t *testing.T) {
	tagger := mitretagger.NewTagger(nil)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0001",
		"Remote code execution vulnerability in the HTTP server component")
	if err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if len(tags) == 0 {
		t.Fatal("expected at least one tag for RCE")
	}
}

func TestRuleBasedTagging_PrivilegeEscalation(t *testing.T) {
	tagger := mitretagger.NewTagger(nil)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0002",
		"Local privilege escalation allows a non-root user to gain root access")
	if err != nil {
		t.Fatalf("Tag: %v", err)
	}
	hasPrivEsc := false
	for _, tag := range tags {
		if tag.Tactic == mitretagger.TacticPrivilegeEscalation {
			hasPrivEsc = true
		}
	}
	if !hasPrivEsc {
		t.Errorf("expected PrivilegeEscalation tactic, got: %+v", tags)
	}
}

func TestRuleBasedTagging_DoS(t *testing.T) {
	tagger := mitretagger.NewTagger(nil)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0003",
		"Denial of service via resource exhaustion in the network stack")
	if err != nil {
		t.Fatalf("Tag: %v", err)
	}
	hasDoS := false
	for _, tag := range tags {
		if tag.Tactic == mitretagger.TacticImpact {
			hasDoS = true
		}
	}
	if !hasDoS {
		t.Errorf("expected Impact tactic for DoS, got: %+v", tags)
	}
}

func TestRuleBasedTagging_NoMatch(t *testing.T) {
	tagger := mitretagger.NewTagger(nil)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0004",
		"Configuration validation issue in the settings parser with no exploitable impact")
	// No match is valid
	if err != nil && len(tags) > 0 {
		t.Error("unexpected error with matches")
	}
}

func TestRuleBasedTagging_MultipleKeywords(t *testing.T) {
	tagger := mitretagger.NewTagger(nil)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0005",
		"SQL injection and remote code execution vulnerability allows privilege escalation")
	if err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if len(tags) < 2 {
		t.Errorf("expected multiple tags for multi-vector, got %d", len(tags))
	}
}

func TestNullLLMFallsBackToRules(t *testing.T) {
	nullLLM := &mitretagger.NullLLMClient{}
	tagger := mitretagger.NewTagger(nullLLM)
	// Should fall back to rule-based tagging
	tags, _ := tagger.Tag(context.Background(), "CVE-2024-0006",
		"Cross-site scripting (XSS) vulnerability in web application")
	hasXSS := false
	for _, tag := range tags {
		if tag.Tactic == mitretagger.TacticInitialAccess {
			hasXSS = true
		}
	}
	if !hasXSS {
		t.Error("expected XSS→InitialAccess tag from rule fallback")
	}
}

func TestTagConfidence(t *testing.T) {
	tagger := mitretagger.NewTagger(nil)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0007",
		"SQL injection vulnerability allows data exfiltration")
	if err != nil {
		t.Fatalf("Tag: %v", err)
	}
	for _, tag := range tags {
		if tag.Confidence <= 0 || tag.Confidence > 1 {
			t.Errorf("confidence should be in (0,1], got %f for %s", tag.Confidence, tag.TechniqueID)
		}
		if tag.Source != "rule" && tag.Source != "llm" {
			t.Errorf("source should be 'rule' or 'llm', got %q", tag.Source)
		}
	}
}

func TestLLMResponseParsing(t *testing.T) {
	// Mock LLM that returns a valid JSON response
	mockLLM := &mockLLMClient{response: `[{"technique_id":"T1190","technique_name":"Exploit Public-Facing Application","tactic":"Initial Access","confidence":0.95}]`}
	tagger := mitretagger.NewTagger(mockLLM)
	tags, err := tagger.Tag(context.Background(), "CVE-2024-0008",
		"A vulnerability allows attackers to exploit the login endpoint")
	if err != nil {
		t.Fatalf("Tag with LLM: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag from LLM, got %d", len(tags))
	}
	if tags[0].TechniqueID != "T1190" {
		t.Errorf("TechniqueID: %s", tags[0].TechniqueID)
	}
	if tags[0].Source != "llm" {
		t.Errorf("Source: %s", tags[0].Source)
	}
}

// ---- Mock LLM ----

type mockLLMClient struct{ response string }

func (m *mockLLMClient) Complete(_ context.Context, _ string) (string, error) {
	return m.response, nil
}
