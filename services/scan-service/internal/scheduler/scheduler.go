package scheduler

import (
    "context"
    "sync"
    "time"

    "github.com/rs/zerolog"
)

// CreateScanFunc is a function that creates a new scan
// (matches CreateScanUseCase.Execute signature)
type CreateScanFunc func(ctx context.Context, input CreateScanInput) (*ScanOutput, error)

// CreateScanInput is the input for creating a scheduled scan
type CreateScanInput struct {
    UserID   string
    Targets  []string
    ScanType string
    Priority int
}

// ScanOutput contains the created scan ID
type ScanOutput struct {
    ScanID string
}

// ScheduledScanRepository defines the storage interface for scheduler
type ScheduledScanRepository interface {
    // FindDue returns all enabled schedules with next_run_at <= now
    FindDue(ctx context.Context, now time.Time) ([]*ScheduledScanRecord, error)
    // FindMissed returns enabled schedules with next_run_at < now (for recovery)
    FindMissed(ctx context.Context, now time.Time) ([]*ScheduledScanRecord, error)
    // Update saves the schedule's last_run_at and next_run_at
    Update(ctx context.Context, record *ScheduledScanRecord) error
}

// ScheduledScanRecord represents a minimal schedule record for the scheduler
type ScheduledScanRecord struct {
    ID         string
    UserID     string
    Name       string
    Targets    []string
    ScanType   string
    CronExpr   string
    Enabled    bool
    LastRunAt  *time.Time
    NextRunAt  *time.Time
}

// ComputeNextRun returns the next scheduled time after now
func (s *ScheduledScanRecord) ComputeNextRun() time.Time {
    // Simple daily fallback
    // In practice, use robfig/cron parser on s.CronExpr
    next := time.Now().UTC().Add(24 * time.Hour)
    return next.Truncate(time.Hour)
}

// Scheduler manages automatic triggering of scheduled scans
type Scheduler struct {
    repo          ScheduledScanRepository
    createScan    CreateScanFunc
    checkInterval time.Duration
    maxConcurrent int
    logger        zerolog.Logger
    
    mu      sync.Mutex
    running bool
    stopCh  chan struct{}
}

// Config holds scheduler configuration
type Config struct {
    CheckInterval time.Duration // How often to check for due schedules (default: 1m)
    MaxConcurrent int           // Max concurrent scheduled scans (default: 3)
}

// DefaultConfig returns reasonable defaults
func DefaultConfig() Config {
    return Config{
        CheckInterval: 1 * time.Minute,
        MaxConcurrent: 3,
    }
}

// New creates a Scheduler
func New(
    repo ScheduledScanRepository,
    createScan CreateScanFunc,
    cfg Config,
    logger zerolog.Logger,
) *Scheduler {
    if cfg.CheckInterval == 0 {
        cfg.CheckInterval = time.Minute
    }
    if cfg.MaxConcurrent == 0 {
        cfg.MaxConcurrent = 3
    }

    return &Scheduler{
        repo:          repo,
        createScan:    createScan,
        checkInterval: cfg.CheckInterval,
        maxConcurrent: cfg.MaxConcurrent,
        logger:        logger.With().Str("component", "scheduler").Logger(),
        stopCh:        make(chan struct{}),
    }
}

// Start launches the scheduler goroutine.
// Call Stop() or cancel the context to stop.
func (s *Scheduler) Start(ctx context.Context) {
    s.mu.Lock()
    if s.running {
        s.mu.Unlock()
        s.logger.Warn().Msg("scheduler already running")
        return
    }
    s.running = true
    s.mu.Unlock()

    s.logger.Info().
        Dur("check_interval", s.checkInterval).
        Int("max_concurrent", s.maxConcurrent).
        Msg("scheduler started")

    go func() {
        defer func() {
            s.mu.Lock()
            s.running = false
            s.mu.Unlock()
        }()

        // Run recovery immediately on startup
        s.recoverMissedSchedules(ctx)

        // Run immediately (in case something is due right now)
        s.checkAndTrigger(ctx)

        ticker := time.NewTicker(s.checkInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                s.checkAndTrigger(ctx)

            case <-s.stopCh:
                s.logger.Info().Msg("scheduler stopped via Stop()")
                return

            case <-ctx.Done():
                s.logger.Info().Msg("scheduler stopped via context cancellation")
                return
            }
        }
    }()
}

// Stop signals the scheduler to stop
func (s *Scheduler) Stop() {
    select {
    case s.stopCh <- struct{}{}:
    default:
    }
}

// IsRunning returns whether the scheduler is currently active
func (s *Scheduler) IsRunning() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.running
}

// checkAndTrigger finds due schedules and triggers scans for them
func (s *Scheduler) checkAndTrigger(ctx context.Context) {
    now := time.Now().UTC()

    dueSchedules, err := s.repo.FindDue(ctx, now)
    if err != nil {
        s.logger.Error().Err(err).Msg("find due schedules failed")
        return
    }

    if len(dueSchedules) == 0 {
        return
    }

    s.logger.Info().Int("count", len(dueSchedules)).Msg("triggering due scheduled scans")

    // Semaphore for max concurrent limit
    sem := make(chan struct{}, s.maxConcurrent)
    var wg sync.WaitGroup

    for _, sched := range dueSchedules {
        wg.Add(1)
        sem <- struct{}{} // Acquire slot

        go func(sched *ScheduledScanRecord) {
            defer wg.Done()
            defer func() { <-sem }() // Release slot

            s.triggerSchedule(ctx, sched)
        }(sched)
    }

    wg.Wait()
}

// triggerSchedule executes one scheduled scan
func (s *Scheduler) triggerSchedule(ctx context.Context, sched *ScheduledScanRecord) {
    logger := s.logger.With().
        Str("schedule_id", sched.ID).
        Str("name", sched.Name).
        Strs("targets", sched.Targets).
        Logger()

    logger.Info().Msg("triggering scheduled scan")

    // Create the scan
    scanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    scan, err := s.createScan(scanCtx, CreateScanInput{
        UserID:   sched.UserID,
        Targets:  sched.Targets,
        ScanType: sched.ScanType,
        Priority: 5, // Default priority for scheduled scans
    })

    if err != nil {
        logger.Error().Err(err).Msg("failed to create scheduled scan")
        return
    }

    logger.Info().Str("scan_id", scan.ScanID).Msg("scheduled scan created")

    // Update schedule timestamps
    now := time.Now().UTC()
    sched.LastRunAt = &now
    nextRun := sched.ComputeNextRun()
    sched.NextRunAt = &nextRun

    updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer updateCancel()

    if err := s.repo.Update(updateCtx, sched); err != nil {
        logger.Error().Err(err).Msg("failed to update schedule timestamps")
        // Don't return — scan was already triggered successfully
    }
}

// recoverMissedSchedules triggers scans for schedules that were due while the service was down
func (s *Scheduler) recoverMissedSchedules(ctx context.Context) {
    now := time.Now().UTC()

    missed, err := s.repo.FindMissed(ctx, now)
    if err != nil {
        s.logger.Error().Err(err).Msg("recover missed schedules: query failed")
        return
    }

    if len(missed) == 0 {
        return
    }

    s.logger.Info().
        Int("count", len(missed)).
        Msg("recovering missed scheduled scans from downtime")

    for _, sched := range missed {
        s.logger.Info().
            Str("schedule_id", sched.ID).
            Str("name", sched.Name).
            Str("missed_run", sched.NextRunAt.Format(time.RFC3339)).
            Msg("recovering missed scan")

        s.triggerSchedule(ctx, sched)
    }
}
