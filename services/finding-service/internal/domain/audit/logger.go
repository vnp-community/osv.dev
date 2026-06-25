package audit

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
)

// ActionType categorizes the finding state change.
type ActionType string

const (
    ActionCreate         ActionType = "CREATE"
    ActionClose          ActionType = "CLOSE"
    ActionReopen         ActionType = "REOPEN"
    ActionMarkFP         ActionType = "MARK_FALSE_POSITIVE"
    ActionAcceptRisk     ActionType = "ACCEPT_RISK"
    ActionMarkOutOfScope ActionType = "MARK_OUT_OF_SCOPE"
    ActionMarkDuplicate  ActionType = "MARK_DUPLICATE"
    ActionUpdateSeverity ActionType = "UPDATE_SEVERITY"
    ActionAddTag         ActionType = "ADD_TAG"
    ActionAssign         ActionType = "ASSIGN"
    ActionComment        ActionType = "COMMENT"
)

// AuditEntry represents a single finding lifecycle event.
type AuditEntry struct {
    ID          uuid.UUID
    FindingID   uuid.UUID
    ActorID     *uuid.UUID  // nil for system actions
    ActorEmail  string
    Action      ActionType
    OldValue    string      // JSON representation of old state
    NewValue    string      // JSON representation of new state
    Comment     string
    IPAddress   string
    CreatedAt   time.Time
}

// AuditRepository defines storage for audit trail.
type AuditRepository interface {
    Save(ctx context.Context, entry *AuditEntry) error
    FindByFindingID(ctx context.Context, findingID uuid.UUID, limit, offset int) ([]*AuditEntry, error)
}

// Logger provides structured audit trail logging.
type Logger struct {
    repo    AuditRepository
    logger  zerolog.Logger
}

// New creates an AuditLogger.
func New(repo AuditRepository, logger zerolog.Logger) *Logger {
    return &Logger{repo: repo, logger: logger}
}

// Log records a finding state change to both the audit repository and structured logs.
func (l *Logger) Log(
    ctx context.Context,
    findingID uuid.UUID,
    actorID *uuid.UUID,
    actorEmail string,
    action ActionType,
    oldValue, newValue, comment, ipAddress string,
) error {
    entry := &AuditEntry{
        ID:         uuid.New(),
        FindingID:  findingID,
        ActorID:    actorID,
        ActorEmail: actorEmail,
        Action:     action,
        OldValue:   oldValue,
        NewValue:   newValue,
        Comment:    comment,
        IPAddress:  ipAddress,
        CreatedAt:  time.Now().UTC(),
    }

    // Structured log for observability
    event := l.logger.Info().
        Str("finding_id", findingID.String()).
        Str("action", string(action)).
        Str("actor", actorEmail).
        Str("ip", ipAddress)

    if actorID != nil {
        event = event.Str("actor_id", actorID.String())
    }
    if comment != "" {
        event = event.Str("comment", comment)
    }
    event.Msg("finding audit event")

    // Persist to repository
    return l.repo.Save(ctx, entry)
}

// LogCreate records finding creation.
func (l *Logger) LogCreate(ctx context.Context, findingID uuid.UUID, actorID *uuid.UUID, email string) error {
    return l.Log(ctx, findingID, actorID, email, ActionCreate, "", "", "", "")
}

// LogClose records finding closure with mitigator details.
func (l *Logger) LogClose(ctx context.Context, findingID uuid.UUID, actorID *uuid.UUID, email, comment string) error {
    return l.Log(ctx, findingID, actorID, email, ActionClose, "active", "mitigated", comment, "")
}

// LogReopen records finding reopening.
func (l *Logger) LogReopen(ctx context.Context, findingID uuid.UUID, actorID *uuid.UUID, email, comment string) error {
    return l.Log(ctx, findingID, actorID, email, ActionReopen, "mitigated", "active", comment, "")
}

// LogFalsePositive records false positive marking.
func (l *Logger) LogFalsePositive(ctx context.Context, findingID uuid.UUID, actorID *uuid.UUID, email, comment string) error {
    return l.Log(ctx, findingID, actorID, email, ActionMarkFP, "active", "false_positive", comment, "")
}

// History returns the full audit trail for a finding.
func (l *Logger) History(ctx context.Context, findingID uuid.UUID, limit, offset int) ([]*AuditEntry, error) {
    return l.repo.FindByFindingID(ctx, findingID, limit, offset)
}
