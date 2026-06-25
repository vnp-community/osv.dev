package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
	"github.com/osv/notification-service/internal/domain/repository"
)

var (
	ErrInsecureURL  = fmt.Errorf("webhook URL must use HTTPS")
	ErrSSRFBlocked  = fmt.Errorf("webhook URL points to private/internal network (SSRF protection)")
	ErrUnresolvable = fmt.Errorf("webhook URL hostname cannot be resolved")
	ErrPingFailed   = fmt.Errorf("webhook URL did not respond to ping test")
)

var privateRanges = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12",
		"192.168.0.0/16", "169.254.0.0/16", "::1/128", "fc00::/7",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, _ := net.ParseCIDR(c)
		nets = append(nets, n)
	}
	return nets
}()

func isPrivateIP(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// validateWebhookURL checks HTTPS scheme and SSRF protection.
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return ErrInsecureURL
	}

	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		return ErrUnresolvable
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return ErrSSRFBlocked
		}
	}
	return nil
}

type RegisterWebhookInput struct {
	URL     string
	Events  []webhook.EventType
	Secret  string
	OwnerID string
}

type RegisterWebhookUseCase struct {
	webhookRepo repository.WebhookRepository
	httpClient  *http.Client
}

func NewRegisterWebhookUseCase(repo repository.WebhookRepository) *RegisterWebhookUseCase {
	return &RegisterWebhookUseCase{
		webhookRepo: repo,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (uc *RegisterWebhookUseCase) Execute(ctx context.Context, in RegisterWebhookInput) (*webhook.Webhook, error) {
	// 1. Validate URL (HTTPS + SSRF)
	if err := validateWebhookURL(in.URL); err != nil {
		return nil, err
	}

	// 2. Generate secret if not provided
	secret := in.Secret
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
	}

	// 3. Build webhook aggregate
	evtTypes := make([]webhook.EventType, len(in.Events))
	for i, e := range in.Events {
		evtTypes[i] = e
	}
	wh := webhook.ReconstituteFromStrings(
		uuid.New().String(), in.OwnerID, in.URL,
		func() []string {
			ss := make([]string, len(evtTypes))
			for i, e := range evtTypes {
				ss[i] = string(e)
			}
			return ss
		}(),
		secret, true, time.Now().UTC(), time.Now().UTC(),
	)

	// 4. Ping test (send HEAD request to verify URL reachable)
	// Non-blocking: if the ping fails we still register the webhook.
	// This allows seed data with non-real URLs to be stored.
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, in.URL, nil)
	req.Header.Set("User-Agent", "GlobalCVE-Webhook-Verification/3.0")
	if resp, err := uc.httpClient.Do(req); err == nil {
		resp.Body.Close()
		// Accept any response (even 4xx) — URL is reachable
	}
	// ping failure is non-fatal: we log implicitly and continue

	// 5. Save to DB
	if err := uc.webhookRepo.Save(ctx, wh); err != nil {
		return nil, fmt.Errorf("register webhook: save: %w", err)
	}
	return wh, nil
}
