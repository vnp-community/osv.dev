// setup-nats-streams.go — Create NATS JetStream streams and durable consumers
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	ctx := context.Background()

	// ── Create Streams ──────────────────────────────────────────────────────────

	// OSV-EVENTS: main domain events stream
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "OSV-EVENTS",
		Description: "All OSV domain events",
		Subjects:    []string{"osv.>"},
		MaxAge:      7 * 24 * time.Hour,
		Storage:     jetstream.FileStorage,
		Replicas:    3,
		MaxBytes:    50 * 1024 * 1024 * 1024, // 50GB
		Retention:   jetstream.LimitsPolicy,
		Discard:     jetstream.DiscardOld,
	})
	if err != nil {
		log.Fatalf("Failed to create OSV-EVENTS stream: %v", err)
	}
	fmt.Println("✓ Stream OSV-EVENTS created")

	// OSV-DLQ: dead letter queue for failed messages
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "OSV-DLQ",
		Description: "Dead letter queue for failed OSV messages",
		Subjects:    []string{"osv.dlq.>"},
		MaxAge:      30 * 24 * time.Hour,
		Storage:     jetstream.FileStorage,
		Replicas:    3,
	})
	if err != nil {
		log.Fatalf("Failed to create OSV-DLQ stream: %v", err)
	}
	fmt.Println("✓ Stream OSV-DLQ created")

	// ── Create Durable Consumers ────────────────────────────────────────────────

	consumers := []struct {
		name          string
		filterSubject string
		maxDeliver    int
		ackWait       time.Duration
	}{
		{
			name:          "ingestion-service",
			filterSubject: "osv.source.change.>",
			maxDeliver:    5,
			ackWait:       30 * time.Minute,
		},
		{
			name:          "impact-analysis",
			filterSubject: "osv.vuln.imported",
			maxDeliver:    3,
			ackWait:       30 * time.Minute,
		},
		{
			name:          "ai-enrichment",
			filterSubject: "osv.vuln.imported",
			maxDeliver:    3,
			ackWait:       5 * time.Minute,
		},
		{
			name:          "search-indexer",
			filterSubject: "osv.vuln.>",
			maxDeliver:    5,
			ackWait:       2 * time.Minute,
		},
		{
			name:          "notification",
			filterSubject: "osv.vuln.>",
			maxDeliver:    10,
			ackWait:       30 * time.Second,
		},
		{
			name:          "alias-service",
			filterSubject: "osv.vuln.imported",
			maxDeliver:    3,
			ackWait:       5 * time.Minute,
		},
	}

	for _, c := range consumers {
		_, err = js.CreateOrUpdateConsumer(ctx, "OSV-EVENTS", jetstream.ConsumerConfig{
			Name:           c.name,
			Durable:        c.name,
			FilterSubject:  c.filterSubject,
			MaxDeliver:     c.maxDeliver,
			AckWait:        c.ackWait,
			AckPolicy:      jetstream.AckExplicitPolicy,
			DeliverPolicy:  jetstream.DeliverNewPolicy,
			MaxAckPending:  1000,
			BackOff: []time.Duration{
				1 * time.Second,
				5 * time.Second,
				30 * time.Second,
			},
		})
		if err != nil {
			log.Printf("⚠ Failed to create consumer %s: %v", c.name, err)
			continue
		}
		fmt.Printf("✓ Consumer %s created (filter: %s, maxDeliver: %d)\n",
			c.name, c.filterSubject, c.maxDeliver)
	}

	fmt.Println("\n✓ NATS JetStream setup complete!")
}
