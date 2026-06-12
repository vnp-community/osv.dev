// cmd/controller/main.go — Version Index Controller (Enterprise Edition)
// Detects repositories that need re-indexing and publishes IndexingTask messages to NATS.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	natsgo "github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

const (
	indexingTaskSubject = "osv.version.index.tasks"
	controllerInterval  = 1 * time.Hour
)

// IndexingTask is the message sent to workers.
type IndexingTask struct {
	RepoURL string `json:"repo_url"`
	Tag     string `json:"tag"`
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "version-index-controller").Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── OTel ────────────────────────────────────────────────────────────
	otelShutdown := observability.MustSetup("version-index-controller", "1.0.0", log)
	defer otelShutdown()

	projectID := envStr("GCP_PROJECT_ID", "osv-dev")
	natsURL := envStr("NATS_URL", "nats://localhost:4222")
	httpPort := envStr("HTTP_PORT", "8081")

	// ── Firestore ────────────────────────────────────────────────────────────
	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal().Err(err).Msg("firestore init failed")
	}
	defer fsClient.Close()

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := natsgo.Connect(natsURL, natsgo.MaxReconnects(-1))
	if err != nil {
		log.Fatal().Err(err).Msg("NATS connect failed")
	}
	defer nc.Drain() //nolint:errcheck

	js, err := natsjs.New(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("JetStream init failed")
	}

	// ── Controller loop ───────────────────────────────────────────────────────
	go func() {
		ticker := time.NewTicker(controllerInterval)
		defer ticker.Stop()

		// Run immediately on start
		runScan(ctx, fsClient, js, log)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runScan(ctx, fsClient, js, log)
			}
		}
	}()

	// ── HTTP Health (pkg/health MultiChecker) ────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.NATSProber(nc),
		health.NewFirestoreProber(func(ctx context.Context) error {
			_, err := fsClient.Collection("health").Doc("probe").Get(ctx)
			if err != nil && err.Error() != "rpc error: code = NotFound" {
				return err
			}
			return nil
		}),
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))
	httpSrv := &http.Server{Addr: fmt.Sprintf(":%s", httpPort), Handler: mux}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP error")
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	log.Info().Msg("controller shutdown")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx) //nolint:errcheck
}

// runScan queries Firestore for repo configs and publishes tasks for unindexed tags.
func runScan(ctx context.Context, fs *firestore.Client, js natsjs.JetStream, log zerolog.Logger) {
	log.Info().Msg("controller: scanning repos for new tags...")

	// Load repo configs from Firestore (repo_configs collection)
	docs, err := fs.Collection("repo_configs").Documents(ctx).GetAll()
	if err != nil {
		log.Error().Err(err).Msg("failed to load repo configs")
		return
	}

	tasks := 0
	for _, doc := range docs {
		data := doc.Data()
		repoURL, _ := data["repo_url"].(string)
		if repoURL == "" {
			continue
		}

		// Get latest tags from repo_indexes that are missing
		// For now: publish a scan task — worker will check what's indexed
		task := fmt.Sprintf(`{"repo_url":%q,"tag":"latest"}`, repoURL)
		if _, err := js.Publish(ctx, indexingTaskSubject, []byte(task)); err != nil {
			log.Warn().Err(err).Str("repo", repoURL).Msg("publish task failed")
			continue
		}
		tasks++
	}

	log.Info().Int("tasks", tasks).Msg("controller: scan complete")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
