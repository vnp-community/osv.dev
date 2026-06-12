// Package events cung cấp NATS JetStream setup và subject constants.
package events

import (
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	// StreamName là tên NATS JetStream stream cho toàn bộ OpenVulnScan.
	StreamName = "OPENVULNSCAN"
)

// SetupJetStream tạo JetStream stream nếu chưa tồn tại.
func SetupJetStream(nc *nats.Conn) (nats.JetStreamContext, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:       StreamName,
		Subjects:   []string{"ovs.>"},
		Storage:    nats.FileStorage,
		Replicas:   1,
		MaxAge:     7 * 24 * time.Hour,
		MaxMsgs:    1_000_000,
		MaxBytes:   5 * 1024 * 1024 * 1024, // 5GB
		Retention:  nats.LimitsPolicy,
		Duplicates: 5 * time.Minute,
	})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return nil, err
	}
	return js, nil
}
