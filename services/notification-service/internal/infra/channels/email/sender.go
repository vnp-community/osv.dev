// Package email provides SMTP email delivery for notification-service.
package email

import (
	"context"
	"fmt"
	"net/smtp"
)

// Config holds SMTP connection settings.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Sender delivers notifications via SMTP email.
type Sender struct {
	cfg Config
}

// NewSender creates an email Sender.
func NewSender(cfg Config) *Sender {
	return &Sender{cfg: cfg}
}

// Send delivers an email to recipient.
// payload must contain "subject" and either "html" or "text" keys.
func (s *Sender) Send(_ context.Context, recipient string, payload map[string]interface{}) error {
	subject, _ := payload["subject"].(string)
	htmlBody, _ := payload["html"].(string)
	textBody, _ := payload["text"].(string)

	contentType := "text/html; charset=UTF-8"
	body := htmlBody
	if htmlBody == "" && textBody != "" {
		contentType = "text/plain; charset=UTF-8"
		body = textBody
	}
	if body == "" {
		body = subject // fallback: subject as body
	}

	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n%s",
		s.cfg.From, recipient, subject, contentType, body,
	))

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{recipient}, msg)
}
