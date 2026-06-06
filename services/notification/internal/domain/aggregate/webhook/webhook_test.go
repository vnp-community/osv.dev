// domain/aggregate/webhook/webhook_test.go
package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/osv/notification/internal/domain/aggregate/webhook"
)

func validSecret() []byte {
	return []byte("this-is-a-32-byte-minimum-secret!!")
}

func TestNew_RequiresHTTPS(t *testing.T) {
	_, err := webhook.New("http://example.com/hook", validSecret(), nil, 60)
	if err == nil {
		t.Error("expected error for non-HTTPS URL")
	}
}

func TestNew_RequiresLongSecret(t *testing.T) {
	_, err := webhook.New("https://example.com/hook", []byte("short"), nil, 60)
	if err == nil {
		t.Error("expected error for short secret")
	}
}

func TestNew_ValidWebhook(t *testing.T) {
	w, err := webhook.New("https://example.com/hook", validSecret(), []string{"osv.vuln.imported"}, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !w.IsActive() {
		t.Error("new webhook should be active")
	}
}

func TestShouldDeliver_MatchingEventType(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", validSecret(), []string{"osv.vuln.imported"}, 60)
	if !w.ShouldDeliver("osv.vuln.imported") {
		t.Error("should deliver for matching event type")
	}
}

func TestShouldDeliver_NonMatchingEventType(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", validSecret(), []string{"osv.vuln.imported"}, 60)
	if w.ShouldDeliver("osv.vuln.withdrawn") {
		t.Error("should NOT deliver for non-matching event type")
	}
}

func TestShouldDeliver_EmptyFilter_ReceivesAll(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", validSecret(), nil, 60)
	if !w.ShouldDeliver("any.event.type") {
		t.Error("empty filter should deliver all event types")
	}
}

func TestShouldDeliver_InactiveWebhook(t *testing.T) {
	w, _ := webhook.New("https://example.com/hook", validSecret(), nil, 60)
	w.Deactivate()
	if w.ShouldDeliver("osv.vuln.imported") {
		t.Error("inactive webhook should not deliver")
	}
}

func TestSign_ValidHMAC(t *testing.T) {
	secret := validSecret()
	w, _ := webhook.New("https://example.com/hook", secret, nil, 60)
	payload := []byte(`{"vuln_id":"CVE-2021-44228"}`)

	sig := w.Sign(payload)

	// Verify format
	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("signature should start with sha256=, got: %s", sig)
	}

	// Verify correctness
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != expected {
		t.Errorf("signature mismatch: got %s, want %s", sig, expected)
	}
}
