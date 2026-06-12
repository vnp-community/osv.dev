// Package nats provides NATS JetStream integration for DefectDojo microservices.
// All inter-service events follow the CloudEvents v1.0 specification.
package nats

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CloudEvent represents a CloudEvents v1.0 compliant event envelope.
// All DefectDojo inter-service events use this format.
type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`     // always "1.0"
	Type            string          `json:"type"`            // e.g. "defectdojo.finding.created"
	Source          string          `json:"source"`          // e.g. "finding-management/v1"
	ID              string          `json:"id"`              // UUID v4
	Time            time.Time       `json:"time"`            // RFC3339
	DataContentType string          `json:"datacontenttype"` // always "application/json"
	Data            json.RawMessage `json:"data"`            // event-specific payload
}

// New creates a new CloudEvent with the given type, source, and data payload.
func New(eventType, source string, data interface{}) (*CloudEvent, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &CloudEvent{
		SpecVersion:     "1.0",
		Type:            eventType,
		Source:          source,
		ID:              uuid.NewString(),
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
		Data:            payload,
	}, nil
}

// UnmarshalData parses the Data field into the given target struct.
func (e *CloudEvent) UnmarshalData(target interface{}) error {
	return json.Unmarshal(e.Data, target)
}
