// Package usecase implements the RecordEvent use case for the audit service.
package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	natsutil "github.com/osv/shared/pkg/nats"
	audit "github.com/defectdojo/finding-service/internal/domain/audit"
)

// RecordEventUseCase persists every DefectDojo event as an immutable audit record.
type RecordEventUseCase struct {
	repo audit.Repository
}

func NewRecordEvent(repo audit.Repository) *RecordEventUseCase {
	return &RecordEventUseCase{repo: repo}
}

// RecordEvent parses a CloudEvent and saves an AuditEvent.
func (uc *RecordEventUseCase) RecordEvent(ctx context.Context, event *natsutil.CloudEvent) error {
	entityType, entityID := extractEntity(event.Type, event.Data)

	var payload map[string]interface{}
	_ = json.Unmarshal(event.Data, &payload)
	actorID := extractActorID(payload)

	audit := &audit.AuditEvent{
		ID:         uuid.New(),
		EventType:  event.Type,
		EntityType: entityType,
		EntityID:   entityID,
		ActorID:    actorID,
		NewState:   event.Data,
		OccurredAt: event.Time,
		CreatedAt:  time.Now().UTC(),
	}
	return uc.repo.Save(ctx, audit)
}

// extractEntity derives entity_type and entity_id from the event type + payload.
// Event types follow: "defectdojo.<entity_type>.<action>"
func extractEntity(eventType string, data json.RawMessage) (entityType, entityID string) {
	parts := strings.Split(eventType, ".")
	if len(parts) >= 2 {
		entityType = parts[1] // e.g., "finding", "product", "engagement"
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return entityType, ""
	}

	// Try common entity ID keys in order of priority
	for _, key := range []string{
		entityType + "_id",
		"finding_id", "product_id", "engagement_id", "test_id", "user_id",
		"id",
	} {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return entityType, s
			}
		}
	}
	return entityType, ""
}

func extractActorID(payload map[string]interface{}) string {
	for _, key := range []string{"requestor_user_id", "user_id", "mitigated_by_id", "actor_id"} {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return "system"
}
