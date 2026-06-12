// Package savedsearch implements saved searches and alert subscriptions.
// TASK-07-04: SavedSearch domain entity + alert threshold triggers.
package savedsearch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AlertThreshold defines when a saved search triggers an alert.
type AlertThreshold struct {
	// MinResults triggers an alert when a search returns >= MinResults new results.
	MinResults int
	// MaxCVSSScore triggers an alert when any result has CVSS >= this value (0 = disabled).
	MaxCVSSScore float64
	// RequireKEV triggers an alert only for results that are in the KEV catalog.
	RequireKEV bool
}

// SavedSearch is a named, persisted query with optional alert settings.
type SavedSearch struct {
	ID          string
	Name        string
	Description string
	OwnerID     string

	// SearchQuery is the serialised search query (JSON or URL-encoded).
	SearchQuery string
	// Facets lists facet dimensions to include in results.
	Facets []string

	// Alert settings (nil = no alerting)
	Alert *AlertThreshold

	// Scheduling
	Schedule string // cron expression, e.g. "0 8 * * 1" = every Monday 8am

	CreatedAt time.Time
	UpdatedAt time.Time
	LastRunAt time.Time
	// LastResultCount is the number of results from the last run.
	LastResultCount int
}

// AlertEvent is emitted when a saved search triggers an alert.
type AlertEvent struct {
	SavedSearchID   string
	SavedSearchName string
	OwnerID         string
	ResultCount     int
	HighCVSS        []string // CVE IDs with high CVSS scores
	KEVEntries      []string // CVE IDs that are in KEV
	TriggeredAt     time.Time
}

// Repository is the storage interface for saved searches.
type Repository interface {
	// Save creates or updates a saved search.
	Save(ctx context.Context, ss *SavedSearch) error
	// Get retrieves a saved search by ID.
	Get(ctx context.Context, id string) (*SavedSearch, error)
	// ListByOwner returns all saved searches for a given owner.
	ListByOwner(ctx context.Context, ownerID string) ([]*SavedSearch, error)
	// Delete removes a saved search.
	Delete(ctx context.Context, id string) error
	// ListDue returns searches whose schedule is due at the given time.
	ListDue(ctx context.Context, at time.Time) ([]*SavedSearch, error)
}

// AlertPublisher emits alert events.
type AlertPublisher interface {
	Publish(ctx context.Context, event AlertEvent) error
}

// SearchResult is the output from a search execution.
type SearchResult struct {
	CVEID     string
	CVSSScore float64
	IsKEV     bool
}

// SearchExecutor runs a saved search query.
type SearchExecutor interface {
	Execute(ctx context.Context, query string) ([]SearchResult, error)
}

// Service provides saved search management and alerting.
type Service struct {
	repo      Repository
	publisher AlertPublisher
	executor  SearchExecutor
}

// NewService creates a new SavedSearch service.
func NewService(repo Repository, publisher AlertPublisher, executor SearchExecutor) *Service {
	return &Service{repo: repo, publisher: publisher, executor: executor}
}

// Create creates a new saved search.
func (s *Service) Create(ctx context.Context, ss *SavedSearch) (*SavedSearch, error) {
	if ss.Name == "" {
		return nil, fmt.Errorf("saved search name is required")
	}
	if ss.SearchQuery == "" {
		return nil, fmt.Errorf("saved search query is required")
	}
	if ss.ID == "" {
		ss.ID = uuid.NewString()
	}
	now := time.Now()
	ss.CreatedAt = now
	ss.UpdatedAt = now

	if err := s.repo.Save(ctx, ss); err != nil {
		return nil, fmt.Errorf("save search: %w", err)
	}
	return ss, nil
}

// Get retrieves a saved search.
func (s *Service) Get(ctx context.Context, id string) (*SavedSearch, error) {
	return s.repo.Get(ctx, id)
}

