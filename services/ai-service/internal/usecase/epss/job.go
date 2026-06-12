// Package epssupdate implements the daily EPSS score batch refresh job.
// TASK-02-06: Scheduled job to fetch updated EPSS scores for all known CVEs.
package epssupdate

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// EPSSScore represents a single CVE's EPSS score.
type EPSSScore struct {
	CVEID      string
	EPSS       float64
	Percentile float64
	UpdatedAt  time.Time
}

// ScoreFetcher fetches EPSS scores for a batch of CVEs.
type ScoreFetcher interface {
	GetBatch(ctx context.Context, cveIDs []string) ([]EPSSScore, error)
}

// CVELister lists all known CVE IDs that need EPSS score updates.
type CVELister interface {
	ListAllCVEIDs(ctx context.Context) ([]string, error)
}

// ScoreStore persists updated EPSS scores.
type ScoreStore interface {
	UpsertBatch(ctx context.Context, scores []EPSSScore) error
}

// AlertPublisher sends alerts when EPSS percentile crosses a threshold.
type AlertPublisher interface {
	PublishHighRiskAlert(ctx context.Context, cveID string, score EPSSScore) error
}

// JobConfig holds configuration for the daily EPSS update job.
type JobConfig struct {
	// BatchSize is the number of CVEs per EPSS API request.
	BatchSize int

	// HighRiskThreshold is the EPSS percentile above which a high-risk alert is sent.
	// Default: 0.95 (top 5% of all CVEs by exploitation probability)
	HighRiskThreshold float64

	// MaxRetries for failed batches.
	MaxRetries int
}

// DefaultJobConfig returns sensible defaults.
func DefaultJobConfig() JobConfig {
	return JobConfig{
		BatchSize:         500,
		HighRiskThreshold: 0.95,
		MaxRetries:        3,
	}
}

// Job runs the daily EPSS score batch update.
type Job struct {
	cfg       JobConfig
	fetcher   ScoreFetcher
	lister    CVELister
	store     ScoreStore
	alerter   AlertPublisher // optional; nil = no alerts
}

// NewJob creates a new EPSS update job.
func NewJob(cfg JobConfig, fetcher ScoreFetcher, lister CVELister, store ScoreStore, alerter AlertPublisher) *Job {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.HighRiskThreshold == 0 {
		cfg.HighRiskThreshold = 0.95
	}
	return &Job{
		cfg:     cfg,
		fetcher: fetcher,
		lister:  lister,
		store:   store,
		alerter: alerter,
	}
}

// RunResult holds the outcome of a single job run.
type RunResult struct {
	TotalCVEs     int
	UpdatedScores int
	HighRiskCVEs  []string
	Errors        []error
	Duration      time.Duration
}

// Run executes the daily EPSS batch update.
func (j *Job) Run(ctx context.Context) (*RunResult, error) {
	start := time.Now()
	logger := log.Ctx(ctx).With().Str("job", "epss_daily_update").Logger()
	logger.Info().Msg("starting daily EPSS batch update")

	result := &RunResult{}

	// 1. List all CVE IDs
	cveIDs, err := j.lister.ListAllCVEIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list CVE IDs: %w", err)
	}
	result.TotalCVEs = len(cveIDs)
	logger.Info().Int("total_cves", len(cveIDs)).Msg("fetched CVE ID list")

	// 2. Fetch EPSS in batches
	for i := 0; i < len(cveIDs); i += j.cfg.BatchSize {
		end := i + j.cfg.BatchSize
		if end > len(cveIDs) {
			end = len(cveIDs)
		}
		batch := cveIDs[i:end]

		var scores []EPSSScore
		var fetchErr error
		for attempt := 0; attempt <= j.cfg.MaxRetries; attempt++ {
			scores, fetchErr = j.fetcher.GetBatch(ctx, batch)
			if fetchErr == nil {
				break
			}
			logger.Warn().Err(fetchErr).Int("attempt", attempt+1).Msg("EPSS batch fetch failed, retrying")
			if attempt < j.cfg.MaxRetries {
				time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			}
		}
		if fetchErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch [%d-%d]: %w", i, end, fetchErr))
			logger.Error().Err(fetchErr).Msg("batch failed after retries, skipping")
			continue
		}

		// 3. Store updated scores
		if err := j.store.UpsertBatch(ctx, scores); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("store batch [%d-%d]: %w", i, end, err))
			logger.Error().Err(err).Msg("failed to store EPSS scores")
			continue
		}
		result.UpdatedScores += len(scores)

		// 4. Send high-risk alerts
		if j.alerter != nil {
			for _, score := range scores {
				if score.Percentile >= j.cfg.HighRiskThreshold {
					result.HighRiskCVEs = append(result.HighRiskCVEs, score.CVEID)
					if alertErr := j.alerter.PublishHighRiskAlert(ctx, score.CVEID, score); alertErr != nil {
						logger.Warn().Err(alertErr).Str("cve_id", score.CVEID).Msg("failed to send high-risk alert")
					}
				}
			}
		}

		logger.Debug().
			Int("batch_start", i).
			Int("batch_end", end).
			Int("scores_updated", len(scores)).
			Msg("EPSS batch processed")
	}

	result.Duration = time.Since(start)
	logger.Info().
		Int("total_cves", result.TotalCVEs).
		Int("updated_scores", result.UpdatedScores).
		Int("high_risk", len(result.HighRiskCVEs)).
		Int("errors", len(result.Errors)).
		Dur("duration", result.Duration).
		Msg("daily EPSS update complete")

	return result, nil
}

// Schedule returns a cron expression for daily execution at 06:00 UTC
// (EPSS daily data is typically published ~05:00 UTC).
func Schedule() string {
	return "0 6 * * *"
}
