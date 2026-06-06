// Package worker provides a concurrent worker pool for executing scan jobs.
package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ScanJob represents a scan task to be executed by a worker.
type ScanJob struct {
	ScanID uuid.UUID
	UserID uuid.UUID
}

// ErrQueueFull is returned when the job queue is at capacity.
var ErrQueueFull = fmt.Errorf("scan queue is full")

// ExecuteScanFunc is the function signature for executing a scan job.
type ExecuteScanFunc func(ctx context.Context, job ScanJob) error

// WorkerPool manages a pool of goroutines that execute scan jobs.
type WorkerPool struct {
	maxWorkers    int
	jobQueue      chan ScanJob
	cancellations sync.Map // scanID (string) → context.CancelFunc
	executeFunc   ExecuteScanFunc
	log           zerolog.Logger
	wg            sync.WaitGroup
}

// NewWorkerPool creates a WorkerPool with the given concurrency limit.
func NewWorkerPool(maxWorkers int, executeFunc ExecuteScanFunc, log zerolog.Logger) *WorkerPool {
	return &WorkerPool{
		maxWorkers:  maxWorkers,
		jobQueue:    make(chan ScanJob, maxWorkers*4), // buffer 4x workers
		executeFunc: executeFunc,
		log:         log,
	}
}

// Start launches the worker goroutines. Blocks until ctx is cancelled.
func (p *WorkerPool) Start(ctx context.Context) {
	p.log.Info().Int("workers", p.maxWorkers).Msg("worker pool starting")
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	<-ctx.Done()
	p.log.Info().Msg("worker pool shutting down")
	p.wg.Wait()
	p.log.Info().Msg("worker pool stopped")
}

// Submit enqueues a scan job. Returns ErrQueueFull if the queue is at capacity.
func (p *WorkerPool) Submit(job ScanJob) error {
	select {
	case p.jobQueue <- job:
		p.log.Debug().Stringer("scan_id", job.ScanID).Msg("job enqueued")
		return nil
	default:
		return ErrQueueFull
	}
}

// Cancel cancels an in-progress or queued scan by its ID.
func (p *WorkerPool) Cancel(scanID uuid.UUID) {
	if fn, ok := p.cancellations.LoadAndDelete(scanID.String()); ok {
		fn.(context.CancelFunc)()
		p.log.Info().Stringer("scan_id", scanID).Msg("scan cancelled")
	}
}

func (p *WorkerPool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	p.log.Debug().Int("worker_id", id).Msg("worker started")

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}
			p.runJob(ctx, job, id)
		}
	}
}

func (p *WorkerPool) runJob(parentCtx context.Context, job ScanJob, workerID int) {
	jobCtx, cancel := context.WithCancel(parentCtx)
	p.cancellations.Store(job.ScanID.String(), cancel)
	defer func() {
		cancel()
		p.cancellations.Delete(job.ScanID.String())
	}()

	p.log.Info().Stringer("scan_id", job.ScanID).Int("worker", workerID).Msg("executing scan")
	if err := p.executeFunc(jobCtx, job); err != nil {
		p.log.Error().Err(err).Stringer("scan_id", job.ScanID).Msg("scan failed")
	}
}
