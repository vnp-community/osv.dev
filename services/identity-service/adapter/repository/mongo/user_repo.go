// Package mongo provides MongoDB-backed repositories for auth-service.
// Used by cve-search deployments where user accounts are stored in MongoDB.
package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/osv/identity-service/internal/domain/entity"
	domainerr "github.com/osv/identity-service/internal/domain/error"
)

// userDocument is the MongoDB wire format for a User.
// Uses string for UUID fields (MongoDB does not have a native UUID type).
type userDocument struct {
	ID                  string     `bson:"_id"`
	Email               string     `bson:"email"`
	Username            string     `bson:"username"`
	HashedPassword      string     `bson:"hashed_password"`
	Role                string     `bson:"role"`
	AuthProvider        string     `bson:"auth_provider"`
	MFAEnabled          bool       `bson:"mfa_enabled"`
	MFATOTPSecret       string     `bson:"mfa_totp_secret"`
	IsActive            bool       `bson:"is_active"`
	IsVerified          bool       `bson:"is_verified"`
	FailedLoginAttempts int        `bson:"failed_login_attempts"`
	LastLoginAt         *time.Time `bson:"last_login_at,omitempty"`
	CreatedAt           time.Time  `bson:"created_at"`
	UpdatedAt           time.Time  `bson:"updated_at"`
}

func toDocument(u *entity.User) *userDocument {
	mfaSecret := ""
	if u.MFATOTPSecret != nil {
		mfaSecret = *u.MFATOTPSecret
	}
	doc := &userDocument{
		ID:                  u.ID.String(),
		Email:               u.Email,
		Username:            u.Username,
		HashedPassword:      u.HashedPassword,
		Role:                u.Role,
		AuthProvider:        string(u.AuthProvider),
		MFAEnabled:          u.MFAEnabled,
		MFATOTPSecret:       mfaSecret, // *string → string (nil → "")
		IsActive:            u.IsActive,
		IsVerified:          u.IsVerified,
		FailedLoginAttempts: u.FailedLoginAttempts,
		LastLoginAt:         u.LastLoginAt,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
	}
	return doc
}

func toEntity(doc *userDocument) (*entity.User, error) {
	id, err := uuid.Parse(doc.ID)
	if err != nil {
		return nil, fmt.Errorf("parse user UUID %q: %w", doc.ID, err)
	}
	var mfaSecret *string
	if doc.MFATOTPSecret != "" {
		s := doc.MFATOTPSecret
		mfaSecret = &s
	}
	return &entity.User{
		ID:                  id,
		Email:               doc.Email,
		Username:            doc.Username,
		HashedPassword:      doc.HashedPassword,
		Role:                doc.Role,
		AuthProvider:        entity.AuthProvider(doc.AuthProvider),
		MFAEnabled:          doc.MFAEnabled,
		MFATOTPSecret:       mfaSecret, // string → *string ("" → nil)
		IsActive:            doc.IsActive,
		IsVerified:          doc.IsVerified,
		FailedLoginAttempts: doc.FailedLoginAttempts,
		LastLoginAt:         doc.LastLoginAt,
		CreatedAt:           doc.CreatedAt,
		UpdatedAt:           doc.UpdatedAt,
	}, nil
}

// UserRepo implements repository.UserRepository backed by MongoDB.
// Collection: "mgmt_users" in the cvedb database.
type UserRepo struct {
	col *mongo.Collection
}

// NewUserRepo creates a UserRepo.
func NewUserRepo(db *mongo.Database) *UserRepo {
	return &UserRepo{col: db.Collection("mgmt_users")}
}

// EnsureIndexes creates required indexes. Idempotent — safe to call on every startup.
func (r *UserRepo) EnsureIndexes(ctx context.Context) error {
	_, err := r.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("email_unique"),
		},
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("username_unique"),
		},
	})
	if isDuplicateIndexErr(err) {
		return nil
	}
	return err
}

// Create inserts a new user. Returns ErrEmailAlreadyExists on duplicate email.
func (r *UserRepo) Create(ctx context.Context, u *entity.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now

	doc := toDocument(u)
	_, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domainerr.ErrEmailAlreadyExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// FindByID returns a user by UUID string.
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	return r.findOne(ctx, bson.M{"_id": id.String()})
}

// FindByEmail returns a user by email (case-insensitive collation).
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	return r.findOne(ctx, bson.M{"email": bson.M{"$regex": "^" + email + "$", "$options": "i"}})
}

// FindByUsername returns a user by username.
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*entity.User, error) {
	return r.findOne(ctx, bson.M{"username": username})
}

// Update saves all mutable user fields.
func (r *UserRepo) Update(ctx context.Context, u *entity.User) error {
	u.UpdatedAt = time.Now().UTC()
	doc := toDocument(u)
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": u.ID.String()}, doc)
	if err != nil {
		return fmt.Errorf("update user %s: %w", u.ID, err)
	}
	return nil
}

// UpdateLastLogin sets last_login_at to now and resets failed_login_attempts.
func (r *UserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id.String()},
		bson.M{"$set": bson.M{
			"last_login_at":         now,
			"failed_login_attempts": 0,
			"updated_at":            now,
		}},
	)
	return err
}

// Delete removes a user document.
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id.String()})
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (r *UserRepo) findOne(ctx context.Context, filter bson.M) (*entity.User, error) {
	var doc userDocument
	err := r.col.FindOne(ctx, filter).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, domainerr.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("findOne user: %w", err)
	}
	return toEntity(&doc)
}

func isDuplicateIndexErr(err error) bool {
	if err == nil {
		return false
	}
	// MongoDB error codes 85 (IndexOptionsConflict) or 86 (IndexKeySpecsConflict)
	if we, ok := err.(mongo.WriteException); ok {
		for _, e := range we.WriteErrors {
			if e.Code == 85 || e.Code == 86 {
				return true
			}
		}
	}
	return false
}
