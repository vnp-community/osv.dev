// cvectl — OSV CVE management CLI
// Usage: cvectl <command> [flags]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/osv/cvectl/internal/client"
	"github.com/osv/cvectl/internal/config"
	"github.com/osv/cvectl/internal/convert"
	"github.com/osv/cvectl/internal/output"
	"github.com/spf13/cobra"
)

var (
	cfgFile      string
	outputFormat string
	cfg          *config.Config
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "cvectl",
	Short: "cvectl — OSV CVE management CLI",
	Long: `cvectl is the command-line interface for managing CVE data in the OSV platform.

Examples:
  cvectl vuln get CVE-2023-44487
  cvectl sources list
  cvectl admin stats
  cvectl convert cve5 CVE-2024-1234.json`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		// CLI flag overrides config file
		if outputFormat != "" {
			cfg.OutputFormat = outputFormat
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format: table, json")

	// Register command groups
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newSourcesCmd())
	rootCmd.AddCommand(newVulnCmd())
	rootCmd.AddCommand(newAdminCmd())
	rootCmd.AddCommand(convert.NewConvertCmd()) // TASK-04-06/07: vulnfeeds CLI adapter
}

// ── version ───────────────────────────────────────────────────────────────────

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show cvectl version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cvectl v1.0.0 (osv-platform)")
	},
}

// ── sources ───────────────────────────────────────────────────────────────────

func newSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage CVE data sources",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all CVE sources with status",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			sources, err := c.ListSources(context.Background())
			if err != nil {
				return err
			}
			p := output.New(cfg.OutputFormat)
			headers := []string{"NAME", "STATE", "LAST_SYNC", "CVE_COUNT", "ERRORS_24H"}
			var rows [][]string
			for _, s := range sources {
				rows = append(rows, []string{
					s.Name,
					s.State,
					output.FormatTime(s.LastSyncAt),
					fmt.Sprintf("%d", s.CVECountLastSync),
					fmt.Sprintf("%d", s.ErrorCount24h),
				})
			}
			p.PrintTable(headers, rows)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Get detailed status of a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			sources, err := c.ListSources(context.Background())
			if err != nil {
				return err
			}
			p := output.New(cfg.OutputFormat)
			for _, s := range sources {
				if s.Name == args[0] {
					if cfg.OutputFormat == "json" {
						p.PrintJSON(s)
					} else {
						fmt.Printf("Name:        %s\n", s.Name)
						fmt.Printf("State:       %s\n", s.State)
						fmt.Printf("Last Sync:   %s\n", output.FormatTime(s.LastSyncAt))
						fmt.Printf("CVE Count:   %d\n", s.CVECountLastSync)
						fmt.Printf("Errors/24h:  %d\n", s.ErrorCount24h)
					}
					return nil
				}
			}
			return fmt.Errorf("source %q not found", args[0])
		},
	}

	syncCmd := &cobra.Command{
		Use:   "sync <name>",
		Short: "Trigger manual sync for a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			if err := c.TriggerSync(context.Background(), args[0]); err != nil {
				return err
			}
			output.New(cfg.OutputFormat).PrintSuccess(fmt.Sprintf("Sync triggered for source %q", args[0]))
			return nil
		},
	}

	pauseCmd := &cobra.Command{
		Use:   "pause <name>",
		Short: "Pause a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			if err := c.PauseSource(context.Background(), args[0]); err != nil {
				return err
			}
			output.New(cfg.OutputFormat).PrintSuccess(fmt.Sprintf("Source %q paused", args[0]))
			return nil
		},
	}

	resumeCmd := &cobra.Command{
		Use:   "resume <name>",
		Short: "Resume a paused source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			if err := c.ResumeSource(context.Background(), args[0]); err != nil {
				return err
			}
			output.New(cfg.OutputFormat).PrintSuccess(fmt.Sprintf("Source %q resumed", args[0]))
			return nil
		},
	}

	cmd.AddCommand(listCmd, statusCmd, syncCmd, pauseCmd, resumeCmd)
	return cmd
}

// ── vuln ──────────────────────────────────────────────────────────────────────

func newVulnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vuln",
		Short: "Query and manage vulnerabilities",
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get vulnerability details (OSV format)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			vuln, err := c.GetVuln(context.Background(), args[0])
			if err != nil {
				return err
			}
			p := output.New(cfg.OutputFormat)
			if cfg.OutputFormat == "json" {
				p.PrintJSON(vuln)
			} else {
				fmt.Printf("ID:        %s\n", getString(vuln, "id"))
				fmt.Printf("Summary:   %s\n", truncate(getString(vuln, "summary"), 80))
				fmt.Printf("Modified:  %s\n", getString(vuln, "modified"))
				fmt.Printf("Source:    %s\n", extractSource(args[0]))
			}
			return nil
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search vulnerabilities by keyword",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			query := strings.Join(args, " ")
			results, err := c.SearchVulns(context.Background(), query)
			if err != nil {
				return err
			}
			p := output.New(cfg.OutputFormat)
			if cfg.OutputFormat == "json" {
				p.PrintJSON(results)
				return nil
			}
			headers := []string{"VULN_ID", "SEVERITY", "SUMMARY"}
			var rows [][]string
			for _, r := range results {
				rows = append(rows, []string{
					r.VulnID,
					r.Severity,
					truncate(r.Summary, 60),
				})
			}
			p.PrintTable(headers, rows)
			return nil
		},
	}

	enrichCmd := &cobra.Command{
		Use:   "enrich <id>",
		Short: "View enrichment data (KEV, EPSS, tags)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			enrichment, err := c.GetEnrichment(context.Background(), args[0])
			if err != nil {
				return err
			}
			p := output.New(cfg.OutputFormat)
			if cfg.OutputFormat == "json" {
				p.PrintJSON(enrichment)
				return nil
			}
			fmt.Printf("Vuln ID:   %s\n", enrichment.VulnID)
			if enrichment.KEV != nil {
				fmt.Printf("KEV:       %s (added: %s)\n",
					output.FormatBool(enrichment.KEV.IsKEV), enrichment.KEV.DateAdded)
			}
			if enrichment.EPSS != nil {
				fmt.Printf("EPSS:      %.4f (%.1f%%, %s)\n",
					enrichment.EPSS.Score, enrichment.EPSS.Percentile*100, enrichment.EPSS.Tier)
			}
			if len(enrichment.Tags) > 0 {
				fmt.Printf("Tags:      %s\n", strings.Join(enrichment.Tags, ", "))
			}
			if len(enrichment.CWEIDs) > 0 {
				fmt.Printf("CWE:       %s\n", strings.Join(enrichment.CWEIDs, ", "))
			}
			fmt.Printf("Exploit:   %s\n", output.FormatBool(enrichment.ExploitAvailable))
			if enrichment.AISummary != "" {
				fmt.Printf("AI Summary:\n  %s\n", truncate(enrichment.AISummary, 200))
			}
			return nil
		},
	}

	relatedCmd := &cobra.Command{
		Use:   "related <id>",
		Short: "Find related vulnerabilities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			related, err := c.GetRelated(context.Background(), args[0])
			if err != nil {
				return err
			}
			p := output.New(cfg.OutputFormat)
			if cfg.OutputFormat == "json" {
				p.PrintJSON(related)
				return nil
			}
			headers := []string{"VULN_ID", "REASON", "SIMILARITY"}
			var rows [][]string
			for _, r := range related {
				sim := "—"
				if r.Score > 0 {
					sim = fmt.Sprintf("%.2f", r.Score)
				}
				rows = append(rows, []string{r.VulnID, "related", sim})
			}
			p.PrintTable(headers, rows)
			return nil
		},
	}

	cmd.AddCommand(getCmd, searchCmd, enrichCmd, relatedCmd)
	return cmd
}

// ── admin ─────────────────────────────────────────────────────────────────────

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Administrative operations",
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show platform statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := apiClient()
			stats, err := c.GetStats(context.Background())
			if err != nil {
				return err
			}
			output.New(cfg.OutputFormat).PrintJSON(stats)
			return nil
		},
	}

	withdrawCmd := &cobra.Command{
		Use:   "withdraw <id>",
		Short: "Withdraw a vulnerability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reason, _ := cmd.Flags().GetString("reason")
			c := apiClient()
			if err := c.WithdrawVuln(context.Background(), args[0], reason); err != nil {
				return err
			}
			output.New(cfg.OutputFormat).PrintSuccess(fmt.Sprintf("%s withdrawn", args[0]))
			return nil
		},
	}
	withdrawCmd.Flags().String("reason", "", "Reason for withdrawal (required)")
	withdrawCmd.MarkFlagRequired("reason") //nolint:errcheck

	reprocessCmd := &cobra.Command{
		Use:   "reprocess <id>",
		Short: "Trigger reprocessing of a vulnerability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reason, _ := cmd.Flags().GetString("reason")
			c := apiClient()
			if err := c.ReprocessVuln(context.Background(), args[0], reason); err != nil {
				return err
			}
			output.New(cfg.OutputFormat).PrintSuccess(fmt.Sprintf("%s queued for reprocessing", args[0]))
			return nil
		},
	}
	reprocessCmd.Flags().String("reason", "", "Reason for reprocessing")

	cmd.AddCommand(statsCmd, withdrawCmd, reprocessCmd)
	return cmd
}

// ── helpers ───────────────────────────────────────────────────────────────────

func apiClient() *client.Client {
	return client.New(cfg.Server, cfg.APIKey)
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch s := v.(type) {
		case string:
			return s
		default:
			b, _ := json.Marshal(v)
			return string(b)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func extractSource(id string) string {
	switch {
	case strings.HasPrefix(id, "CVE-"):
		return "NVD"
	case strings.HasPrefix(id, "GHSA-"):
		return "GitHub Advisory"
	case strings.HasPrefix(id, "OSV-"):
		return "OSS-Fuzz"
	default:
		return "Unknown"
	}
}
