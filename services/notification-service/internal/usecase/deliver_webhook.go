package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/osv/notification-service/internal/domain/repository"
	entity "github.com/osv/notification-service/internal/domain/webhook"
	"github.com/redis/go-redis/v9"
)

// Retry delays: immediate, 5min, 30min, 2h, 12h
var retryDelays = []time.Duration{0, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}

type WebhookDeliverer struct {
	webhookRepo repository.WebhookRepository
	redis       *redis.Client
	httpClient  *http.Client
}

func NewWebhookDeliverer(repo repository.WebhookRepository, redisClient *redis.Client) *WebhookDeliverer {
	return &WebhookDeliverer{
		webhookRepo: repo,
		redis:       redisClient,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

type DeliveryInput struct {
	WebhookID string
	EventType entity.EventType
	CVEID     string // for deduplication key
	Payload   map[string]interface{}
}

func (d *WebhookDeliverer) Deliver(ctx context.Context, in DeliveryInput) error {
	// 1. Deduplication: prevent same alert within 1 hour
	if d.redis != nil && d.isDuplicate(ctx, in.WebhookID, in.CVEID, in.EventType) {
		return nil // silently skip duplicate
	}

	wh, err := d.webhookRepo.FindByID(ctx, in.WebhookID, "")
	if err != nil {
		return err
	}
	if !wh.IsActive() {
		return nil
	}

	// 2. Build signed payload
	body := d.buildPayload(in.EventType, in.Payload)
	signature := d.sign(body, wh.Secret())

	// 3. Send HTTP request
	deliveryID := uuid.New().String()
	statusCode, err := d.sendRequest(ctx, wh.URL(), body, signature, deliveryID, in.EventType)

	// 4. Record delivery
	now := time.Now().UTC()
	delivery := &entity.WebhookDelivery{
		ID:        deliveryID,
		WebhookID: in.WebhookID,
		EventType: in.EventType,
		Payload:   string(body),
		Attempt:   1,
		CreatedAt: now,
	}
	if err == nil && statusCode >= 200 && statusCode < 300 {
		delivery.Status = entity.DeliveryDelivered
		delivery.StatusCode = &statusCode
		delivery.DeliveredAt = &now
	} else {
		delivery.Status = entity.DeliveryRetrying
		if statusCode > 0 {
			delivery.StatusCode = &statusCode
		}
		nextRetry := now.Add(retryDelays[1])
		delivery.NextRetryAt = &nextRetry
	}
	d.webhookRepo.SaveDelivery(ctx, delivery) //nolint:errcheck
	return err
}

func (d *WebhookDeliverer) buildPayload(eventType entity.EventType, data map[string]interface{}) []byte {
	payload := map[string]interface{}{
		"event":   string(eventType),
		"sent_at": time.Now().UTC().Format(time.RFC3339),
		"data":    data,
	}
	b, _ := json.Marshal(payload)
	return b
}

func (d *WebhookDeliverer) sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (d *WebhookDeliverer) sendRequest(ctx context.Context, url string, body []byte, sig, deliveryID string, event entity.EventType) (int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GlobalCVE-Event", string(event))
	req.Header.Set("X-GlobalCVE-Signature", sig)
	req.Header.Set("X-GlobalCVE-Delivery", deliveryID)
	req.Header.Set("User-Agent", "GlobalCVE/3.0 (+https://globalcve.xyz)")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func (d *WebhookDeliverer) isDuplicate(ctx context.Context, webhookID, cveID string, event entity.EventType) bool {
	key := fmt.Sprintf("alert:dedup:%s:%s:%s", webhookID, cveID, event)
	ok, _ := d.redis.SetNX(ctx, key, "1", 1*time.Hour).Result()
	return !ok // SetNX returns false if key already exists (duplicate)
}

// Retry a delivery with appropriate backoff.
func (d *WebhookDeliverer) Retry(ctx context.Context, delivery *entity.WebhookDelivery) {
	wh, err := d.webhookRepo.FindByID(ctx, delivery.WebhookID, "")
	if err != nil || !wh.IsActive() {
		return
	}

	var payload map[string]interface{}
	json.Unmarshal([]byte(delivery.Payload), &payload)

	body, _ := json.Marshal(payload)
	signature := d.sign(body, wh.Secret())
	deliveryID := uuid.New().String()

	statusCode, err := d.sendRequest(ctx, wh.URL(), body, signature, deliveryID, delivery.EventType)

	now := time.Now().UTC()
	delivery.Attempt++
	delivery.StatusCode = &statusCode

	if err == nil && statusCode >= 200 && statusCode < 300 {
		delivery.Status = entity.DeliveryDelivered
		delivery.DeliveredAt = &now
		delivery.NextRetryAt = nil
	} else if delivery.Attempt >= len(retryDelays) {
		delivery.Status = entity.DeliveryFailed
		delivery.NextRetryAt = nil
	} else {
		delivery.Status = entity.DeliveryRetrying
		next := now.Add(retryDelays[delivery.Attempt])
		delivery.NextRetryAt = &next
	}
	d.webhookRepo.UpdateDelivery(ctx, delivery) //nolint:errcheck
}