// List returns all saved searches for a given owner.
func (s *Service) List(ctx context.Context, ownerID string) ([]*SavedSearch, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

// Delete removes a saved search.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Execute runs a saved search, checks alert thresholds, and publishes alerts.
func (s *Service) Execute(ctx context.Context, id string) (*SavedSearch, []SearchResult, error) {
	ss, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get saved search %q: %w", id, err)
	}

	results, err := s.executor.Execute(ctx, ss.SearchQuery)
	if err != nil {
		return ss, nil, fmt.Errorf("execute search %q: %w", id, err)
	}

	// Update run stats
	ss.LastRunAt = time.Now()
	ss.LastResultCount = len(results)
	ss.UpdatedAt = ss.LastRunAt
	_ = s.repo.Save(ctx, ss)

	// Check alert threshold
	if ss.Alert != nil && s.publisher != nil {
		s.checkAndAlert(ctx, ss, results)
	}

	return ss, results, nil
}

// RunDue executes all saved searches whose schedule is currently due.
func (s *Service) RunDue(ctx context.Context, at time.Time) (int, error) {
	due, err := s.repo.ListDue(ctx, at)
	if err != nil {
		return 0, fmt.Errorf("list due searches: %w", err)
	}
	ran := 0
	for _, ss := range due {
		if _, _, err := s.Execute(ctx, ss.ID); err == nil {
			ran++
		}
	}
	return ran, nil
}

// checkAndAlert evaluates thresholds and publishes alerts when triggered.
func (s *Service) checkAndAlert(ctx context.Context, ss *SavedSearch, results []SearchResult) {
	threshold := ss.Alert
	triggered := false

	event := AlertEvent{
		SavedSearchID:   ss.ID,
		SavedSearchName: ss.Name,
		OwnerID:         ss.OwnerID,
		ResultCount:     len(results),
		TriggeredAt:     time.Now(),
	}

	if threshold.MinResults > 0 && len(results) >= threshold.MinResults {
		triggered = true
	}

	for _, r := range results {
		if threshold.MaxCVSSScore > 0 && r.CVSSScore >= threshold.MaxCVSSScore {
			event.HighCVSS = append(event.HighCVSS, r.CVEID)
			triggered = true
		}
		if threshold.RequireKEV && r.IsKEV {
			event.KEVEntries = append(event.KEVEntries, r.CVEID)
			triggered = true
		}
	}

	if triggered {
		_ = s.publisher.Publish(ctx, event)
	}
}

// ---- In-Memory Repository (for dev/testing) ----

// InMemoryRepository implements Repository using an in-memory store.
type InMemoryRepository struct {
	mu      sync.RWMutex
	searches map[string]*SavedSearch
}

// NewInMemoryRepository creates a new in-memory saved search repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{searches: make(map[string]*SavedSearch)}
}

func (r *InMemoryRepository) Save(_ context.Context, ss *SavedSearch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Deep copy to prevent mutation
	cp := *ss
	r.searches[ss.ID] = &cp
	return nil
}

func (r *InMemoryRepository) Get(_ context.Context, id string) (*SavedSearch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ss, ok := r.searches[id]
	if !ok {
		return nil, fmt.Errorf("saved search %q not found", id)
	}
	cp := *ss
	return &cp, nil
}

func (r *InMemoryRepository) ListByOwner(_ context.Context, ownerID string) ([]*SavedSearch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*SavedSearch
	for _, ss := range r.searches {
		if ss.OwnerID == ownerID {
			cp := *ss
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (r *InMemoryRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.searches[id]; !ok {
		return fmt.Errorf("saved search %q not found", id)
	}
	delete(r.searches, id)
	return nil
}

func (r *InMemoryRepository) ListDue(_ context.Context, _ time.Time) ([]*SavedSearch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*SavedSearch
	for _, ss := range r.searches {
		if ss.Schedule != "" {
			cp := *ss
			result = append(result, &cp)
		}
	}
	return result, nil
}
