// Package apiv2 — Firestore-backed EnrichmentStore implementation.
package apiv2

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// FirestoreEnrichmentStore reads enrichment data from Firestore.
// Collection layout:
//
//	enriched_vulns/{vulnID}              — EnrichmentData document
//	enriched_vulns/{vulnID}/events       — subcollection of TimelineEvent docs
type FirestoreEnrichmentStore struct {
	client *firestore.Client
}

// NewFirestoreEnrichmentStore creates a Firestore-backed EnrichmentStore.
func NewFirestoreEnrichmentStore(client *firestore.Client) *FirestoreEnrichmentStore {
	return &FirestoreEnrichmentStore{client: client}
}

// GetEnrichment fetches enrichment data for a single vulnerability.
func (s *FirestoreEnrichmentStore) GetEnrichment(ctx context.Context, vulnID string) (*EnrichmentData, error) {
	doc, err := s.client.Collection("enriched_vulns").Doc(vulnID).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("firestore get enrichment %s: %w", vulnID, err)
	}

	var data EnrichmentData
	if err := doc.DataTo(&data); err != nil {
		return nil, fmt.Errorf("decode enrichment %s: %w", vulnID, err)
	}
	data.VulnID = vulnID
	return &data, nil
}

// GetRelated fetches related vulnerabilities using the alias_groups collection.
func (s *FirestoreEnrichmentStore) GetRelated(ctx context.Context, vulnID string, limit int) ([]*RelatedVuln, error) {
	// Primary: query alias_groups where aliases array contains vulnID
	iter := s.client.Collection("alias_groups").
		Where("members", "array-contains", vulnID).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	var related []*RelatedVuln
	seen := map[string]bool{vulnID: true}

	doc, err := iter.Next()
	if err == nil && doc.Exists() {
		var group struct {
			Members []string `firestore:"members"`
		}
		if doc.DataTo(&group) == nil {
			for _, m := range group.Members {
				if !seen[m] {
					seen[m] = true
					related = append(related, &RelatedVuln{
						VulnID: m,
						Reason: "alias",
					})
				}
			}
		}
	}

	// Fill up to limit from related_vulns subcollection
	if len(related) < limit {
		relIter := s.client.Collection("enriched_vulns").Doc(vulnID).
			Collection("related").Limit(limit - len(related)).Documents(ctx)
		defer relIter.Stop()
		for {
			rDoc, err := relIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				break
			}
			var rv RelatedVuln
			if rDoc.DataTo(&rv) == nil && !seen[rv.VulnID] {
				seen[rv.VulnID] = true
				related = append(related, &rv)
			}
		}
	}
	return related, nil
}

// GetTimeline fetches the event timeline for a vulnerability.
func (s *FirestoreEnrichmentStore) GetTimeline(ctx context.Context, vulnID string) ([]*TimelineEvent, error) {
	iter := s.client.Collection("enriched_vulns").Doc(vulnID).
		Collection("events").
		OrderBy("timestamp", firestore.Asc).
		Documents(ctx)
	defer iter.Stop()

	var events []*TimelineEvent
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("get timeline %s: %w", vulnID, err)
		}
		var ev TimelineEvent
		if doc.DataTo(&ev) == nil {
			events = append(events, &ev)
		}
	}
	return events, nil
}

// BatchGetEnrichment fetches enrichment data for multiple vulnerabilities in parallel.
func (s *FirestoreEnrichmentStore) BatchGetEnrichment(ctx context.Context, ids []string) (map[string]*EnrichmentData, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build doc refs for batch read
	refs := make([]*firestore.DocumentRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, s.client.Collection("enriched_vulns").Doc(id))
	}

	docs, err := s.client.GetAll(ctx, refs)
	if err != nil {
		return nil, fmt.Errorf("batch get enrichment: %w", err)
	}

	results := make(map[string]*EnrichmentData, len(docs))
	for i, doc := range docs {
		if !doc.Exists() {
			continue
		}
		var data EnrichmentData
		if doc.DataTo(&data) == nil {
			data.VulnID = ids[i]
			results[ids[i]] = &data
		}
	}
	return results, nil
}

// RecordTimelineEvent appends a new event to a vulnerability's timeline.
func (s *FirestoreEnrichmentStore) RecordTimelineEvent(ctx context.Context, vulnID string, ev TimelineEvent) error {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	_, _, err := s.client.Collection("enriched_vulns").Doc(vulnID).
		Collection("events").Add(ctx, ev)
	return err
}

// isNotFound returns true if the error is a Firestore "not found" error.
func isNotFound(err error) bool {
	return err != nil && (containsCode(err, "NotFound") || containsCode(err, "not found"))
}

func containsCode(err error, code string) bool {
	return err != nil && (len(err.Error()) > 0) && (err.Error() != "") &&
		(err.Error() == code || len(err.Error()) > len(code))
}
