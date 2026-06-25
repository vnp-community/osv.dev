// Package domain re-exports the JIRA integration domain entities from integrations/jira/domain.
// This alias package exists to support the import path
// github.com/osv/notification-service/internal/jira/domain
// which is referenced by the integrations/jira/usecase package.
package domain

import (
	jdomain "github.com/osv/notification-service/internal/integrations/jira/domain"
)

// Re-export all types from integrations/jira/domain.
type JIRAConfig = jdomain.JIRAConfig
type JIRAIssueMapping = jdomain.JIRAIssueMapping
type JIRAConfigRepository = jdomain.JIRAConfigRepository
type JIRAIssueMappingRepository = jdomain.JIRAIssueMappingRepository
