// Package tool_usecase implements CRUD use cases for ToolConfiguration entities.
package tool_usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/tool"
	"github.com/osv/finding-service/internal/infra/crypto"
)

var (
	// ErrNotFound is returned when a ToolConfiguration does not exist.
	ErrNotFound = errors.New("tool configuration not found")
	// ErrNameRequired is returned when the name is empty.
	ErrNameRequired = errors.New("tool configuration name is required")
)

// ─── CreateToolConfig ─────────────────────────────────────────────────────────

// CreateToolConfigInput is the request for creating a new ToolConfiguration.
type CreateToolConfigInput struct {
	Name        string
	Description string
	ToolType    string
	URL         string
	AuthType    tool.AuthType
	Username    string
	Password    string // plaintext — encrypted before save
	APIKey      string // plaintext — encrypted before save
}

// CreateToolConfigUseCase creates a new ToolConfiguration with encrypted credentials.
type CreateToolConfigUseCase struct {
	toolRepo tool.ToolConfigurationRepository
	crypto   *crypto.AES256GCM
}

// NewCreateToolConfig creates a new CreateToolConfigUseCase.
func NewCreateToolConfig(repo tool.ToolConfigurationRepository, c *crypto.AES256GCM) *CreateToolConfigUseCase {
	return &CreateToolConfigUseCase{toolRepo: repo, crypto: c}
}

// Execute creates a ToolConfiguration, encrypting password and API key.
func (uc *CreateToolConfigUseCase) Execute(ctx context.Context, in CreateToolConfigInput) (*tool.ToolConfiguration, error) {
	if in.Name == "" {
		return nil, ErrNameRequired
	}
	if !in.AuthType.IsValid() {
		return nil, errors.New("invalid auth type")
	}

	passwordEnc, err := uc.crypto.Encrypt(in.Password)
	if err != nil {
		return nil, err
	}
	apiKeyEnc, err := uc.crypto.Encrypt(in.APIKey)
	if err != nil {
		return nil, err
	}

	tc := tool.New(in.Name, in.ToolType, in.URL, in.AuthType)
	tc.Description = in.Description
	tc.Username = in.Username
	tc.PasswordEnc = passwordEnc
	tc.APIKeyEnc = apiKeyEnc

	if err := uc.toolRepo.Save(ctx, tc); err != nil {
		return nil, err
	}
	return tc, nil
}

// ─── UpdateToolConfig ─────────────────────────────────────────────────────────

// UpdateToolConfigInput is the request for updating a ToolConfiguration.
type UpdateToolConfigInput struct {
	ID          uuid.UUID
	Name        string
	Description string
	ToolType    string
	URL         string
	AuthType    tool.AuthType
	Username    string
	Password    string // empty = keep existing
	APIKey      string // empty = keep existing
}

// UpdateToolConfigUseCase updates an existing ToolConfiguration.
type UpdateToolConfigUseCase struct {
	toolRepo tool.ToolConfigurationRepository
	crypto   *crypto.AES256GCM
}

// NewUpdateToolConfig creates a new UpdateToolConfigUseCase.
func NewUpdateToolConfig(repo tool.ToolConfigurationRepository, c *crypto.AES256GCM) *UpdateToolConfigUseCase {
	return &UpdateToolConfigUseCase{toolRepo: repo, crypto: c}
}

// Execute updates a ToolConfiguration, re-encrypting credentials only if provided.
func (uc *UpdateToolConfigUseCase) Execute(ctx context.Context, in UpdateToolConfigInput) (*tool.ToolConfiguration, error) {
	existing, err := uc.toolRepo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, ErrNotFound
	}

	if in.Name != "" {
		existing.Name = in.Name
	}
	existing.Description = in.Description
	if in.ToolType != "" {
		existing.ToolType = in.ToolType
	}
	if in.URL != "" {
		existing.URL = in.URL
	}
	if in.AuthType.IsValid() {
		existing.AuthType = in.AuthType
	}
	if in.Username != "" {
		existing.Username = in.Username
	}
	// Only re-encrypt if new credentials provided
	if in.Password != "" {
		enc, err := uc.crypto.Encrypt(in.Password)
		if err != nil {
			return nil, err
		}
		existing.PasswordEnc = enc
	}
	if in.APIKey != "" {
		enc, err := uc.crypto.Encrypt(in.APIKey)
		if err != nil {
			return nil, err
		}
		existing.APIKeyEnc = enc
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := uc.toolRepo.Save(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// ─── DeleteToolConfig ─────────────────────────────────────────────────────────

// DeleteToolConfigUseCase deletes a ToolConfiguration by ID.
type DeleteToolConfigUseCase struct {
	toolRepo tool.ToolConfigurationRepository
}

// NewDeleteToolConfig creates a new DeleteToolConfigUseCase.
func NewDeleteToolConfig(repo tool.ToolConfigurationRepository) *DeleteToolConfigUseCase {
	return &DeleteToolConfigUseCase{toolRepo: repo}
}

// Execute deletes a ToolConfiguration.
func (uc *DeleteToolConfigUseCase) Execute(ctx context.Context, id uuid.UUID) error {
	_, err := uc.toolRepo.FindByID(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	return uc.toolRepo.Delete(ctx, id)
}

// ─── GetToolConfig ─────────────────────────────────────────────────────────────

// GetToolConfigUseCase retrieves a ToolConfiguration — masking encrypted credentials.
type GetToolConfigUseCase struct {
	toolRepo tool.ToolConfigurationRepository
}

// NewGetToolConfig creates a new GetToolConfigUseCase.
func NewGetToolConfig(repo tool.ToolConfigurationRepository) *GetToolConfigUseCase {
	return &GetToolConfigUseCase{toolRepo: repo}
}

// Execute returns a ToolConfiguration with credentials masked.
func (uc *GetToolConfigUseCase) Execute(ctx context.Context, id uuid.UUID) (*tool.ToolConfiguration, error) {
	tc, err := uc.toolRepo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	// Never expose encrypted credentials in API responses
	tc.PasswordEnc = "***"
	tc.APIKeyEnc = "***"
	return tc, nil
}

// ListToolConfigsUseCase returns all ToolConfigurations with masked credentials.
type ListToolConfigsUseCase struct {
	toolRepo tool.ToolConfigurationRepository
}

// NewListToolConfigs creates a new ListToolConfigsUseCase.
func NewListToolConfigs(repo tool.ToolConfigurationRepository) *ListToolConfigsUseCase {
	return &ListToolConfigsUseCase{toolRepo: repo}
}

// Execute returns all ToolConfigurations with credentials masked.
func (uc *ListToolConfigsUseCase) Execute(ctx context.Context) ([]*tool.ToolConfiguration, error) {
	tools, err := uc.toolRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, tc := range tools {
		tc.PasswordEnc = "***"
		tc.APIKeyEnc = "***"
	}
	return tools, nil
}
