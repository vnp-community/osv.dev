// Package register provides the user registration use case.
package register

import (
	"context"
	"fmt"
	"strings"

	"github.com/osv/auth-service/internal/domain/entity"
	domainerr "github.com/osv/auth-service/internal/domain/error"
	"github.com/osv/auth-service/internal/domain/repository"
	"github.com/osv/auth-service/internal/infrastructure/crypto"
)

// Request is the input DTO for the Register use case.
type Request struct {
	Email    string
	Username string
	Password string
}

// Response is the output DTO returned on successful registration.
type Response struct {
	UserID string
	Email  string
	Role   string
}

// UseCase orchestrates user registration.
type UseCase struct {
	userRepo repository.UserRepository
}

// NewUseCase creates a new Register use case.
func NewUseCase(userRepo repository.UserRepository) *UseCase {
	return &UseCase{userRepo: userRepo}
}

// Execute registers a new user. Returns ErrEmailAlreadyExists if email is taken.
func (uc *UseCase) Execute(ctx context.Context, req Request) (*Response, error) {
	// Normalise inputs
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	if err := validate(req); err != nil {
		return nil, err
	}

	// Hash password using Argon2id
	hashedPW, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &entity.User{
		Email:          req.Email,
		Username:       req.Username,
		HashedPassword: hashedPW,
		Role:           "user", // default role
		AuthProvider:   entity.AuthProviderLocal,
		IsActive:       true,
		IsVerified:     false, // email verification pending
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err // passes through ErrEmailAlreadyExists
	}

	return &Response{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   user.Role,
	}, nil
}

func validate(req Request) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(req.Email, "@") {
		return fmt.Errorf("invalid email format")
	}
	if req.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(req.Username) < 3 || len(req.Username) > 50 {
		return fmt.Errorf("username must be 3-50 characters")
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(req.Password) > 128 {
		return domainerr.ErrInvalidCredentials // prevent DoS via long password hash
	}
	return nil
}
