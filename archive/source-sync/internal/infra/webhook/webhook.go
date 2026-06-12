// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package webhook implements HTTP handlers for receiving push notifications
// from source repositories (GitHub, GitLab, etc.), enabling near-realtime
// CVE feed updates instead of relying solely on scheduled polling.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// SyncTrigger is the interface used by the webhook handler to trigger source syncs.
// Implementations publish to NATS or call the source-sync scheduler directly.
type SyncTrigger interface {
	// TriggerSync enqueues an immediate sync for the named source.
	TriggerSync(sourceName string, reason string) error
}

// SourceResolver maps a repository URL to a source name in source.yaml.
type SourceResolver interface {
	// ResolveByRepoURL returns the source name for a given repo URL.
	// Returns ("", false) if no matching source is configured.
	ResolveByRepoURL(repoURL string) (string, bool)
}

// Handler handles incoming webhook requests from GitHub and GitLab.
type Handler struct {
	trigger       SyncTrigger
	resolver      SourceResolver
	githubSecret  string // HMAC-SHA256 secret for GitHub webhook validation
	gitlabToken   string // X-Gitlab-Token for GitLab webhook validation
	log           zerolog.Logger
}

// Config holds Handler configuration.
type Config struct {
	GitHubSecret string
	GitLabToken  string
}

// NewHandler creates a new webhook Handler.
func NewHandler(trigger SyncTrigger, resolver SourceResolver, cfg Config, log zerolog.Logger) *Handler {
	return &Handler{
		trigger:      trigger,
		resolver:     resolver,
		githubSecret: cfg.GitHubSecret,
		gitlabToken:  cfg.GitLabToken,
		log:          log,
	}
}

// RegisterRoutes registers webhook endpoints on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /webhooks/github", h.handleGitHub)
	mux.HandleFunc("POST /webhooks/gitlab", h.handleGitLab)
}

// ── GitHub Webhook ────────────────────────────────────────────────────────────

// githubPushEvent is the subset of GitHub's push event payload we care about.
type githubPushEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	HeadCommit struct {
		ID      string    `json:"id"`
		Message string    `json:"message"`
		Added   []string  `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
		Timestamp time.Time `json:"timestamp"`
	} `json:"head_commit"`
}

func (h *Handler) handleGitHub(w http.ResponseWriter, r *http.Request) {
	// Validate event type
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		// Accept but ignore non-push events (e.g., ping)
		h.log.Debug().Str("event", event).Msg("github: ignoring non-push event")
		w.WriteHeader(http.StatusOK)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		h.log.Error().Err(err).Msg("github: failed to read request body")
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Validate HMAC signature
	if h.githubSecret != "" {
		if err := validateGitHubSignature(r.Header.Get("X-Hub-Signature-256"), body, h.githubSecret); err != nil {
			h.log.Warn().Err(err).Msg("github: invalid signature")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload githubPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		h.log.Error().Err(err).Msg("github: failed to parse push event")
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Only trigger on default branch pushes
	if !isDefaultBranchRef(payload.Ref) {
		h.log.Debug().Str("ref", payload.Ref).Msg("github: ignoring non-default branch push")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Resolve source from repo URL
	repoURL := payload.Repository.CloneURL
	sourceName, ok := h.resolver.ResolveByRepoURL(repoURL)
	if !ok {
		// Try SSH URL as fallback
		sourceName, ok = h.resolver.ResolveByRepoURL(payload.Repository.SSHURL)
	}
	if !ok {
		h.log.Warn().
			Str("repo", payload.Repository.FullName).
			Str("url", repoURL).
			Msg("github: no source configured for this repository, ignoring")
		w.WriteHeader(http.StatusOK) // 200: we received it, just not tracking this repo
		return
	}

	reason := fmt.Sprintf("github-push:%s:%s", payload.HeadCommit.ID[:8], payload.Ref)
	if err := h.trigger.TriggerSync(sourceName, reason); err != nil {
		h.log.Error().Err(err).Str("source", sourceName).Msg("github: failed to trigger sync")
		http.Error(w, "failed to trigger sync", http.StatusInternalServerError)
		return
	}

	h.log.Info().
		Str("source", sourceName).
		Str("repo", payload.Repository.FullName).
		Str("commit", payload.HeadCommit.ID[:8]).
		Msg("github: triggered sync from push event")

	w.WriteHeader(http.StatusAccepted)
}

// validateGitHubSignature verifies the HMAC-SHA256 signature from GitHub.
func validateGitHubSignature(signature string, body []byte, secret string) error {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return fmt.Errorf("invalid signature format")
	}
	sigBytes, err := hex.DecodeString(signature[len(prefix):])
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(sigBytes, expected) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// ── GitLab Webhook ────────────────────────────────────────────────────────────

// gitlabPushEvent is the subset of GitLab's push event payload we care about.
type gitlabPushEvent struct {
	ObjectKind string `json:"object_kind"` // "push"
	Ref        string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	Project    struct {
		Name          string `json:"name"`
		HTTPURLToRepo string `json:"http_url_to_repo"`
		SSHURLToRepo  string `json:"ssh_url_to_repo"`
	} `json:"project"`
	Commits []struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	} `json:"commits"`
}

func (h *Handler) handleGitLab(w http.ResponseWriter, r *http.Request) {
	// Validate token
	if h.gitlabToken != "" {
		if r.Header.Get("X-Gitlab-Token") != h.gitlabToken {
			h.log.Warn().Msg("gitlab: invalid token")
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
	}

	// Validate event type
	event := r.Header.Get("X-Gitlab-Event")
	if event != "Push Hook" {
		h.log.Debug().Str("event", event).Msg("gitlab: ignoring non-push event")
		w.WriteHeader(http.StatusOK)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var payload gitlabPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if !isDefaultBranchRef(payload.Ref) {
		w.WriteHeader(http.StatusOK)
		return
	}

	repoURL := payload.Project.HTTPURLToRepo
	sourceName, ok := h.resolver.ResolveByRepoURL(repoURL)
	if !ok {
		sourceName, ok = h.resolver.ResolveByRepoURL(payload.Project.SSHURLToRepo)
	}
	if !ok {
		h.log.Warn().Str("project", payload.Project.Name).Msg("gitlab: no source configured")
		w.WriteHeader(http.StatusOK)
		return
	}

	commitID := payload.CheckoutSHA
	if len(commitID) > 8 {
		commitID = commitID[:8]
	}
	reason := fmt.Sprintf("gitlab-push:%s:%s", commitID, payload.Ref)
	if err := h.trigger.TriggerSync(sourceName, reason); err != nil {
		h.log.Error().Err(err).Str("source", sourceName).Msg("gitlab: failed to trigger sync")
		http.Error(w, "failed to trigger sync", http.StatusInternalServerError)
		return
	}

	h.log.Info().
		Str("source", sourceName).
		Str("project", payload.Project.Name).
		Str("commit", commitID).
		Msg("gitlab: triggered sync from push event")

	w.WriteHeader(http.StatusAccepted)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// isDefaultBranchRef returns true if the ref is a default/main branch ref.
func isDefaultBranchRef(ref string) bool {
	for _, branch := range []string{"refs/heads/main", "refs/heads/master", "refs/heads/trunk"} {
		if ref == branch {
			return true
		}
	}
	return false
}
