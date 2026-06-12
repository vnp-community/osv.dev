// Package app cung cấp ServiceRunner interface và Registry quản lý goroutine lifecycle.
package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

// ServiceRunner là interface mà mỗi service goroutine phải implement.
type ServiceRunner interface {
	// Name trả về tên duy nhất của service (dùng cho logging và health check).
	Name() string

	// Run khởi động service và block cho đến khi ctx bị cancel hoặc có lỗi.
	// Trả về nil khi shutdown gracefully, error khi có sự cố.
	Run(ctx context.Context) error

	// Health kiểm tra xem service có đang hoạt động đúng không.
	Health(ctx context.Context) error
}

type serviceState string

const (
	stateIdle    serviceState = "idle"
	stateRunning serviceState = "running"
	stateStopped serviceState = "stopped"
	stateFailed  serviceState = "failed"
)

type entry struct {
	runner ServiceRunner
	state  serviceState
	err    error
	mu     sync.RWMutex
}

func (e *entry) setState(s serviceState) {
	e.mu.Lock()
	e.state = s
	e.mu.Unlock()
}

func (e *entry) setError(err error) {
	e.mu.Lock()
	e.err = err
	e.mu.Unlock()
}

func (e *entry) getState() serviceState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// Registry quản lý vòng đời của tất cả service goroutines.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*entry
	wg      sync.WaitGroup
	log     zerolog.Logger
}

// NewRegistry tạo Registry mới.
func NewRegistry(l zerolog.Logger) *Registry {
	return &Registry{
		entries: make(map[string]*entry),
		log:     l,
	}
}

// Register đăng ký một ServiceRunner. Phải gọi trước Start.
// Nếu runner đã registered thì skip (idempotent).
func (r *Registry) Register(runner ServiceRunner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[runner.Name()]; exists {
		return // idempotent
	}
	r.entries[runner.Name()] = &entry{runner: runner, state: stateIdle}
	r.log.Debug().Str("svc", runner.Name()).Msg("service registered")
}

// Start khởi động tất cả runners chưa được start như goroutines độc lập.
func (r *Registry) Start(ctx context.Context) {
	r.mu.RLock()
	runners := make([]*entry, 0, len(r.entries))
	for _, e := range r.entries {
		runners = append(runners, e)
	}
	r.mu.RUnlock()

	for _, e := range runners {
		// Chỉ start runner chưa start
		if e.getState() != stateIdle {
			continue
		}

		e := e
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			defer r.recoverPanic(e)

			e.setState(stateRunning)
			r.log.Info().Str("svc", e.runner.Name()).Msg("goroutine started")

			err := e.runner.Run(ctx)
			if err != nil && err != context.Canceled {
				e.setError(err)
				e.setState(stateFailed)
				r.log.Error().Str("svc", e.runner.Name()).Err(err).Msg("goroutine failed")
			} else {
				e.setState(stateStopped)
				r.log.Info().Str("svc", e.runner.Name()).Msg("goroutine stopped cleanly")
			}
		}()
	}
}

// Wait block cho đến khi tất cả goroutines kết thúc.
func (r *Registry) Wait() { r.wg.Wait() }

// HealthAll chạy health check đồng thời cho tất cả services.
func (r *Registry) HealthAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	snap := make(map[string]*entry, len(r.entries))
	for k, v := range r.entries {
		snap[k] = v
	}
	r.mu.RUnlock()

	out := make(map[string]error, len(snap))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, e := range snap {
		wg.Add(1)
		go func(n string, en *entry) {
			defer wg.Done()
			var err error
			if en.getState() == stateRunning {
				err = en.runner.Health(ctx)
			} else {
				err = fmt.Errorf("service not running (state: %s)", en.getState())
			}
			mu.Lock()
			out[n] = err
			mu.Unlock()
		}(name, e)
	}
	wg.Wait()
	return out
}

// Status trả về trạng thái của tất cả services.
func (r *Registry) Status() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := make(map[string]string, len(r.entries))
	for k, v := range r.entries {
		s[k] = string(v.getState())
	}
	return s
}

func (r *Registry) recoverPanic(e *entry) {
	if rec := recover(); rec != nil {
		err := fmt.Errorf("panic: %v", rec)
		e.setError(err)
		e.setState(stateFailed)
		r.log.Error().
			Str("svc", e.runner.Name()).
			Interface("panic", rec).
			Msg("goroutine panicked — recovered")
	}
}
