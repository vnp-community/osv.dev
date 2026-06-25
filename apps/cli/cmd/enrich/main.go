// Command enrich triggers AI enrichment for CVEs via ai-service gRPC.
// This is a NEW command — no existing CLI code is modified.
//
// Usage:
//
//	# Enrich a single CVE:
//	osv-enrich -cve CVE-2021-44228
//
//	# Batch enrich from a text file (one CVE-ID per line):
//	osv-enrich -batch -input cve-ids.txt
//
// Environment variables:
//
//	AI_ENRICHER_ADDR — ai-service gRPC address (default: localhost:50052)
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

func main() {
	if err := run(); err != nil {
		slog.Error("enrich failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cveID  := flag.String("cve", "", "Single CVE ID to enrich (e.g. CVE-2021-44228)")
	batch  := flag.Bool("batch", false, "Batch mode: read CVE IDs from -input file")
	input  := flag.String("input", "", "Path to text file with CVE IDs (one per line), required when -batch=true")
	force  := flag.Bool("force", false, "Force re-enrichment even if already enriched")
	timeout := flag.Duration("timeout", 5*time.Minute, "Total operation timeout")
	flag.Parse()

	addr := os.Getenv("AI_ENRICHER_ADDR")
	if addr == "" {
		addr = "localhost:50052"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	// Dial ai-service
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect ai-service@%s: %w", addr, err)
	}
	defer conn.Close()

	client := aiv1.NewAIEnrichmentServiceClient(conn)

	if *batch {
		if *input == "" {
			return fmt.Errorf("-input is required in batch mode")
		}
		return runBatch(ctx, client, *input, *force)
	}

	if *cveID == "" {
		return fmt.Errorf("-cve or -batch required")
	}
	return runSingle(ctx, client, *cveID, *force)
}

func runSingle(ctx context.Context, client aiv1.AIEnrichmentServiceClient, cveID string, force bool) error {
	slog.Info("enriching CVE", slog.String("id", cveID))
	resp, err := client.EnrichCVE(ctx, &aiv1.EnrichCVERequest{
		CveId:        cveID,
		ForceRefresh: force,
	})
	if err != nil {
		return fmt.Errorf("EnrichCVE(%s): %w", cveID, err)
	}
	slog.Info("enriched",
		slog.String("id", cveID),
		slog.String("summary", resp.GetSummaryShort()),
	)
	return nil
}

func runBatch(ctx context.Context, client aiv1.AIEnrichmentServiceClient, filePath string, force bool) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	var ids []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", filePath, err)
	}

	slog.Info("batch enrich starting", slog.Int("count", len(ids)))
	_, err = client.BatchEnrich(ctx, &aiv1.BatchEnrichRequest{CveIds: ids})
	if err != nil {
		return fmt.Errorf("BatchEnrich: %w", err)
	}
	slog.Info("batch enrich complete", slog.Int("count", len(ids)))
	return nil
}
