// Package scheduler implements smart scheduling for source sync jobs.
// TASK-03-04: Priority queue scheduler with adaptive backoff for rate-limited sources.
package scheduler

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// Priority levels for source sync jobs.
type Priority int

const (
	PriorityHigh   Priority = 1
	PriorityNormal Priority = 5
	PriorityLow    Priority = 10
)

// SyncJob represents a scheduled source synchronization job.
type SyncJob struct {
	SourceID    string
	ScheduledAt time.Time
	Priority    Priority
	RetryCount  int
	// LastError records the most recent failure reason (empty if none)
	LastError string
	// index is managed by the heap
	index int
}

// jobHeap implements heap.Interface for SyncJobs.
type jobHeap []*SyncJob

func (h jobHeap) Len() int { return len(h) }
func (h jobHeap) Less(i, j int) bool {
	// Lower priority value = higher urgency (runs first)
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	// Tie-break: earliest scheduled time runs first
	return h[i].ScheduledAt.Before(h[j].ScheduledAt)
}
func (h jobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *jobHeap) Push(x interface{}) {
	job := x.(*SyncJob)
	job.index = len(*h)
	*h = append(*h, job)
}
func (h *jobHeap) Pop() interface{} {
	old := *h
	n := len(old)
	job := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	job.index = -1
	return job
}

// BackoffConfig controls retry backoff behavior.
type BackoffConfig struct {
	// BaseDelay is the initial backoff delay.
	BaseDelay time.Duration
	// MaxDelay is the maximum backoff delay.
	MaxDelay time.Duration
	// Multiplier is the exponential multiplier per retry.
	Multiplier float64
	// MaxRetries is the maximum number of retries (0 = unlimited).
	MaxRetries int
}

// DefaultBackoff is the default backoff configuration.
var DefaultBackoff = BackoffConfig{
	BaseDelay:  1 * time.Minute,
	MaxDelay:   60 * time.Minute,
	Multiplier: 2.0,
	MaxRetries: 5,
}

// NextDelay returns the backoff delay for the given retry count.
func (c BackoffConfig) NextDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return c.BaseDelay
	}
	delay := c.BaseDelay
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * c.Multiplier)
		if delay > c.MaxDelay {
			return c.MaxDelay
		}
	}
	return delay
}

// IsExhausted returns true if the job has exceeded the maximum retry count.
func (c BackoffConfig) IsExhausted(retryCount int) bool {
	return c.MaxRetries > 0 && retryCount >= c.MaxRetries
}

// Scheduler manages a priority queue of source sync jobs.
type Scheduler struct {
	mu      sync.Mutex
	heap    jobHeap
	waiting map[string]*SyncJob // sourceID → queued job
	backoff BackoffConfig
}

// NewScheduler creates a new smart scheduler.
func NewScheduler(backoff BackoffConfig) *Scheduler {
	s := &Scheduler{
		waiting: make(map[string]*SyncJob),
		backoff: backoff,
	}
	heap.Init(&s.heap)
	return s
}

// Enqueue adds or updates a job in the priority queue.
// If a job for the source already exists, it is replaced.
func (s *Scheduler) Enqueue(job *SyncJob) {
	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// If an existing job is already queued for this source, remove it
	if existing, ok := s.waiting[job.SourceID]; ok {
		s.removeFromHeap(existing)
	}

	heap.Push(&s.heap, job)
	s.waiting[job.SourceID] = job
}

// ScheduleRetry re-queues a failed job with exponential backoff.
// Returns false if the job has exceeded the max retry count.
func (s *Scheduler) ScheduleRetry(sourceID, errReason string) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	retryCount := 0
	if existing, ok := s.waiting[sourceID]; ok {
		retryCount = existing.RetryCount + 1
	}

	if s.backoff.IsExhausted(retryCount) {
		return false, 0
	}

	delay := s.backoff.NextDelay(retryCount)
	job := &SyncJob{
		SourceID:    sourceID,
		ScheduledAt: time.Now().Add(delay),
		Priority:    PriorityLow,
		RetryCount:  retryCount,
		LastError:   errReason,
	}
	if existing, ok := s.waiting[sourceID]; ok {
		s.removeFromHeap(existing)
	}
	heap.Push(&s.heap, job)
	s.waiting[sourceID] = job
	return true, delay
}

// Next returns the next job that is due to run, blocking until one is ready
// or the context is cancelled.
func (s *Scheduler) Next(ctx context.Context) (*SyncJob, error) {
	for {
		s.mu.Lock()
		if s.heap.Len() > 0 {
			top := s.heap[0]
			if !top.ScheduledAt.After(time.Now()) {
				job := heap.Pop(&s.heap).(*SyncJob)
				delete(s.waiting, job.SourceID)
				s.mu.Unlock()
				return job, nil
			}
			// Job is scheduled for the future — wait until it's due
			waitFor := time.Until(top.ScheduledAt)
			s.mu.Unlock()

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitFor):
				// Re-check
			}
			continue
		}
		s.mu.Unlock()

		// Queue is empty — wait for something to be enqueued
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Poll
		}
	}
}

// Len returns the number of pending jobs.
func (s *Scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heap.Len()
}

// PendingJobs returns a snapshot of all queued jobs.
func (s *Scheduler) PendingJobs() []*SyncJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]*SyncJob, len(s.heap))
	copy(jobs, s.heap)
	return jobs
}

// removeFromHeap removes a job from the heap (internal, must hold mu).
func (s *Scheduler) removeFromHeap(job *SyncJob) {
	if job.index >= 0 && job.index < s.heap.Len() {
		heap.Remove(&s.heap, job.index)
	}
	delete(s.waiting, job.SourceID)
}
