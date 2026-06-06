package webhook_test

import (
	"strings"
	"testing"
	"time"

	"github.com/globalcve/notification-service/internal/domain/aggregate/webhook"
)

func TestNew_ValidWebhook(t *testing.T) {
	w, err := webhook.New("https://example.com/hook", []string{"cve.created"}, "mysecret", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if w.ID() == "" {
		t.Error("expected non-empty ID")
	}
	if !w.IsActive() {
		t.Error("expected webhook to be active")
	}
	if w.OwnerID() != "user-1" {
		t.Errorf("expected ownerID=user-1, got %s", w.OwnerID())
	}
}

func TestNew_InvalidURL(t *testing.T) {
	_, err := webhook.New("http://example.com/hook", []string{"cve.created"}, "mysecret", "user-1")
	if err == nil {
		t.Fatal("expected error for HTTP URL")
	}
}

func TestNew_EmptyEvents(t *testing.T) {
	_, err := webhook.New("https://example.com/hook", nil, "mysecret", "user-1")
	if err == nil {
		t.Fatal("expected error for empty events")
	}
}

func TestSign(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", []string{"cve.created"}, "s3cr3t_32_bytes_long_secret_here!!", "user-1")
	sig := w.Sign([]byte(`{"event":"cve.created"}`))
	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("expected signature to start with sha256=, got %s", sig)
	}
}

func TestShouldDeliver(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", []string{"cve.created", "kev.added"}, "s3cr3t", "user-1")
	if !w.ShouldDeliver("cve.created") {
		t.Error("expected ShouldDeliver=true for cve.created")
	}
	if w.ShouldDeliver("sync.completed") {
		t.Error("expected ShouldDeliver=false for sync.completed")
	}
}

func TestDeactivate(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", []string{"cve.created"}, "s3cr3t", "user-1")
	w.Deactivate()
	if w.IsActive() {
		t.Error("expected inactive after Deactivate()")
	}
	if w.ShouldDeliver("cve.created") {
		t.Error("inactive webhook should not deliver")
	}
}

func TestReconstitute(t *testing.T) {
	ts := time.Now().UTC()
	w := webhook.Reconstitute("id-1", "owner-1", "https://a.com", []string{"kev.added"}, "sec", true, ts, ts)
	if w.ID() != "id-1" {
		t.Errorf("expected id=id-1, got %s", w.ID())
	}
}
