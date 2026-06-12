package batch_enrich

import (
	"context"
	"sync"
)

// UseCase handles bulk CVE enrichment asynchronously
type UseCase struct {
	maxConcurrency int
}

// New creates a new batch enrich usecase
func New(maxConcurrency int) *UseCase {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	return &UseCase{maxConcurrency: maxConcurrency}
}

// Result holds the result of a single CVE enrichment
type Result struct {
	CVEID string
	Err   error
}

// ExecuteAsync enriches a list of CVE IDs in parallel with rate limiting
func (uc *UseCase) ExecuteAsync(ctx context.Context, cveIDs []string) []Result {
	sem := make(chan struct{}, uc.maxConcurrency)
	results := make([]Result, len(cveIDs))
	var wg sync.WaitGroup

	for i, cveID := range cveIDs {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// TODO: call enrich_cve usecase per CVE
			results[idx] = Result{CVEID: id}
		}(i, cveID)
	}
	wg.Wait()
	return results
}
