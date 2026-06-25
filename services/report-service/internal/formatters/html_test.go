package formatters

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/google/osv.dev/services/report-service/internal/domain/entity"
)

func buildTestInput(minSeverity string) *entity.ReportInput {
    score74 := 7.4
    score45 := 4.5

    return &entity.ReportInput{
        ScanTarget:  "192.168.1.0/24",
        GeneratedAt: time.Now(),
        Theme:       entity.ThemeDark,
        MinSeverity: minSeverity,
        Products: []*entity.ProductSection{
            {
                ProductName: "Test Product",
                Findings: []*entity.ReportFinding{
                    {CVE: "CVE-2021-44228", Severity: "Critical", CVSSv3Score: nil, Title: "Log4j RCE", ComponentName: "192.168.1.10"},
                    {CVE: "CVE-2021-45105", Severity: "High", CVSSv3Score: &score74, Title: "Log4j DoS", ComponentName: "192.168.1.10"},
                    {CVE: "CVE-2021-99999", Severity: "Medium", CVSSv3Score: &score45, Title: "Some Medium", ComponentName: "192.168.1.20"},
                },
            },
        },
    }
}

func TestHTMLFormatter_Format(t *testing.T) {
    f, err := NewHTMLFormatter()
    if err != nil {
        t.Fatalf("NewHTMLFormatter: %v", err)
    }

    html, err := f.Format(context.Background(), buildTestInput(""))
    if err != nil {
        t.Fatalf("Format(): %v", err)
    }
    if len(html) == 0 {
        t.Fatal("HTML output is empty")
    }

    output := string(html)
    if !strings.Contains(output, "192.168.1.0/24") {
        t.Error("HTML should contain scan target")
    }
    if !strings.Contains(output, "CVE-2021-44228") {
        t.Error("HTML should contain Critical CVE")
    }
    if !strings.Contains(output, "Vulnerability Scan Report") {
        t.Error("HTML should contain report title")
    }
    if !strings.Contains(output, "bootstrap") {
        t.Error("HTML should include Bootstrap")
    }
}

func TestHTMLFormatter_SeverityFilter(t *testing.T) {
    f, _ := NewHTMLFormatter()
    html, _ := f.Format(context.Background(), buildTestInput("high"))
    output := string(html)

    if !strings.Contains(output, "CVE-2021-44228") {
        t.Error("Critical should pass high filter")
    }
    if !strings.Contains(output, "CVE-2021-45105") {
        t.Error("High should pass high filter")
    }
    if strings.Contains(output, "CVE-2021-99999") {
        t.Error("Medium should NOT pass high filter")
    }
}

func TestHTMLFormatter_DarkTheme(t *testing.T) {
    f, _ := NewHTMLFormatter()
    html, _ := f.Format(context.Background(), buildTestInput(""))
    if !strings.Contains(string(html), `data-bs-theme="dark"`) {
        t.Error("HTML should have dark theme attribute")
    }
}

func TestMeetsFilter(t *testing.T) {
    score80 := 8.0
    score45 := 4.5

    criticalFinding := &entity.ReportFinding{Severity: "Critical", CVSSv3Score: nil}
    highFinding := &entity.ReportFinding{Severity: "High", CVSSv3Score: &score80}
    mediumFinding := &entity.ReportFinding{Severity: "Medium", CVSSv3Score: &score45}

    if !MeetsFilter(criticalFinding, "high", nil) {
        t.Error("Critical should pass high filter")
    }
    if !MeetsFilter(highFinding, "high", nil) {
        t.Error("High should pass high filter")
    }
    if MeetsFilter(mediumFinding, "high", nil) {
        t.Error("Medium should NOT pass high filter")
    }

    // Score filter
    minScore := 7.0
    if MeetsFilter(mediumFinding, "", &minScore) {
        t.Error("CVSS 4.5 should NOT pass minScore=7.0")
    }
    if !MeetsFilter(highFinding, "", &minScore) {
        t.Error("CVSS 8.0 should pass minScore=7.0")
    }
}
