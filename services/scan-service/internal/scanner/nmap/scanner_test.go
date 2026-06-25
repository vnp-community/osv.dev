package nmap

import (
    "strings"
    "testing"
)

// Sample nmap XML output for testing (minimal)
var sampleNmapXML = `<?xml version="1.0"?>
<nmaprun>
  <host>
    <status state="up"/>
    <address addr="192.168.1.10" addrtype="ipv4"/>
    <hostnames>
      <hostname name="web-server.local" type="PTR"/>
    </hostnames>
    <os>
      <osmatch name="Linux 5.15" accuracy="95"/>
    </os>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http" product="Apache httpd" version="2.4.51"/>
        <script id="vulners" output="
          CVE-2021-41773 7.5 https://vulners.com/cve/CVE-2021-41773
          CVE-2021-42013 9.8 https://vulners.com/cve/CVE-2021-42013
        "/>
      </port>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="8.4p1"/>
      </port>
    </ports>
  </host>
</nmaprun>`

func TestParseXMLOutput_Basic(t *testing.T) {
    findings, err := ParseXMLOutput([]byte(sampleNmapXML))
    if err != nil {
        t.Fatalf("ParseXMLOutput() error: %v", err)
    }
    if len(findings) != 1 {
        t.Fatalf("expected 1 finding, got %d", len(findings))
    }

    f := findings[0]
    if f.IPAddress != "192.168.1.10" {
        t.Errorf("IP = %s, want 192.168.1.10", f.IPAddress)
    }
    if f.Hostname != "web-server.local" {
        t.Errorf("Hostname = %s, want web-server.local", f.Hostname)
    }
    if f.OS != "Linux 5.15" {
        t.Errorf("OS = %s, want 'Linux 5.15'", f.OS)
    }
    if f.Status != "up" {
        t.Errorf("Status = %s, want up", f.Status)
    }
    if len(f.OpenPorts) != 2 {
        t.Errorf("OpenPorts len = %d, want 2", len(f.OpenPorts))
    }
    if len(f.Services) != 2 {
        t.Errorf("Services len = %d, want 2", len(f.Services))
    }
}

func TestParseXMLOutput_CVEExtraction(t *testing.T) {
    findings, _ := ParseXMLOutput([]byte(sampleNmapXML))
    f := findings[0]

    if len(f.CVEIDs) != 2 {
        t.Errorf("CVEIDs len = %d, want 2", len(f.CVEIDs))
    }

    cveSet := make(map[string]bool)
    for _, cve := range f.CVEIDs {
        cveSet[cve] = true
    }
    if !cveSet["CVE-2021-41773"] {
        t.Error("CVE-2021-41773 should be in CVEIDs")
    }
    if !cveSet["CVE-2021-42013"] {
        t.Error("CVE-2021-42013 should be in CVEIDs")
    }
}

func TestExtractCVEs_Regex(t *testing.T) {
    text := `CVE-2021-44228 10.0 RCE\nCVE-2021-45105 7.5 DoS\nnot-a-cve ignored`
    cves := extractCVEs(text)
    if len(cves) != 2 {
        t.Errorf("expected 2 CVEs, got %d: %v", len(cves), cves)
    }
    for _, cve := range cves {
        if !strings.HasPrefix(cve, "CVE-") {
            t.Errorf("CVE %s should start with CVE-", cve)
        }
    }
}

func TestExtractCVEs_Deduplication(t *testing.T) {
    text := "CVE-2021-44228 CVE-2021-44228 CVE-2021-44228"
    cves := extractCVEs(text)
    if len(cves) != 1 {
        t.Errorf("expected 1 unique CVE, got %d", len(cves))
    }
}

func TestParseXMLOutput_InvalidXML(t *testing.T) {
    _, err := ParseXMLOutput([]byte("not valid xml <"))
    if err == nil {
        t.Error("expected error for invalid XML")
    }
}
