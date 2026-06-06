package finding

import (
	"time"

	"github.com/google/uuid"
)

// helper functions used by state_machine.go

func nowUTC() time.Time { return time.Now().UTC() }

func mustParseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
