package nmap

import (
    "encoding/xml"
    "fmt"
    "regexp"
    "strconv"
    "strings"
)

// cveRegex matches CVE IDs in nmap vulners script output
var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)

// nmapRun is the top-level XML structure from nmap -oX
type nmapRun struct {
    XMLName xml.Name  `xml:"nmaprun"`
    Hosts   []nmapHost `xml:"host"`
}

type nmapHost struct {
    Status    nmapStatus    `xml:"status"`
    Addresses []nmapAddress `xml:"address"`
    Hostnames []nmapHostname `xml:"hostnames>hostname"`
    OS        nmapOS        `xml:"os"`
    Ports     nmapPorts     `xml:"ports"`
}

type nmapStatus struct {
    State string `xml:"state,attr"` // "up" | "down"
}

type nmapAddress struct {
    Addr     string `xml:"addr,attr"`
    AddrType string `xml:"addrtype,attr"` // "ipv4" | "ipv6" | "mac"
}

type nmapHostname struct {
    Name string `xml:"name,attr"`
    Type string `xml:"type,attr"` // "user" | "PTR"
}

type nmapOS struct {
    Matches []nmapOSMatch `xml:"osmatch"`
}

type nmapOSMatch struct {
    Name     string `xml:"name,attr"`
    Accuracy string `xml:"accuracy,attr"`
}

type nmapPorts struct {
    Ports []nmapPort `xml:"port"`
}

type nmapPort struct {
    Protocol string      `xml:"protocol,attr"` // "tcp" | "udp"
    PortID   string      `xml:"portid,attr"`
    State    nmapState   `xml:"state"`
    Service  nmapService `xml:"service"`
    Scripts  []nmapScript `xml:"script"`
}

type nmapState struct {
    State string `xml:"state,attr"` // "open" | "closed" | "filtered"
}

type nmapService struct {
    Name    string `xml:"name,attr"`
    Product string `xml:"product,attr"`
    Version string `xml:"version,attr"`
}

type nmapScript struct {
    ID     string `xml:"id,attr"`
    Output string `xml:"output,attr"`
}

// ParseXMLOutput parses nmap XML output into ScanFinding slice
func ParseXMLOutput(data []byte) ([]*ScanFinding, error) {
    var run nmapRun
    if err := xml.Unmarshal(data, &run); err != nil {
        return nil, fmt.Errorf("parse XML: %w", err)
    }

    findings := make([]*ScanFinding, 0, len(run.Hosts))
    for _, h := range run.Hosts {
        f := convertHost(h)
        findings = append(findings, f)
    }

    return findings, nil
}

// ParseDiscoveryXML parses nmap -sn XML output
func ParseDiscoveryXML(data []byte) ([]*DiscoveryHost, error) {
    var run nmapRun
    if err := xml.Unmarshal(data, &run); err != nil {
        return nil, fmt.Errorf("parse XML: %w", err)
    }

    hosts := make([]*DiscoveryHost, 0, len(run.Hosts))
    for _, h := range run.Hosts {
        dh := &DiscoveryHost{Status: h.Status.State}
        for _, addr := range h.Addresses {
            if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
                dh.IPAddress = addr.Addr
            }
        }
        for _, hostname := range h.Hostnames {
            if hostname.Type == "PTR" || hostname.Type == "user" {
                dh.Hostname = hostname.Name
                break
            }
        }
        hosts = append(hosts, dh)
    }

    return hosts, nil
}

func convertHost(h nmapHost) *ScanFinding {
    f := &ScanFinding{Status: h.Status.State}

    // Extract IP and hostname
    for _, addr := range h.Addresses {
        if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
            f.IPAddress = addr.Addr
        }
    }
    for _, hn := range h.Hostnames {
        if hn.Type == "PTR" || hn.Type == "user" {
            f.Hostname = hn.Name
            break
        }
    }

    // Extract OS
    if len(h.OS.Matches) > 0 {
        f.OS = h.OS.Matches[0].Name
    }

    // Extract ports, services, CVEs
    cveSet := make(map[string]struct{})
    for _, p := range h.Ports.Ports {
        portNum, _ := strconv.Atoi(p.PortID)

        f.OpenPorts = append(f.OpenPorts, Port{
            Port:     portNum,
            Protocol: p.Protocol,
            State:    p.State.State,
        })

        if p.State.State == "open" && p.Service.Name != "" {
            f.Services = append(f.Services, Service{
                Port:     portNum,
                Protocol: p.Protocol,
                Name:     p.Service.Name,
                Product:  p.Service.Product,
                Version:  p.Service.Version,
            })
        }

        // Extract CVE IDs from vulners script output
        for _, script := range p.Scripts {
            if script.ID == "vulners" {
                for _, cve := range extractCVEs(script.Output) {
                    cveSet[cve] = struct{}{}
                }
            }
        }
    }

    // Deduplicate CVE IDs
    for cve := range cveSet {
        f.CVEIDs = append(f.CVEIDs, cve)
    }

    return f
}

// extractCVEs finds all CVE IDs in a text string
func extractCVEs(text string) []string {
    matches := cveRegex.FindAllString(text, -1)
    // Deduplicate
    seen := make(map[string]struct{})
    result := make([]string, 0, len(matches))
    for _, m := range matches {
        upper := strings.ToUpper(m)
        if _, exists := seen[upper]; !exists {
            seen[upper] = struct{}{}
            result = append(result, upper)
        }
    }
    return result
}
