// Package ssrf provides URL validation to prevent Server-Side Request Forgery attacks.
// Used by webhook channel senders to validate destination URLs before HTTP requests.
package ssrf

import (
	"fmt"
	"net"
	"net/url"
)

var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",     // loopback
		"10.0.0.0/8",      // private class A
		"172.16.0.0/12",   // private class B
		"192.168.0.0/16",  // private class C
		"169.254.0.0/16",  // link-local (AWS metadata: 169.254.169.254)
		"0.0.0.0/8",       // unspecified
		"::1/128",          // IPv6 loopback
		"fc00::/7",         // IPv6 unique local
		"fe80::/10",        // IPv6 link-local
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateRanges = append(privateRanges, block)
		}
	}
}

// Checker validates webhook URLs against SSRF attack vectors.
type Checker struct{}

// New creates a new SSRF Checker.
func New() *Checker { return &Checker{} }

// Validate checks that rawURL is safe to use as a webhook destination:
//   - Must be http or https scheme
//   - Must not resolve to private/internal IP addresses
//   - Must not be localhost
func (c *Checker) Validate(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https URLs allowed, got %q", u.Scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}

	// Resolve hostname to IPs
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("cannot resolve hostname %q: %w", hostname, err)
	}

	for _, ip := range ips {
		for _, block := range privateRanges {
			if block.Contains(ip) {
				return fmt.Errorf("SSRF protection: %q resolves to private/internal IP %s", rawURL, ip)
			}
		}
	}
	return nil
}
