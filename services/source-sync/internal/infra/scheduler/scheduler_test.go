package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/osv/source-sync/internal/infra/scheduler"
)

func TestScheduler_EnqueueAndNext(t *testing.T) {
	s := scheduler.NewScheduler(scheduler.DefaultBackoff)
	s.Enqueue(&scheduler.SyncJob{
		SourceID:    "nvd",
		Priority:    scheduler.PriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second), // overdue
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	job, err := s.Next(ctx)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if job.SourceID != "nvd" {
		t.Errorf("SourceID: got %q, want %q", job.SourceID, "nvd")
	}
}

func TestScheduler_PriorityOrder(t *testing.T) {
	s := scheduler.NewScheduler(scheduler.DefaultBackoff)
	now := time.Now().Add(-time.Second)

	s.Enqueue(&scheduler.SyncJob{SourceID: "low", Priority: scheduler.PriorityLow, ScheduledAt: now})
	s.Enqueue(&scheduler.SyncJob{SourceID: "high", Priority: scheduler.PriorityHigh, ScheduledAt: now})
	s.Enqueue(&scheduler.SyncJob{SourceID: "normal", Priority: scheduler.PriorityNormal, ScheduledAt: now})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	first, _ := s.Next(ctx)
	second, _ := s.Next(ctx)
	third, _ := s.Next(ctx)

	if first.SourceID != "high" {
		t.Errorf("first job should be high priority, got %q", first.SourceID)
	}
	if second.SourceID != "normal" {
		t.Errorf("second job should be normal priority, got %q", second.SourceID)
	}
	if third.SourceID != "low" {
		t.Errorf("third job should be low priority, got %q", third.SourceID)
	}
}

func TestScheduler_ContextCancellation(t *testing.T) {
	s := scheduler.NewScheduler(scheduler.DefaultBackoff)
	// Queue a job scheduled for the future
	s.Enqueue(&scheduler.SyncJob{
		SourceID:    "future",
		ScheduledAt: time.Now().Add(10 * time.Minute),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := s.Next(ctx)
	if err == nil {
		t.Error("expected context deadline exceeded")
	}
}

func TestScheduler_ScheduleRetry_Backoff(t *testing.T) {
	backoff := scheduler.BackoffConfig{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		MaxRetries: 3,
	}
	s := scheduler.NewScheduler(backoff)

	ok, delay1 := s.ScheduleRetry("src1", "rate limited")
	if !ok {
		t.Error("first retry should be allowed")
	}
	if delay1 != 100*time.Millisecond {
		t.Errorf("first retry delay: got %v, want 100ms", delay1)
	}

	ok, delay2 := s.ScheduleRetry("src1", "still limited")
	if !ok {
		t.Error("second retry should be allowed")
	}
	if delay2 != 200*time.Millisecond {
		t.Errorf("second retry delay: got %v, want 200ms", delay2)
	}

	s.ScheduleRetry("src1", "third")
	ok, _ = s.ScheduleRetry("src1", "fourth") // MaxRetries=3, this is 4th
	if ok {
		t.Error("fourth retry should be exhausted")
	}
}

func TestScheduler_MaxDelayCap(t *testing.T) {
	backoff := scheduler.BackoffConfig{
		BaseDelay:  1 * time.Hour,
		MaxDelay:   2 * time.Hour,
		Multiplier: 10.0,
		MaxRetries: 0,
	}
	delay := backoff.NextDelay(5)
	if delay > backoff.MaxDelay {
		t.Errorf("delay %v exceeded max %v", delay, backoff.MaxDelay)
	}
}

func TestScheduler_Len(t *testing.T) {
	s := scheduler.NewScheduler(scheduler.DefaultBackoff)
	if s.Len() != 0 {
		t.Error("new scheduler should be empty")
	}
	s.Enqueue(&scheduler.SyncJob{SourceID: "a", ScheduledAt: time.Now().Add(time.Hour)})
	s.Enqueue(&scheduler.SyncJob{SourceID: "b", ScheduledAt: time.Now().Add(time.Hour)})
	if s.Len() != 2 {
		t.Errorf("expected 2, got %d", s.Len())
	}
}

func TestScheduler_DeduplicateSourceID(t *testing.T) {
	s := scheduler.NewScheduler(scheduler.DefaultBackoff)
	s.Enqueue(&scheduler.SyncJob{SourceID: "src1", Priority: scheduler.PriorityLow, ScheduledAt: time.Now().Add(time.Hour)})
	s.Enqueue(&scheduler.SyncJob{SourceID: "src1", Priority: scheduler.PriorityHigh, ScheduledAt: time.Now().Add(time.Hour)})
	if s.Len() != 1 {
		t.Errorf("duplicate sourceID should replace: expected len=1, got %d", s.Len())
	}
}
