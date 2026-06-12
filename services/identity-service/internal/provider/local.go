// Package provider — Local provider authenticates via bcrypt-hashed passwords in MongoDB.
package provider

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/osv/identity-service/internal/domain/repository"
)

// LocalProvider authenticates users against bcrypt-hashed passwords.
// Backs the "local" entry in the auth chain spec.
type LocalProvider struct {
	users repository.UserRepository
}

// NewLocalProvider creates a LocalProvider backed by the given user repository.
// Works with both PostgreSQL and MongoDB implementations of UserRepository.
func NewLocalProvider(users repository.UserRepository) Provider {
	return &LocalProvider{users: users}
}

// Name returns "local".
func (p *LocalProvider) Name() string { return "local" }

// Authenticate verifies username + bcrypt password against the user repository.
// Returns AuthWrongCreds (not an error) for any user-facing failure to prevent
// user enumeration attacks.
func (p *LocalProvider) Authenticate(ctx context.Context, username, password string) (AuthResult, error) {
	user, err := p.users.FindByUsername(ctx, username)
	if err != nil {
		// User not found → treat as wrong credentials, never reveal existence
		return AuthWrongCreds, nil
	}

	if !user.IsActive {
		return AuthWrongCreds, nil
	}

	if user.HashedPassword == "" {
		// OAuth-only account — no local password
		return AuthWrongCreds, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password)); err != nil {
		return AuthWrongCreds, nil
	}

	return AuthOK, nil
}

// HashPassword creates a bcrypt hash suitable for storage.
// Cost factor 10 balances security and performance (approx. 100ms on modern hardware).
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}
