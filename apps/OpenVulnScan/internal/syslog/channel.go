// Package syslog — channel.go
// SyslogChannel implements SIEM notification channel using RFC 5424 syslog.
// This is the only new business logic file in the monolith.
// Implements the notification dispatcher interface for forwarding security events.
package syslog

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config holds SIEM syslog endpoint configuration.
type Config struct {
	Host     string // SIEM server hostname/IP
	Port     int    // Default: 514 (syslog) or 6514 (TLS)
	Protocol string // "udp" or "tcp"
	Enabled  bool
	// Facility for syslog: 1=user, 16=local0, 17=local1 ... 23=local7
	Facility int // Default: 16 (local0)
}

// Severity levels (RFC 5424).
const (
	SeverityEmergency = 0
	SeverityAlert     = 1
	SeverityCritical  = 2
	SeverityError     = 3
	SeverityWarning   = 4
	SeverityNotice    = 5
	SeverityInfo      = 6
	SeverityDebug     = 7
)

// Channel implements syslog/SIEM forwarding.
type Channel struct {
	cfg Config
	log zerolog.Logger
}

// New creates a new Syslog channel.
func New(cfg Config, l zerolog.Logger) *Channel {
	if cfg.Port == 0 {
		cfg.Port = 514
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "udp"
	}
	if cfg.Facility == 0 {
		cfg.Facility = 16 // local0
	}
	return &Channel{cfg: cfg, log: l.With().Str("channel", "syslog").Logger()}
}

// Send formats and forwards a security event to the configured syslog server.
func (c *Channel) Send(ctx context.Context, event interface{}) error {
	if !c.cfg.Enabled || c.cfg.Host == "" {
		return nil
	}
	msg := c.formatRFC5424(SeverityInfo, event)
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)

	switch c.cfg.Protocol {
	case "tcp":
		return c.sendTCP(ctx, addr, msg)
	default: // "udp"
		return c.sendUDP(addr, msg)
	}
}

// SendSecurityEvent forwards a security event with appropriate severity.
func (c *Channel) SendSecurityEvent(ctx context.Context, eventType string, payload interface{}) error {
	if !c.cfg.Enabled || c.cfg.Host == "" {
		return nil
	}

	sev := SeverityInfo
	// Map event type to syslog severity
	switch eventType {
	case "finding.created.critical", "scan.failed":
		sev = SeverityCritical
	case "finding.created.high":
		sev = SeverityError
	case "finding.created.medium":
		sev = SeverityWarning
	case "scan.completed":
		sev = SeverityNotice
	}

	msg := c.formatSecurityEvent(eventType, payload)
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	formatted := c.formatRFC5424(sev, msg)
	switch c.cfg.Protocol {
	case "tcp":
		return c.sendTCP(ctx, addr, formatted)
	default:
		return c.sendUDP(addr, formatted)
	}
}

// TestConnectivity tests the connection to the syslog server.
func (c *Channel) TestConnectivity(ctx context.Context) error {
	if c.cfg.Host == "" {
		return fmt.Errorf("syslog host not configured")
	}
	return c.Send(ctx, "OpenVulnScan SIEM connectivity test — "+time.Now().UTC().Format(time.RFC3339))
}

// UpdateConfig updates the channel configuration at runtime.
func (c *Channel) UpdateConfig(cfg Config) {
	c.cfg = cfg
}

// GetConfig returns the current configuration.
func (c *Channel) GetConfig() Config {
	return c.cfg
}

// ── RFC 5424 formatting ───────────────────────────────────────────────────────

// formatRFC5424 formats an event as RFC 5424 syslog message.
// <PRIVAL>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
func (c *Channel) formatRFC5424(severity int, event interface{}) string {
	// Priority = Facility * 8 + Severity
	prival := c.cfg.Facility*8 + severity
	timestamp := time.Now().UTC().Format(time.RFC3339)
	hostname := "openvulnscan"
	appname := "openvulnscan"
	procid := "-"
	msgid := "SECURITY"
	structuredData := "-"

	var msg string
	switch e := event.(type) {
	case string:
		msg = e
	case fmt.Stringer:
		msg = e.String()
	default:
		msg = fmt.Sprintf("%+v", e)
	}

	// Clean msg for syslog (no newlines within message)
	msg = strings.ReplaceAll(msg, "\n", " ")

	return fmt.Sprintf("<%d>1 %s %s %s %s %s %s %s\n",
		prival, timestamp, hostname, appname, procid, msgid, structuredData, msg)
}

// formatSecurityEvent creates a human-readable security event message.
func (c *Channel) formatSecurityEvent(eventType string, payload interface{}) string {
	switch eventType {
	case "scan.completed":
		return fmt.Sprintf("SCAN_COMPLETED payload=%v", payload)
	case "scan.failed":
		return fmt.Sprintf("SCAN_FAILED payload=%v", payload)
	case "finding.created.critical":
		return fmt.Sprintf("CRITICAL_FINDING_CREATED payload=%v", payload)
	case "finding.created.high":
		return fmt.Sprintf("HIGH_FINDING_CREATED payload=%v", payload)
	case "finding.status_changed":
		return fmt.Sprintf("FINDING_STATUS_CHANGED payload=%v", payload)
	case "agent.report.submitted":
		return fmt.Sprintf("AGENT_REPORT_SUBMITTED payload=%v", payload)
	case "auth.login.failed":
		return fmt.Sprintf("AUTH_LOGIN_FAILED payload=%v", payload)
	case "auth.login.success":
		return fmt.Sprintf("AUTH_LOGIN_SUCCESS payload=%v", payload)
	default:
		return fmt.Sprintf("SECURITY_EVENT type=%s payload=%v", eventType, payload)
	}
}

// ── Transport ─────────────────────────────────────────────────────────────────

// sendUDP sends a syslog message via UDP (stateless, best-effort).
func (c *Channel) sendUDP(addr, msg string) error {
	conn, err := net.DialTimeout("udp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("syslog udp dial %s: %w", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, err = fmt.Fprint(conn, msg)
	if err != nil {
		c.log.Warn().Err(err).Str("addr", addr).Msg("syslog UDP send failed")
	}
	return err
}

// sendTCP sends a syslog message via TCP with context support.
func (c *Channel) sendTCP(ctx context.Context, addr, msg string) error {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("syslog tcp dial %s: %w", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
	_, err = fmt.Fprint(conn, msg)
	return err
}
