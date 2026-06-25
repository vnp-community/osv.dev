// Package email — gomail-based SMTP sender (alternative to standard smtp.SendMail).
// Provides NewGoMailSender which wraps gomail.Dialer for SMTP with TLS support.
package email

import (
	"context"

	"gopkg.in/gomail.v2"
)

// GoMailSender implements SMTP delivery using the gomail library (supports STARTTLS).
type GoMailSender struct {
	dialer *gomail.Dialer
	from   string
}

// NewGoMailSender creates a GoMailSender with TLS support.
func NewGoMailSender(cfg Config, useTLS bool) *GoMailSender {
	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	d.SSL = useTLS
	return &GoMailSender{dialer: d, from: cfg.From}
}

// Send delivers an HTML email to the recipient.
func (s *GoMailSender) Send(_ context.Context, to, subject, htmlBody string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)
	return s.dialer.DialAndSend(m)
}
