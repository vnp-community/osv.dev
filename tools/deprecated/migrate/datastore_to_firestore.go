// migrate/datastore_to_firestore.go — Migrate Bug entities from Datastore to Firestore
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/firestore"
)

// Bug is a simplified representation of the Datastore Bug NDB model.
type Bug struct {
	Key           *datastore.Key `datastore:"__key__"`
	ID            string         `datastore:"id"`
	Status        string         `datastore:"status"`
	Source        string         `datastore:"source"`
	SourceID      string         `datastore:"source_id"`
	Public        bool           `datastore:"public"`
	ImportLastModified time.Time `datastore:"import_last_modified"`
	Timestamp     time.Time      `datastore:"timestamp"`
	LastModified  time.Time      `datastore:"last_modified"`
	RawJSONBlob   []byte         `datastore:"raw_json_blob,noindex"`
}

type MigrationJob struct {
	projectID string
	batchSize int
	workers   int
	dryRun    bool
	startFrom string // resume: last processed vuln_id
}

type MigrationProgress struct {
	ProcessedCount int64     `json:"processed_count"`
	ErrorCount     int64     `json:"error_count"`
	LastVulnID     string    `json:"last_vuln_id"`
	StartedAt      time.Time `json:"started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func main() {
	job := MigrationJob{}
	flag.StringVar(&job.projectID, "project", os.Getenv("GCP_PROJECT"), "GCP project ID")
	flag.IntVar(&job.batchSize, "batch-size", 500, "Documents per batch")
	flag.IntVar(&job.workers, "workers", 10, "Concurrent write workers")
	flag.BoolVar(&job.dryRun, "dry-run", false, "Dry run: read but don't write")
	flag.StringVar(&job.startFrom, "start-from", "", "Resume from vuln_id (exclusive)")
	flag.Parse()

	if job.projectID == "" {
		log.Fatal("--project or GCP_PROJECT env required")
	}

	ctx := context.Background()

	dsClient, err := datastore.NewClient(ctx, job.projectID)
	if err != nil {
		log.Fatalf("Datastore client: %v", err)
	}
	defer dsClient.Close()

	fsClient, err := firestore.NewClient(ctx, job.projectID)
	if err != nil {
		log.Fatalf("Firestore client: %v", err)
	}
	defer fsClient.Close()

	fmt.Printf("=== OSV Datastore → Firestore Migration ===\n")
	fmt.Printf("Project: %s | Batch: %d | Workers: %d | DryRun: %v\n\n",
		job.projectID, job.batchSize, job.workers, job.dryRun)

	var (
		processed int64
		errors    int64
		lastID    string
		mu        sync.Mutex
	)

	// Query all Bug entities from Datastore
	query := datastore.NewQuery("Bug").Order("__key__")
	it := dsClient.Run(ctx, query)

	type work struct {
		bugs []*Bug
	}
	ch := make(chan work, job.workers*2)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < job.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range ch {
				if job.dryRun {
					atomic.AddInt64(&processed, int64(len(w.bugs)))
					continue
				}
				// Batch write to Firestore
				batch := fsClient.BulkWriter(ctx)
				for _, bug := range w.bugs {
					vulnID := bug.Key.Name
					if vulnID == "" {
						vulnID = bug.ID
					}

					var rawData map[string]interface{}
					if len(bug.RawJSONBlob) > 0 {
						if err := json.Unmarshal(bug.RawJSONBlob, &rawData); err != nil {
							log.Printf("Failed to unmarshal %s: %v", vulnID, err)
						}
					}

					doc := map[string]interface{}{
						"vuln_id":       vulnID,
						"status":        bug.Status,
						"source":        bug.Source,
						"source_id":     bug.SourceID,
						"public":        bug.Public,
						"last_modified": bug.LastModified,
						"timestamp":     bug.Timestamp,
						"migrated_at":   time.Now(),
					}
					if rawData != nil {
						doc["raw_data"] = rawData
					}

					ref := fsClient.Collection("vulnerabilities").Doc(vulnID)
					batch.Set(ref, doc, firestore.MergeAll)
				}
				if _, err := batch.Flush(ctx); err != nil {
					log.Printf("Batch write error: %v", err)
					atomic.AddInt64(&errors, int64(len(w.bugs)))
				} else {
					count := atomic.AddInt64(&processed, int64(len(w.bugs)))
					mu.Lock()
					if len(w.bugs) > 0 {
						lastID = w.bugs[len(w.bugs)-1].Key.Name
					}
					mu.Unlock()
					if count%10000 == 0 {
						fmt.Printf("Progress: %d records migrated\n", count)
					}
				}
			}
		}()
	}

	// Read and batch from Datastore
	batch := make([]*Bug, 0, job.batchSize)
	for {
		var bug Bug
		_, err := it.Next(&bug)
		if err == datastore.Done {
			break
		}
		if err != nil {
			log.Printf("Iterator error: %v", err)
			break
		}

		batch = append(batch, &bug)
		if len(batch) >= job.batchSize {
			batchCopy := make([]*Bug, len(batch))
			copy(batchCopy, batch)
			ch <- work{bugs: batchCopy}
			batch = batch[:0]
		}
	}

	// Send remaining
	if len(batch) > 0 {
		ch <- work{bugs: batch}
	}

	close(ch)
	wg.Wait()

	fmt.Printf("\n=== Migration Complete ===\n")
	fmt.Printf("Processed: %d\n", atomic.LoadInt64(&processed))
	fmt.Printf("Errors:    %d\n", atomic.LoadInt64(&errors))
	fmt.Printf("Last ID:   %s\n", lastID)

	if errors > 0 {
		os.Exit(1)
	}
}
