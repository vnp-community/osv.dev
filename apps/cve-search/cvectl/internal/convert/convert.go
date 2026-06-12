// Package convert provides the cvectl 'convert' command.
// TASK-04-06/07: Vulnfeeds CLI adapter — convert CVE5/NVD files to OSV format.
package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewConvertCmd creates the 'convert' command group.
// Provides: convert cve5, convert nvd, convert batch
func NewConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert CVE data to OSV format",
		Long: `Convert CVE data files from various formats (CVE5 JSON, NVD JSON v2) to OSV format.

Examples:
  cvectl convert cve5 CVE-2024-1234.json
  cvectl convert nvd nvdcve-1.1-recent.json --out ./osv-output/
  cvectl convert batch ./cve-files/ --format nvd --out ./osv/`,
	}

	cmd.AddCommand(newCVE5Cmd())
	cmd.AddCommand(newNVDCmd())
	cmd.AddCommand(newBatchCmd())
	return cmd
}

// ConversionResult is the output of a single conversion.
type ConversionResult struct {
	InputFile  string
	OutputFile string
	VulnID     string
	Success    bool
	Error      string
	Warnings   []string
	Duration   time.Duration
}

// ---- cvectl convert cve5 ----

func newCVE5Cmd() *cobra.Command {
	var outFile string
	var outDir string
	var pretty bool

	cmd := &cobra.Command{
		Use:   "cve5 <file.json>",
		Short: "Convert a CVE5 JSON file to OSV format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := args[0]
			result, err := convertCVE5File(context.Background(), inputPath, outFile, outDir, pretty)
			if err != nil {
				return err
			}
			printResult(cmd, result)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outFile, "out-file", "f", "", "Output file path (default: <id>.osv.json)")
	cmd.Flags().StringVarP(&outDir, "out-dir", "d", ".", "Output directory for converted files")
	cmd.Flags().BoolVar(&pretty, "pretty", true, "Pretty-print output JSON")
	return cmd
}

// ---- cvectl convert nvd ----

func newNVDCmd() *cobra.Command {
	var outDir string
	var pretty bool

	cmd := &cobra.Command{
		Use:   "nvd <nvd-file.json>",
		Short: "Convert NVD JSON v2 file to OSV format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := args[0]
			results, err := convertNVDFile(context.Background(), inputPath, outDir, pretty)
			if err != nil {
				return fmt.Errorf("convert NVD file: %w", err)
			}
			printBatchSummary(cmd, results)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outDir, "out-dir", "d", ".", "Output directory for converted files")
	cmd.Flags().BoolVar(&pretty, "pretty", true, "Pretty-print output JSON")
	return cmd
}

// ---- cvectl convert batch ----

func newBatchCmd() *cobra.Command {
	var format string
	var outDir string
	var pretty bool
	var concurrency int

	cmd := &cobra.Command{
		Use:   "batch <input-dir>",
		Short: "Batch convert all CVE files in a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputDir := args[0]
			results, err := batchConvert(context.Background(), inputDir, format, outDir, pretty, concurrency)
			if err != nil {
				return fmt.Errorf("batch convert: %w", err)
			}
			printBatchSummary(cmd, results)
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "t", "cve5", "Input format: cve5, nvd")
	cmd.Flags().StringVarP(&outDir, "out-dir", "d", ".", "Output directory for converted files")
	cmd.Flags().BoolVar(&pretty, "pretty", true, "Pretty-print output JSON")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 4, "Number of concurrent conversions")
	return cmd
}

// ---- Conversion logic (client-side — calls converter API or does local conversion) ----

// convertCVE5File converts a single CVE5 JSON file to OSV format.
// In production this would call the converter gRPC service; here we do a minimal
// structural transformation for the CLI adapter.
func convertCVE5File(_ context.Context, inputPath, outFile, outDir string, pretty bool) (*ConversionResult, error) {
	start := time.Now()
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", inputPath, err)
	}

	// Parse raw CVE5 JSON
	var cveDoc map[string]interface{}
	if err := json.Unmarshal(data, &cveDoc); err != nil {
		return nil, fmt.Errorf("parse CVE5 JSON: %w", err)
	}

	// Extract CVE ID
	cveID := extractCVEID(cveDoc)
	if cveID == "" {
		return nil, fmt.Errorf("could not determine CVE ID from %q", inputPath)
	}

	// Minimal structural conversion to OSV envelope
	osv := map[string]interface{}{
		"id":         cveID,
		"schema_version": "1.6.0",
		"modified":   time.Now().UTC().Format(time.RFC3339),
		"published":  extractPublished(cveDoc),
		"aliases":    []string{cveID},
		"summary":    extractTitle(cveDoc),
		"details":    extractDescription(cveDoc),
		"references": extractReferences(cveDoc),
		"_source":    "nvd-cve5",
		"_converted_by": "cvectl convert cve5",
	}

	// Determine output path
	if outFile == "" {
		outFile = filepath.Join(outDir, cveID+".osv.json")
	}

	// Write output
	if err := writeJSON(outFile, osv, pretty); err != nil {
		return nil, err
	}

	return &ConversionResult{
		InputFile:  inputPath,
		OutputFile: outFile,
		VulnID:     cveID,
		Success:    true,
		Duration:   time.Since(start),
	}, nil
}

