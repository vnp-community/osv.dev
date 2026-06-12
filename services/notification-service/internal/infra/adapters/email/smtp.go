// Package email provides SMTP email sending
package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSender sends emails via SMTP
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// New creates a new SMTP sender
func New(host string, port int, username, password, from string) *SMTPSender {
	return &SMTPSender{Host: host, Port: port, Username: username, Password: password, From: from}
}

// Send sends an email to one or more recipients
func (s *SMTPSender) Send(to []string, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html\r\n\r\n%s",
		s.From, strings.Join(to, ","), subject, body,
	)
	return smtp.SendMail(addr, auth, s.From, to, []byte(msg))
}
