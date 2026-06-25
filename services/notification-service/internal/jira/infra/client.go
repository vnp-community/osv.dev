// Package jira_client re-exports the JIRA infra client from integrations/jira/infra.
// This alias package supports the import path
// github.com/osv/notification-service/internal/jira/infra
package jira_client

import (
	jinfra "github.com/osv/notification-service/internal/integrations/jira/infra"
)

// Re-export all types and functions from integrations/jira/infra.
type Client = jinfra.Client
type CreateIssueRequest = jinfra.CreateIssueRequest
type IssueFields = jinfra.IssueFields
type CreateIssueResponse = jinfra.CreateIssueResponse
type IssueTransition = jinfra.IssueTransition

// New creates a JIRA Client with the given credentials.
var New = jinfra.New

// ADFDescription builds an Atlassian Document Format description from plaintext.
var ADFDescription = jinfra.ADFDescription
