// Package batch_enrich handles bulk CVE enrichment in parallel with rate limiting.
// [FIX TASK-HC-012] Replaced TODO stub: now calls enrich.UseCase per CVE.
package batch_enrich

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	enrichuc "github.com/osv/ai-service/internal/usecase/enrich"
)

// UseCase handles bulk CVE enrichment asynchronously.
type UseCase struct {
	enrichUC       *enrichuc.UseCase // [FIX TASK-HC-012] injected enrich use-case
	maxConcurrency int
	log            zerolog.Logger
}

// New creates a new batch enrich usecase.
// enrichUC may be nil when AI is disabled — each CVE gets a no-op result.
func New(enrichUC *enrichuc.UseCase, maxConcurrency int) *UseCase {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	return &UseCase{enrichUC: enrichUC, maxConcurrency: maxConcurrency, log: zerolog.Nop()}
}

// NewWithLogger creates a new batch enrich usecase with logger.
// [FIX TASK-HC-012 AC] Logger required to emit success/failed count.
func NewWithLogger(enrichUC *enrichuc.UseCase, maxConcurrency int, log zerolog.Logger) *UseCase {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	return &UseCase{enrichUC: enrichUC, maxConcurrency: maxConcurrency, log: log}
}

// Result holds the result of a single CVE enrichment.
type Result struct {
	CVEID     string
	Embedding []float32
	Severity  string
	Err       error
}

// ExecuteAsync enriches a list of CVE IDs in parallel with rate limiting.
// [FIX TASK-HC-012] Calls enrich.UseCase.Execute per CVE instead of a TODO stub.
// Logs success/failed count after all goroutines complete.
func (uc *UseCase) ExecuteAsync(ctx context.Context, cveIDs []string) []Result {
	if len(cveIDs) == 0 {
		return nil
	}

	uc.log.Info().
		Int("count", len(cveIDs)).
		Int("concurrency", uc.maxConcurrency).
		Msg("batch_enrich: starting")

	sem := make(chan struct{}, uc.maxConcurrency)
	results := make([]Result, len(cveIDs))
	var wg sync.WaitGroup

	for i, cveID := range cveIDs {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if uc.enrichUC == nil {
				// AI disabled — return empty result, not an error
				results[idx] = Result{CVEID: id}
				return
			}

			out, err := uc.enrichUC.Execute(ctx, enrichuc.CVEInput{CVEID: id})
			if err != nil {
				results[idx] = Result{CVEID: id, Err: err}
				return
			}
			severity := ""
			if out.Severity != nil {
				severity = string(out.Severity.Severity)
			}
			results[idx] = Result{
				CVEID:     id,
				Embedding: out.Embedding,
				Severity:  severity,
			}
		}(i, cveID)
	}
	wg.Wait()

	// [FIX TASK-HC-012 AC] Log success/failed count after completion
	success, failed := 0, 0
	for _, r := range results {
		if r.Err != nil {
			failed++
			uc.log.Warn().Err(r.Err).Str("cve_id", r.CVEID).Msg("batch_enrich: item failed")
		} else {
			success++
		}
	}
	uc.log.Info().
		Int("success", success).
		Int("failed", failed).
		Int("total", len(cveIDs)).
		Msg("batch_enrich: completed")

	return results
}
