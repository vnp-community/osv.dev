// Package smtp — email_sender.go
// TASK-HC-014: SMTP email sender implementing repository.EmailSender.
// Reads credentials from env vars; gracefully disabled when not configured.
package smtp

import (
	"context"
	"fmt"
	gosmtp "net/smtp"
	"strings"
)

// Sender sends emails via SMTP using net/smtp (stdlib).
type Sender struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// New creates a new SMTP Sender.
// host, port, username, password, from are read from environment — see embedded.go.
func New(host, port, username, password, from string) *Sender {
	return &Sender{host: host, port: port, username: username, password: password, from: from}
}

// SendInvitation sends an invitation email with the invite URL and temp password.
// Implements repository.EmailSender.
func (s *Sender) SendInvitation(_ context.Context, to, inviterName, inviteURL, tempPassword string) error {
	if s.host == "" {
		return fmt.Errorf("smtp: host not configured")
	}
	subject := "Invited to OSV Platform"
	body := strings.Join([]string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		fmt.Sprintf("Hello,\n\n%s has invited you to join OSV Platform.\n", inviterName),
		fmt.Sprintf("Accept your invitation here:\n  %s\n", inviteURL),
		fmt.Sprintf("Temporary password: %s\n", tempPassword),
		"Your invitation expires in 48 hours.",
		"",
		"If you did not expect this email, you can safely ignore it.",
	}, "\r\n")

	auth := gosmtp.PlainAuth("", s.username, s.password, s.host)
	addr := s.host + ":" + s.port
	if err := gosmtp.SendMail(addr, auth, s.from, []string{to}, []byte(body)); err != nil {
		return fmt.Errorf("smtp.SendMail: %w", err)
	}
	return nil
}