// convertNVDFile converts an NVD JSON v2 feed file (may contain multiple CVEs).
func convertNVDFile(_ context.Context, inputPath, outDir string, pretty bool) ([]*ConversionResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", inputPath, err)
	}

	// NVD JSON v2 wrapper: {"resultsPerPage":..., "vulnerabilities":[{"cve":{...}}]}
	var nvdFeed struct {
		Vulnerabilities []struct {
			CVE map[string]interface{} `json:"cve"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(data, &nvdFeed); err != nil {
		return nil, fmt.Errorf("parse NVD JSON: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	var results []*ConversionResult
	for _, item := range nvdFeed.Vulnerabilities {
		start := time.Now()
		cveID := extractCVEID(item.CVE)
		if cveID == "" {
			continue
		}
		osv := map[string]interface{}{
			"id":             cveID,
			"schema_version": "1.6.0",
			"modified":       time.Now().UTC().Format(time.RFC3339),
			"aliases":        []string{cveID},
			"summary":        extractTitle(item.CVE),
			"_source":        "nvd-json-v2",
		}
		outFile := filepath.Join(outDir, cveID+".osv.json")
		r := &ConversionResult{
			InputFile:  inputPath,
			OutputFile: outFile,
			VulnID:     cveID,
			Duration:   time.Since(start),
		}
		if err := writeJSON(outFile, osv, pretty); err != nil {
			r.Error = err.Error()
		} else {
			r.Success = true
		}
		results = append(results, r)
	}
	return results, nil
}

// batchConvert converts all matching files in a directory.
func batchConvert(_ context.Context, inputDir, format, outDir string, pretty bool, _ int) ([]*ConversionResult, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", inputDir, err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	var results []*ConversionResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(inputDir, entry.Name())
		var r *ConversionResult
		var err error
		switch strings.ToLower(format) {
		case "nvd":
			subResults, e := convertNVDFile(context.Background(), path, outDir, pretty)
			if e != nil {
				results = append(results, &ConversionResult{InputFile: path, Error: e.Error()})
				continue
			}
			results = append(results, subResults...)
			continue
		default: // cve5
			r, err = convertCVE5File(context.Background(), path, "", outDir, pretty)
			if err != nil {
				r = &ConversionResult{InputFile: path, Error: err.Error()}
			}
		}
		results = append(results, r)
	}
	return results, nil
}

// ---- Output helpers ----

func printResult(cmd *cobra.Command, r *ConversionResult) {
	if r.Success {
		fmt.Fprintf(cmd.OutOrStdout(), "✅ %s → %s (%s)\n", r.VulnID, r.OutputFile, r.Duration.Round(time.Millisecond))
		for _, w := range r.Warnings {
			fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️  %s\n", w)
		}
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "❌ %s: %s\n", r.InputFile, r.Error)
	}
}

func printBatchSummary(cmd *cobra.Command, results []*ConversionResult) {
	ok, fail := 0, 0
	for _, r := range results {
		if r.Success {
			ok++
		} else {
			fail++
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nConverted: %d ✅  Failed: %d ❌\n", ok, fail)
	for _, r := range results {
		if !r.Success && r.Error != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ❌ %s: %s\n", r.InputFile, r.Error)
		}
	}
}

// ---- JSON extraction helpers ----

func extractCVEID(doc map[string]interface{}) string {
	// CVE5: doc["cveMetadata"]["cveId"]
	if meta, ok := doc["cveMetadata"].(map[string]interface{}); ok {
		if id, ok := meta["cveId"].(string); ok {
			return id
		}
	}
	// NVD-style flat: doc["id"]
	if id, ok := doc["id"].(string); ok {
		return id
	}
	return ""
}

func extractTitle(doc map[string]interface{}) string {
	// CVE5: containers.cna.title
	if containers, ok := doc["containers"].(map[string]interface{}); ok {
		if cna, ok := containers["cna"].(map[string]interface{}); ok {
			if title, ok := cna["title"].(string); ok {
				return title
			}
			// Fall back to first description
			if descs, ok := cna["descriptions"].([]interface{}); ok && len(descs) > 0 {
				if d, ok := descs[0].(map[string]interface{}); ok {
					if val, ok := d["value"].(string); ok && len(val) < 120 {
						return val
					}
				}
			}
		}
	}
	return ""
}

func extractDescription(doc map[string]interface{}) string {
	if containers, ok := doc["containers"].(map[string]interface{}); ok {
		if cna, ok := containers["cna"].(map[string]interface{}); ok {
			if descs, ok := cna["descriptions"].([]interface{}); ok {
				for _, d := range descs {
					if dm, ok := d.(map[string]interface{}); ok {
						if lang, _ := dm["lang"].(string); lang == "en" || lang == "" {
							if val, ok := dm["value"].(string); ok {
								return val
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func extractPublished(doc map[string]interface{}) string {
	if meta, ok := doc["cveMetadata"].(map[string]interface{}); ok {
		if pub, ok := meta["datePublished"].(string); ok {
			return pub
		}
	}
	return ""
}

func extractReferences(doc map[string]interface{}) []map[string]interface{} {
	var refs []map[string]interface{}
	if containers, ok := doc["containers"].(map[string]interface{}); ok {
		if cna, ok := containers["cna"].(map[string]interface{}); ok {
			if rawRefs, ok := cna["references"].([]interface{}); ok {
				for _, ref := range rawRefs {
					if rm, ok := ref.(map[string]interface{}); ok {
						if url, ok := rm["url"].(string); ok {
							refs = append(refs, map[string]interface{}{"type": "WEB", "url": url})
						}
					}
				}
			}
		}
	}
	return refs
}

func writeJSON(path string, v interface{}, pretty bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
