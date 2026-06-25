package scheduler

import (
    "context"
    "sync/atomic"
    "testing"
    "time"

    "github.com/rs/zerolog"
)

// mockRepo tracks schedule calls
type mockRepo struct {
    dueSchedules    []*ScheduledScanRecord
    missedSchedules []*ScheduledScanRecord
    updated         []*ScheduledScanRecord
}

func (m *mockRepo) FindDue(_ context.Context, _ time.Time) ([]*ScheduledScanRecord, error) {
    return m.dueSchedules, nil
}
func (m *mockRepo) FindMissed(_ context.Context, _ time.Time) ([]*ScheduledScanRecord, error) {
    return m.missedSchedules, nil
}
func (m *mockRepo) Update(_ context.Context, r *ScheduledScanRecord) error {
    m.updated = append(m.updated, r)
    return nil
}

func TestScheduler_TriggersDueSchedules(t *testing.T) {
    var triggerCount atomic.Int32

    mockCreateScan := func(_ context.Context, _ CreateScanInput) (*ScanOutput, error) {
        triggerCount.Add(1)
        return &ScanOutput{ScanID: "scan-123"}, nil
    }

    now := time.Now().UTC()
    past := now.Add(-5 * time.Minute)

    repo := &mockRepo{
        dueSchedules: []*ScheduledScanRecord{
            {
                ID: "sched-1", UserID: "user-1", Name: "Daily Scan",
                Targets: []string{"192.168.1.0/24"}, ScanType: "full",
                CronExpr: "0 2 * * *", NextRunAt: &past,
            },
        },
    }

    sched := New(repo, mockCreateScan, Config{CheckInterval: time.Minute}, zerolog.Nop())

    // Manually call checkAndTrigger instead of Start() for deterministic testing
    sched.checkAndTrigger(context.Background())

    if triggerCount.Load() != 1 {
        t.Errorf("expected 1 scan triggered, got %d", triggerCount.Load())
    }
    if len(repo.updated) != 1 {
        t.Errorf("expected 1 schedule updated, got %d", len(repo.updated))
    }
    if repo.updated[0].LastRunAt == nil {
        t.Error("LastRunAt should be set after trigger")
    }
    if repo.updated[0].NextRunAt == nil {
        t.Error("NextRunAt should be computed after trigger")
    }
}

func TestScheduler_NoopWhenNoDueSchedules(t *testing.T) {
    var triggerCount atomic.Int32

    mockCreateScan := func(_ context.Context, _ CreateScanInput) (*ScanOutput, error) {
        triggerCount.Add(1)
        return &ScanOutput{ScanID: "scan-123"}, nil
    }

    repo := &mockRepo{dueSchedules: []*ScheduledScanRecord{}}
    sched := New(repo, mockCreateScan, Config{}, zerolog.Nop())
    sched.checkAndTrigger(context.Background())

    if triggerCount.Load() != 0 {
        t.Error("should not trigger any scans when no due schedules")
    }
}

func TestScheduler_RecoverMissedSchedules(t *testing.T) {
    var triggerCount atomic.Int32

    mockCreateScan := func(_ context.Context, _ CreateScanInput) (*ScanOutput, error) {
        triggerCount.Add(1)
        return &ScanOutput{ScanID: "scan-123"}, nil
    }

    past := time.Now().UTC().Add(-3 * time.Hour)
    repo := &mockRepo{
        missedSchedules: []*ScheduledScanRecord{
            {ID: "sched-missed", Targets: []string{"10.0.0.0/8"}, NextRunAt: &past},
        },
    }

    sched := New(repo, mockCreateScan, Config{}, zerolog.Nop())
    sched.recoverMissedSchedules(context.Background())

    if triggerCount.Load() != 1 {
        t.Errorf("expected 1 recovered scan, got %d", triggerCount.Load())
    }
}

func TestScheduler_StartAndStop(t *testing.T) {
    repo := &mockRepo{}
    sched := New(repo, func(_ context.Context, _ CreateScanInput) (*ScanOutput, error) {
        return &ScanOutput{ScanID: "x"}, nil
    }, Config{CheckInterval: 100 * time.Millisecond}, zerolog.Nop())

    ctx, cancel := context.WithCancel(context.Background())
    sched.Start(ctx)

    // Give it a moment to start
    time.Sleep(50 * time.Millisecond)

    if !sched.IsRunning() {
        t.Error("scheduler should be running")
    }

    cancel() // Stop via context
    time.Sleep(200 * time.Millisecond)

    if sched.IsRunning() {
        t.Error("scheduler should have stopped after context cancel")
    }
}

func TestScheduler_MaxConcurrent(t *testing.T) {
    var concurrent atomic.Int32
    var maxConcurrent atomic.Int32

    mockCreateScan := func(ctx context.Context, _ CreateScanInput) (*ScanOutput, error) {
        current := concurrent.Add(1)
        if current > maxConcurrent.Load() {
            maxConcurrent.Store(current)
        }
        time.Sleep(100 * time.Millisecond) // Simulate scan time
        concurrent.Add(-1)
        return &ScanOutput{ScanID: "scan"}, nil
    }

    past := time.Now().UTC().Add(-5 * time.Minute)
    schedules := make([]*ScheduledScanRecord, 10)
    for i := range schedules {
        schedules[i] = &ScheduledScanRecord{
            ID: string(rune('a' + i)), Targets: []string{"1.2.3.4"}, NextRunAt: &past,
        }
    }

    repo := &mockRepo{dueSchedules: schedules}
    sched := New(repo, mockCreateScan, Config{MaxConcurrent: 3}, zerolog.Nop())
    sched.checkAndTrigger(context.Background())

    if maxConcurrent.Load() > 3 {
        t.Errorf("max concurrent = %d, should not exceed 3", maxConcurrent.Load())
    }
}
