package finding

import "errors"

var (
    ErrTitleRequired          = errors.New("finding title is required")
    ErrInvalidSeverity        = errors.New("invalid severity: must be Critical, High, Medium, Low, or Info")
    ErrTestIDRequired         = errors.New("test ID is required")
    ErrEngagementIDRequired   = errors.New("engagement ID is required")
    ErrProductIDRequired      = errors.New("product ID is required")
    ErrNotActive              = errors.New("finding must be active to perform this action")
    ErrAlreadyActive          = errors.New("finding is already active")
    ErrAlreadyMitigated       = errors.New("finding is already mitigated")
    ErrCannotCloseDuplicate   = errors.New("duplicate findings cannot be closed directly")
    ErrCannotReopenDuplicate  = errors.New("duplicate findings cannot be reopened")
    ErrCannotModifyDuplicate  = errors.New("duplicate findings cannot be modified")
    ErrFindingNotFound        = errors.New("finding not found")
)
