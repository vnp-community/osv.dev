// Package mongo provides MongoDB-backed session repository for auth-service.
package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/osv/auth-service/internal/domain/entity"
	domainerr "github.com/osv/auth-service/internal/domain/error"
)

// sessionDocument is the MongoDB wire format for a Session.
type sessionDocument struct {
	ID               string    `bson:"_id"`
	UserID           string    `bson:"user_id"`
	RefreshTokenHash string    `bson:"refresh_token_hash"`
	TokenFamily      string    `bson:"token_family"`
	IPAddress        string    `bson:"ip_address"`
	UserAgent        string    `bson:"user_agent"`
	ExpiresAt        time.Time `bson:"expires_at"`
	RevokedAt        *time.Time `bson:"revoked_at,omitempty"`
	CreatedAt        time.Time `bson:"created_at"`
}

// SessionRepo implements repository.SessionRepository backed by MongoDB.
// Collection: "mgmt_sessions" in the cvedb database.
type SessionRepo struct {
	col *mongo.Collection
}

// NewSessionRepo creates a SessionRepo.
func NewSessionRepo(db *mongo.Database) *SessionRepo {
	return &SessionRepo{col: db.Collection("mgmt_sessions")}
}

// EnsureIndexes creates TTL + lookup indexes.
func (r *SessionRepo) EnsureIndexes(ctx context.Context) error {
	_, err := r.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// TTL index: MongoDB auto-deletes expired sessions
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("session_ttl"),
		},
		// Lookup by token hash
		{
			Keys:    bson.D{{Key: "refresh_token_hash", Value: 1}},
			Options: options.Index().SetName("token_hash_idx"),
		},
		// Revoke all for user
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetName("user_id_idx"),
		},
		// Token family for replay detection
		{
			Keys:    bson.D{{Key: "token_family", Value: 1}},
			Options: options.Index().SetName("token_family_idx"),
		},
	})
	if isDuplicateIndexErr(err) {
		return nil
	}
	return err
}

// Create inserts a new session.
func (r *SessionRepo) Create(ctx context.Context, s *entity.Session) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	doc := &sessionDocument{
		ID:               s.ID.String(),
		UserID:           s.UserID.String(),
		RefreshTokenHash: s.RefreshTokenHash,
		TokenFamily:      s.TokenFamily,
		IPAddress:        s.IPAddress,
		UserAgent:        s.UserAgent,
		ExpiresAt:        s.ExpiresAt,
		CreatedAt:        now,
	}
	_, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// FindByRefreshTokenHash finds an active (non-revoked, non-expired) session.
func (r *SessionRepo) FindByRefreshTokenHash(ctx context.Context, hash string) (*entity.Session, error) {
	var doc sessionDocument
	err := r.col.FindOne(ctx, bson.M{
		"refresh_token_hash": hash,
		"revoked_at":         bson.M{"$exists": false},
		"expires_at":         bson.M{"$gt": time.Now().UTC()},
	}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, domainerr.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("findByRefreshTokenHash: %w", err)
	}
	return sessionToEntity(&doc)
}

// RevokeByID marks a session as revoked.
func (r *SessionRepo) RevokeByID(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id.String()},
		bson.M{"$set": bson.M{"revoked_at": now}},
	)
	return err
}

// RevokeByFamily revokes all sessions in a token family (replay attack response).
func (r *SessionRepo) RevokeByFamily(ctx context.Context, family string) error {
	now := time.Now().UTC()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"token_family": family},
		bson.M{"$set": bson.M{"revoked_at": now}},
	)
	return err
}

// RevokeByUserID revokes all sessions for a user (logout all devices).
func (r *SessionRepo) RevokeByUserID(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"user_id": userID.String()},
		bson.M{"$set": bson.M{"revoked_at": now}},
	)
	return err
}

// CleanExpired deletes sessions past their expiry (MongoDB TTL index handles this
// automatically, but this method allows manual cleanup if needed).
func (r *SessionRepo) CleanExpired(ctx context.Context) error {
	_, err := r.col.DeleteMany(ctx, bson.M{
		"expires_at": bson.M{"$lt": time.Now().UTC()},
	})
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sessionToEntity(doc *sessionDocument) (*entity.Session, error) {
	id, err := uuid.Parse(doc.ID)
	if err != nil {
		return nil, fmt.Errorf("parse session UUID %q: %w", doc.ID, err)
	}
	userID, err := uuid.Parse(doc.UserID)
	if err != nil {
		return nil, fmt.Errorf("parse session UserID %q: %w", doc.UserID, err)
	}
	return &entity.Session{
		ID:               id,
		UserID:           userID,
		RefreshTokenHash: doc.RefreshTokenHash,
		TokenFamily:      doc.TokenFamily,
		IPAddress:        doc.IPAddress,
		UserAgent:        doc.UserAgent,
		ExpiresAt:        doc.ExpiresAt,
	}, nil
}
