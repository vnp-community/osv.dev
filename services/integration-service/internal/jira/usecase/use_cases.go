// Package usecase implements JIRA integration use cases.
package usecase

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	natsutil "github.com/osv/shared/pkg/nats"
	"github.com/defectdojo/integration-service/internal/jira/domain"
	jira_client "github.com/defectdojo/integration-service/internal/jira/infra"
)

// ─── CreateIssue ──────────────────────────────────────────────────────────────

type CreateIssueInput struct {
	FindingID   uuid.UUID
	ProductID   uuid.UUID
	Title       string
	Description string
	Severity    string
	CVE         string
	URL         string // DefectDojo finding URL
}

type CreateIssueUseCase struct {
	configRepo  domain.JIRAConfigRepository
	mappingRepo domain.JIRAIssueMappingRepository
	eventPub    *natsutil.Publisher
}

func NewCreateIssue(cfg domain.JIRAConfigRepository, mapping domain.JIRAIssueMappingRepository, pub *natsutil.Publisher) *CreateIssueUseCase {
	return &CreateIssueUseCase{configRepo: cfg, mappingRepo: mapping, eventPub: pub}
}

// Execute finds JIRA config for the product and creates a JIRA issue.
// Returns nil, nil if JIRA is not configured for this product.
func (uc *CreateIssueUseCase) Execute(ctx context.Context, in CreateIssueInput) (*domain.JIRAIssueMapping, error) {
	// 1. Find config (product → global)
	cfg, err := uc.configRepo.FindForProduct(ctx, in.ProductID)
	if err != nil {
		cfg, err = uc.configRepo.FindGlobal(ctx)
		if err != nil || !cfg.IsEnabled {
			return nil, nil // no config = skip silently
		}
	}
	if !cfg.IsEnabled {
		return nil, nil
	}

	// 2. Decrypt API token
	apiToken, err := decryptToken(cfg.APITokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("jira: decrypt api token: %w", err)
	}

	// 3. Build JIRA client
	client := jira_client.New(cfg.URL, cfg.Username, apiToken)

	// 4. Map severity to JIRA priority
	priority := cfg.IssuePriority[in.Severity]
	if priority == "" {
		priority = "Medium"
	}

	// 5. Create issue
	desc := fmt.Sprintf("[DefectDojo] %s\n\nCVE: %s\nSeverity: %s\n\n%s\n\nView: %s",
		in.Title, in.CVE, in.Severity, in.Description, in.URL)

	resp, err := client.CreateIssue(ctx, jira_client.CreateIssueRequest{
		Fields: jira_client.IssueFields{
			Project:     map[string]string{"key": cfg.ProjectKey},
			Summary:     fmt.Sprintf("[DefectDojo][%s] %s", in.Severity, in.Title),
			Description: jira_client.ADFDescription(desc),
			IssueType:   map[string]string{"name": cfg.IssueType},
			Priority:    map[string]string{"name": priority},
			Labels:      cfg.Labels,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create jira issue: %w", err)
	}

	// 6. Save mapping
	mapping := &domain.JIRAIssueMapping{
		ID:           uuid.New(),
		FindingID:    in.FindingID,
		JIRAConfigID: cfg.ID,
		JIRAIssueID:  resp.ID,
		JIRAKey:      resp.Key,
		JIRAURL:      cfg.URL + "/browse/" + resp.Key,
		LastSyncedAt: time.Now().UTC(),
		CreatedAt:    time.Now().UTC(),
	}
	if err := uc.mappingRepo.Save(ctx, mapping); err != nil {
		return nil, err
	}

	// 7. Publish event
	_ = uc.eventPub.Publish(ctx, "defectdojo.jira.issue.created", map[string]interface{}{
		"finding_id": in.FindingID,
		"jira_key":   resp.Key,
		"jira_url":   mapping.JIRAURL,
		"product_id": in.ProductID,
	})

	return mapping, nil
}

// ─── SyncIssueStatus ─────────────────────────────────────────────────────────

type SyncIssueStatusUseCase struct {
	mappingRepo domain.JIRAIssueMappingRepository
	eventPub    *natsutil.Publisher
}

func NewSyncIssueStatus(mapping domain.JIRAIssueMappingRepository, pub *natsutil.Publisher) *SyncIssueStatusUseCase {
	return &SyncIssueStatusUseCase{mappingRepo: mapping, eventPub: pub}
}

// OnJIRAWebhook processes an incoming JIRA webhook event.
func (uc *SyncIssueStatusUseCase) OnJIRAWebhook(ctx context.Context, issueKey, newStatus string) error {
	mapping, err := uc.mappingRepo.FindByJIRAKey(ctx, issueKey)
	if err != nil {
		return nil // no mapping found, ignore
	}

	if err := uc.mappingRepo.UpdateStatus(ctx, mapping.ID, newStatus); err != nil {
		return err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.jira.issue.updated", map[string]interface{}{
		"finding_id": mapping.FindingID,
		"jira_key":   issueKey,
		"new_status": newStatus,
	})
	return nil
}

// ─── AES-256 token decryption ─────────────────────────────────────────────────

// decryptToken decrypts an AES-256-GCM encrypted API token.
// The encryption key is loaded from the JIRA_ENCRYPTION_KEY env var.
func decryptToken(encrypted string) (string, error) {
	key := []byte(os.Getenv("JIRA_ENCRYPTION_KEY")) // must be 32 bytes
	if len(key) != 32 {
		return "", fmt.Errorf("JIRA_ENCRYPTION_KEY must be exactly 32 bytes")
	}

	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
