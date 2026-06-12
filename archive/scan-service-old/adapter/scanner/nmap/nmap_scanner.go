// Package nmap provides an nmap-based network scanner adapter.
package nmap

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/rs/zerolog"
)

// NmapScanner wraps the nmap binary for vulnerability scanning.
type NmapScanner struct {
	BinaryPath string
	Log        zerolog.Logger
}

// NewNmapScanner creates a new NmapScanner using the system nmap binary.
func NewNmapScanner(binaryPath string, log zerolog.Logger) *NmapScanner {
	if binaryPath == "" {
		binaryPath = "nmap"
	}
	return &NmapScanner{BinaryPath: binaryPath, Log: log}
}

// RunFull performs a full vulnerability scan (-sV -O --script=vulners).
func (s *NmapScanner) RunFull(ctx context.Context, target string, opts entity.ScanOptions) ([]*entity.Finding, error) {
	ports := opts.Ports
	if ports == "" {
		ports = "1-1024,8080,8443,9000,9090"
	}
	intensity := opts.Intensity
	if intensity < 1 || intensity > 5 {
		intensity = 3 // default: normal
	}

	args := []string{
		"-sV", "-O",
		"--script=vulners",
		fmt.Sprintf("-T%d", intensity),
		"-p", ports,
		"-oX", "-", // XML output to stdout
		"--host-timeout", "120s",
		target,
	}

	s.Log.Info().Str("target", target).Strs("args", args).Msg("starting full nmap scan")
	output, err := s.runNmap(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("nmap full scan: %w", err)
	}

	nmapRun, err := ParseXML(output)
	if err != nil {
		return nil, fmt.Errorf("parse nmap xml: %w", err)
	}

	scanID := uuid.New() // caller should override with real scan ID
	return nmapRun.ToFindings(scanID), nil
}

// RunDiscovery performs a host discovery scan (-sn, no port scan).
func (s *NmapScanner) RunDiscovery(ctx context.Context, cidr string) ([]*entity.DiscoveryHost, error) {
	args := []string{"-sn", "-oX", "-", "--host-timeout", "30s", cidr}
	s.Log.Info().Str("cidr", cidr).Msg("starting discovery scan")

	output, err := s.runNmap(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("nmap discovery: %w", err)
	}

	nmapRun, err := ParseXML(output)
	if err != nil {
		return nil, err
	}
	return nmapRun.ToDiscoveryHosts(uuid.Nil), nil
}

func (s *NmapScanner) runNmap(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, s.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("nmap exited: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// ── XML Parser ────────────────────────────────────────────────────────────────

// NmapRun is the top-level XML element from nmap -oX output.
type NmapRun struct {
	XMLName xml.Name   `xml:"nmaprun"`
	Hosts   []NmapHost `xml:"host"`
}

// NmapHost represents a single host in nmap output.
type NmapHost struct {
	Status    NmapStatus    `xml:"status"`
	Addresses []NmapAddress `xml:"address"`
	Hostnames []NmapHostname `xml:"hostnames>hostname"`
	Ports     []NmapPort    `xml:"ports>port"`
	OS        []NmapOSMatch `xml:"os>osmatch"`
}

type NmapStatus    struct { State string `xml:"state,attr"` }
type NmapAddress   struct { Addr string `xml:"addr,attr"`; AddrType string `xml:"addrtype,attr"` }
type NmapHostname  struct { Name string `xml:"name,attr"` }
type NmapPort      struct {
	Protocol string     `xml:"protocol,attr"`
	PortID   int        `xml:"portid,attr"`
	State    NmapState  `xml:"state"`
	Service  NmapSvc    `xml:"service"`
	Scripts  []NmapScript `xml:"script"`
}
type NmapState   struct { State string `xml:"state,attr"` }
type NmapSvc     struct { Name string `xml:"name,attr"`; Product string `xml:"product,attr"`; Version string `xml:"version,attr"` }
type NmapScript  struct { ID string `xml:"id,attr"`; Output string `xml:"output,attr"` }
type NmapOSMatch struct { Name string `xml:"name,attr"` }

// ParseXML parses nmap XML output.
func ParseXML(data []byte) (*NmapRun, error) {
	var run NmapRun
	if err := xml.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("unmarshal nmap xml: %w", err)
	}
	return &run, nil
}

// ToFindings converts nmap hosts to domain findings.
func (r *NmapRun) ToFindings(scanID uuid.UUID) []*entity.Finding {
	findings := make([]*entity.Finding, 0, len(r.Hosts))
	for _, h := range r.Hosts {
		if h.Status.State != "up" {
			continue
		}
		f := &entity.Finding{
			ID:     uuid.New(),
			ScanID: scanID,
		}
		for _, addr := range h.Addresses {
			if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
				f.IPAddress = addr.Addr
			}
		}
		if len(h.Hostnames) > 0 {
			f.Hostname = h.Hostnames[0].Name
		}
		if len(h.OS) > 0 {
			f.OS = h.OS[0].Name
		}

		cveSet := map[string]struct{}{}
		for _, p := range h.Ports {
			if p.State.State != "open" {
				continue
			}
			f.OpenPorts = append(f.OpenPorts, entity.Port{
				Port: p.PortID, Protocol: p.Protocol, State: p.State.State,
			})
			f.Services = append(f.Services, entity.Service{
				Port: p.PortID, Name: p.Service.Name,
				Product: p.Service.Product, Version: p.Service.Version,
			})
			for _, script := range p.Scripts {
				for _, cve := range ExtractCVEs(script.Output) {
					cveSet[cve] = struct{}{}
				}
			}
		}
		for cve := range cveSet {
			f.CVEIDs = append(f.CVEIDs, cve)
		}
		sort.Strings(f.CVEIDs)
		findings = append(findings, f)
	}
	return findings
}

// ToDiscoveryHosts converts nmap hosts to DiscoveryHost entities.
func (r *NmapRun) ToDiscoveryHosts(scanID uuid.UUID) []*entity.DiscoveryHost {
	var hosts []*entity.DiscoveryHost
	for _, h := range r.Hosts {
		dh := &entity.DiscoveryHost{
			ID:     uuid.New(),
			ScanID: scanID,
			Status: h.Status.State,
		}
		for _, addr := range h.Addresses {
			if addr.AddrType == "ipv4" {
				dh.IPAddress = addr.Addr
			}
		}
		if len(h.Hostnames) > 0 {
			dh.Hostname = h.Hostnames[0].Name
		}
		hosts = append(hosts, dh)
	}
	return hosts
}

// ── CVE Extractor ─────────────────────────────────────────────────────────────

var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)

// ExtractCVEs extracts deduplicated, sorted CVE IDs from nmap script output.
func ExtractCVEs(scriptOutput string) []string {
	matches := cveRegex.FindAllString(strings.ToUpper(scriptOutput), -1)
	seen := map[string]struct{}{}
	var result []string
	for _, m := range matches {
		if _, ok := seen[m]; !ok {
			seen[m] = struct{}{}
			result = append(result, m)
		}
	}
	sort.Strings(result)
	return result
}
