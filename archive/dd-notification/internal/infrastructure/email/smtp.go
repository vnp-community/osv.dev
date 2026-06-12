// Package smtp provides the SMTP email sender for the notification service.
package smtp

import (
	"context"

	"gopkg.in/gomail.v2"
)

// Config holds SMTP connection settings.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	UseTLS   bool
}

// Sender implements dispatch.EmailSender using SMTP via gomail.
type Sender struct {
	dialer *gomail.Dialer
	from   string
}

// New creates an SMTP Sender with the given configuration.
func New(cfg Config) *Sender {
	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	d.SSL = cfg.UseTLS
	return &Sender{dialer: d, from: cfg.From}
}

// Send delivers an HTML email to the recipient.
func (s *Sender) Send(_ context.Context, to, subject, htmlBody string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)
	return s.dialer.DialAndSend(m)
}
