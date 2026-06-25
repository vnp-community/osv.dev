// Package subscriber implements the NATS event subscriber for the audit service.
// It subscribes to all auditable subjects and persists events to audit_events.
package subscriber

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/osv/audit-service/internal/domain/event"
	"github.com/osv/audit-service/internal/infra/crypto"
	"github.com/rs/zerolog/log"
)

// AuditSubscriber subscribes to all auditable NATS subjects.
type AuditSubscriber struct {
	nc       *nats.Conn
	repo     event.Repository
	hmac     *crypto.HMACSvc
	subs     []*nats.Subscription
}

// New creates a new AuditSubscriber.
func New(nc *nats.Conn, repo event.Repository, hmac *crypto.HMACSvc) *AuditSubscriber {
	return &AuditSubscriber{nc: nc, repo: repo, hmac: hmac}
}

// Start subscribes to all auditable subjects.
func (s *AuditSubscriber) Start(ctx context.Context) error {
	for _, subject := range event.AuditableSubjects {
		sub, err := s.nc.QueueSubscribe(subject, "audit-service", s.handleMessage(subject))
		if err != nil {
			log.Error().Err(err).Str("subject", subject).Msg("failed to subscribe")
			return err
		}
		s.subs = append(s.subs, sub)
		log.Debug().Str("subject", subject).Msg("subscribed")
	}
	log.Info().Int("subjects", len(s.subs)).Msg("audit subscriber started")

	<-ctx.Done()
	return s.stop()
}

func (s *AuditSubscriber) stop() error {
	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}
	return nil
}

// handleMessage processes each NATS message and records it as an audit event.
func (s *AuditSubscriber) handleMessage(subject string) nats.MsgHandler {
	return func(msg *nats.Msg) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Parse payload
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Warn().Err(err).Str("subject", subject).Msg("failed to parse audit message")
			return
		}

		// Extract resource_id (try common field names)
		resourceID := extractUUID(payload, "finding_id", "product_id", "engagement_id",
			"test_id", "risk_acceptance_id", "user_id", "id")

		// Extract actor
		actorID := extractUUIDOpt(payload, "actor_id", "user_id", "requestor_id")
		serviceName := extractString(payload, "_service", "service_name")

		// Build audit event
		ae := event.New(
			subject,
			event.SubjectToResourceType(subject),
			event.SubjectToAction(subject),
			resourceID,
		)
		ae.ActorID = actorID
		ae.ServiceName = serviceName
		ae.ActorType = actorType(payload, actorID)
		ae.Changes = payload
		ae.Signature = s.hmac.Sign(ae.SignablePayload())

		if msgID := msg.Header.Get("Nats-Msg-Id"); msgID != "" {
			ae.EventID = msgID
		}

		if err := s.repo.Create(ctx, ae); err != nil {
			log.Error().Err(err).Str("subject", subject).Msg("failed to record audit event")
			return
		}
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func extractUUID(payload map[string]interface{}, keys ...string) uuid.UUID {
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok {
				if id, err := uuid.Parse(s); err == nil {
					return id
				}
			}
		}
	}
	return uuid.Nil
}

func extractUUIDOpt(payload map[string]interface{}, keys ...string) *uuid.UUID {
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok {
				if id, err := uuid.Parse(s); err == nil {
					cp := id
					return &cp
				}
			}
		}
	}
	return nil
}

func extractString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func actorType(payload map[string]interface{}, actorID *uuid.UUID) string {
	if svc := extractString(payload, "_service"); svc != "" {
		return "service"
	}
	if actorID == nil {
		return "system"
	}
	return "user"
}
