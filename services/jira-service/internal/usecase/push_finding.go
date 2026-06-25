// Package pushjira implements the use case for pushing a finding to JIRA.
// Called when a finding is created or when the user manually requests JIRA push.
package pushjira

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/google/uuid"
	"github.com/osv/jira-service/internal/domain/jiraconfig"
)

var (
	ErrNoJIRAConfig  = errors.New("no JIRA configuration found for this product")
	ErrAlreadyPushed = errors.New("finding already has a JIRA issue")
)

// FindingData is the minimum finding data needed to create a JIRA issue.
type FindingData struct {
	ID          uuid.UUID
	ProductID   uuid.UUID
	Title       string
	Description string
	Severity    string
	CVE         string
	FilePath    string
	URL         string
}

// JIRAIssueMapping records the link between a finding and a JIRA issue.
type JIRAIssueMapping struct {
	ID         uuid.UUID
	FindingID  uuid.UUID
	JIRAID     string
	JIRAKey    string
	JIRAURL    string
	JIRAStatus string
	Synced     bool
	CreatedAt  time.Time
}

// ConfigRepository provides JIRA configs by product.
type ConfigRepository interface {
	GetByProductID(ctx context.Context, productID uuid.UUID) (*jiraconfig.JIRAConfig, error)
}

// MappingRepository persists finding ↔ JIRA issue links.
type MappingRepository interface {
	Save(ctx context.Context, m *JIRAIssueMapping) error
	FindByFindingID(ctx context.Context, findingID uuid.UUID) (*JIRAIssueMapping, error)
}

// CryptoService decrypts the JIRA password.
type CryptoService interface {
	Decrypt(ciphertext string) (string, error)
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// PushFindingToJIRAUseCase creates a JIRA issue for a finding.
type PushFindingToJIRAUseCase struct {
	configRepo  ConfigRepository
	mappingRepo MappingRepository
	crypto      CryptoService
	eventPub    EventPublisher
}

// New creates a PushFindingToJIRAUseCase.
func New(cr ConfigRepository, mr MappingRepository, c CryptoService, ep EventPublisher) *PushFindingToJIRAUseCase {
	return &PushFindingToJIRAUseCase{
		configRepo:  cr,
		mappingRepo: mr,
		crypto:      c,
		eventPub:    ep,
	}
}

// Execute creates a JIRA issue for the finding.
// Returns error if:
// - no JIRA config for the product
// - finding already has a JIRA mapping (and config.EnableDeduplication=true)
// - JIRA API returns an error
func (uc *PushFindingToJIRAUseCase) Execute(ctx context.Context, f FindingData) (*JIRAIssueMapping, error) {
	// 1. Load JIRA config for the product
	cfg, err := uc.configRepo.GetByProductID(ctx, f.ProductID)
	if err != nil {
		return nil, ErrNoJIRAConfig
	}
	if !cfg.IsActive {
		return nil, errors.New("JIRA integration is not active for this product")
	}

	// 2. Check for existing mapping (dedup)
	if cfg.EnableDeduplication {
		existing, _ := uc.mappingRepo.FindByFindingID(ctx, f.ID)
		if existing != nil {
			return nil, ErrAlreadyPushed
		}
	}

	// 3. Decrypt JIRA token
	password, err := uc.crypto.Decrypt(cfg.PasswordEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypting JIRA credentials: %w", err)
	}

	// 4. Create JIRA client
	tp := cloud.BasicAuthTransport{
		Username: cfg.Username,
		APIToken: password,
	}
	jiraClient, err := cloud.NewClient(cfg.URL, tp.Client())
	if err != nil {
		return nil, fmt.Errorf("creating JIRA client: %w", err)
	}

	// 5. Build JIRA issue
	issueFields := &cloud.IssueFields{
		Summary: truncate(fmt.Sprintf("[%s] %s — %s", f.Severity, f.Title, f.CVE), 255),
		Project: cloud.Project{Key: cfg.ProjectKey},
		Type:    cloud.IssueType{ID: cfg.IssueTypeID},
		Description: &cloud.CommentVisibility{
			Value: buildDescription(f),
		},
		Priority: &cloud.Priority{
			Name: cfg.JIRAPriority(f.Severity),
		},
	}
	if cfg.DefaultAssignee != "" {
		issueFields.Assignee = &cloud.User{EmailAddress: cfg.DefaultAssignee}
	}

	jiraIssue, resp, err := jiraClient.Issue.Create(ctx, &cloud.Issue{Fields: issueFields})
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("JIRA API error (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("JIRA API error: %w", err)
	}

	// 6. Persist mapping
	mapping := &JIRAIssueMapping{
		ID:         uuid.New(),
		FindingID:  f.ID,
		JIRAID:     jiraIssue.ID,
		JIRAKey:    jiraIssue.Key,
		JIRAURL:    fmt.Sprintf("%s/browse/%s", cfg.URL, jiraIssue.Key),
		JIRAStatus: "To Do",
		Synced:     true,
		CreatedAt:  time.Now().UTC(),
	}
	if err := uc.mappingRepo.Save(ctx, mapping); err != nil {
		return nil, fmt.Errorf("saving JIRA mapping: %w", err)
	}

	// 7. Publish event
	_ = uc.eventPub.Publish(ctx, "defectdojo.jira.issue.created", map[string]any{
		"finding_id": f.ID.String(),
		"product_id": f.ProductID.String(),
		"jira_key":   jiraIssue.Key,
		"jira_url":   mapping.JIRAURL,
	})

	return mapping, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func buildDescription(f FindingData) string {
	desc := fmt.Sprintf("*Severity*: %s\n\n", f.Severity)
	if f.CVE != "" {
		desc += fmt.Sprintf("*CVE*: %s\n\n", f.CVE)
	}
	if f.FilePath != "" {
		desc += fmt.Sprintf("*File*: %s\n\n", f.FilePath)
	}
	if f.Description != "" {
		desc += fmt.Sprintf("*Description*:\n%s\n", f.Description)
	}
	return desc
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
