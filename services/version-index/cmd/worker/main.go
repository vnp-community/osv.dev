// cmd/worker/main.go — Version Index Worker
// Consumes IndexingTask messages and executes the full repo indexing pipeline.
package main

import (
	"context"
	"encoding/json"
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
	"go.opentelemetry.io/otel"

	"github.com/osv/version-index/internal/application/command/index_repository"
	fsrepo "github.com/osv/version-index/internal/infra/persistence/firestore"
	"github.com/osv/version-index/internal/domain/service"
)

const (
	streamName      = "OSV_TASKS"
	consumerName    = "version-index-worker"
	filterSubject   = "osv.version.index.tasks"
	maxConcurrency  = 10 // max concurrent indexing jobs
)

type indexingTask struct {
	RepoURL string `json:"repo_url"`
	Tag     string `json:"tag"`
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "version-index-worker").Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	projectID := envStr("GCP_PROJECT_ID", "osv-dev")
	natsURL := envStr("NATS_URL", "nats://localhost:4222")
	cloneDir := envStr("CLONE_DIR", "/tmp/version-index-repos")
	httpPort := envStr("HTTP_PORT", "8082")

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

	// ── Wiring ───────────────────────────────────────────────────────────────
	tracer := otel.Tracer("version-index-worker")
	hasher := &service.BucketHasher{}

	bucketRepo := fsrepo.NewRepoIndexBucketRepo(fsClient, log)
	indexRepo := fsrepo.NewRepoIndexRepo(fsClient, log)

	handler := index_repository.NewHandler(bucketRepo, indexRepo, hasher, cloneDir, tracer, log)

	// ── NATS Consumer ─────────────────────────────────────────────────────────
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, natsjs.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: filterSubject,
		AckPolicy:     natsjs.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       30 * time.Minute, // large repos take time
		DeliverPolicy: natsjs.DeliverNewPolicy,
		MaxAckPending: maxConcurrency,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("create consumer failed")
	}

	msgs, err := consumer.Messages(natsjs.PullMaxMessages(maxConcurrency))
	if err != nil {
		log.Fatal().Err(err).Msg("subscribe failed")
	}

	log.Info().Int("concurrency", maxConcurrency).Msg("worker started")

	// Semaphore to limit concurrent indexing jobs
	sem := make(chan struct{}, maxConcurrency)

	go func() {
		<-ctx.Done()
		msgs.Stop()
	}()

	go func() {
		for {
			msg, err := msgs.Next()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Error().Err(err).Msg("next message error")
				return
			}

			sem <- struct{}{}
			go func(m natsjs.Msg) {
				defer func() { <-sem }()

				var task indexingTask
				if err := json.Unmarshal(m.Data(), &task); err != nil {
					log.Warn().Err(err).Msg("invalid task, acking")
					m.Ack() //nolint:errcheck
					return
				}

				log.Info().Str("repo", task.RepoURL).Str("tag", task.Tag).Msg("indexing task received")

				jobCtx, cancel := context.WithTimeout(ctx, 25*time.Minute)
				defer cancel()

				if err := handler.Handle(jobCtx, index_repository.Command{
					RepoURL: task.RepoURL,
					Tag:     task.Tag,
				}); err != nil {
					log.Error().Err(err).Str("repo", task.RepoURL).Str("tag", task.Tag).Msg("indexing failed")
					m.Nak() //nolint:errcheck
				} else {
					m.Ack() //nolint:errcheck
				}
			}(msg)
		}
	}()

	// ── HTTP Health ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if nc.IsConnected() {
			fmt.Fprintf(w, `{"status":"ready","role":"worker"}`)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"not_ready"}`)
		}
	})
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
	log.Info().Msg("worker shutdown")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx) //nolint:errcheck
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
