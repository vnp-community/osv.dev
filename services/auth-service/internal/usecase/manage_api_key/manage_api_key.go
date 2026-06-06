// Package manageapikey provides use cases for creating, listing, and revoking API keys.
package manageapikey

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osv/auth-service/internal/domain/entity"
	domainerr "github.com/osv/auth-service/internal/domain/error"
	"github.com/osv/auth-service/internal/domain/repository"
	"github.com/osv/auth-service/internal/domain/valueobject"
	"github.com/osv/auth-service/internal/infrastructure/crypto"
)

// CreateRequest is the input for creating a new API key.
type CreateRequest struct {
	UserID      uuid.UUID
	Name        string
	Permissions []string
	ExpiresAt   *time.Time
}

// CreateResponse contains the newly created API key.
// The FullKey is ONLY returned once — show to user immediately.
type CreateResponse struct {
	FullKey   string
	KeyID     string
	Prefix    string
	Name      string
	ExpiresAt *time.Time
}

// UseCase manages API key lifecycle.
type UseCase struct {
	apiKeyRepo repository.APIKeyRepository
}

// NewUseCase creates a new API key management use case.
func NewUseCase(apiKeyRepo repository.APIKeyRepository) *UseCase {
	return &UseCase{apiKeyRepo: apiKeyRepo}
}

// CreateAPIKey generates a new API key and stores only its hash.
func (uc *UseCase) CreateAPIKey(ctx context.Context, req CreateRequest) (*CreateResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("key name is required")
	}

	perms := req.Permissions
	if len(perms) == 0 {
		perms = valueobject.PermissionsFor("agent")
	}

	fullKey, prefix, hash, err := crypto.GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	apiKey := &entity.APIKey{
		UserID:      req.UserID,
		Name:        req.Name,
		KeyHash:     hash,
		Prefix:      prefix,
		Permissions: perms,
		ExpiresAt:   req.ExpiresAt,
	}
	if err := uc.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, err
	}

	return &CreateResponse{
		FullKey:   fullKey,
		KeyID:     apiKey.ID.String(),
		Prefix:    prefix,
		Name:      req.Name,
		ExpiresAt: req.ExpiresAt,
	}, nil
}

// ListAPIKeys returns all API keys for the given user.
func (uc *UseCase) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]*entity.APIKey, error) {
	return uc.apiKeyRepo.ListByUserID(ctx, userID)
}

// RevokeAPIKey revokes an API key by ID, verifying ownership.
func (uc *UseCase) RevokeAPIKey(ctx context.Context, keyID, userID uuid.UUID) error {
	keys, err := uc.apiKeyRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k.ID == keyID {
			return uc.apiKeyRepo.Revoke(ctx, keyID)
		}
	}
	return domainerr.ErrAPIKeyNotFound
}
