// Package nats provides a shared NATS JetStream client.
package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/globalcve/mono/internal/config"
)

// Client holds the NATS connection and JetStream context.
type Client struct {
	Conn *nats.Conn
	JS   jetstream.JetStream
}

// NewClient creates a new NATS client with JetStream enabled.
func NewClient(ctx context.Context, cfg config.NATSConfig) (*Client, error) {
	nc, err := nats.Connect(cfg.URL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
		nats.Timeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create jetstream: %w", err)
	}

	// Ensure streams exist
	if err := ensureStreams(ctx, js, cfg); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure streams: %w", err)
	}

	return &Client{Conn: nc, JS: js}, nil
}

// Close closes the NATS connection.
func (c *Client) Close() {
	if c.Conn != nil && !c.Conn.IsClosed() {
		c.Conn.Close()
	}
}

// ensureStreams creates JetStream streams if they don't exist.
func ensureStreams(ctx context.Context, js jetstream.JetStream, cfg config.NATSConfig) error {
	streams := []jetstream.StreamConfig{
		{
			Name:     cfg.StreamCVE,
			Subjects: []string{"cve.>"},
			MaxAge:   24 * time.Hour,
			Storage:  jetstream.FileStorage,
		},
		{
			Name:     cfg.StreamKEV,
			Subjects: []string{"kev.>"},
			MaxAge:   24 * time.Hour,
			Storage:  jetstream.FileStorage,
		},
		{
			Name:     cfg.StreamAlert,
			Subjects: []string{"alert.>"},
			MaxAge:   48 * time.Hour,
			Storage:  jetstream.FileStorage,
		},
	}

	for _, sc := range streams {
		if sc.Name == "" {
			continue
		}
		_, err := js.CreateOrUpdateStream(ctx, sc)
		if err != nil {
			return fmt.Errorf("create stream %s: %w", sc.Name, err)
		}
	}
	return nil
}
