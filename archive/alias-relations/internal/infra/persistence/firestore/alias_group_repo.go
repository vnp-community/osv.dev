// infra/persistence/firestore/alias_group_repo.go
package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	aliasgroup "github.com/osv/alias-relations/internal/domain/aggregate/alias_group"
	"github.com/osv/alias-relations/internal/domain/repository"
	"github.com/osv/alias-relations/internal/domain/valueobject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	collectionAliasGroups  = "alias-groups"
	collectionGroupMembers = "alias-group-members"
)

// AliasGroupRepo implements repository.AliasGroupRepository using Firestore.
type AliasGroupRepo struct {
	client *firestore.Client
}

// NewAliasGroupRepo creates a Firestore-backed AliasGroupRepository.
func NewAliasGroupRepo(client *firestore.Client) repository.AliasGroupRepository {
	return &AliasGroupRepo{client: client}
}

// GetByMemberID finds the AliasGroup containing the given vulnID.
func (r *AliasGroupRepo) GetByMemberID(ctx context.Context, vulnID string) (*aliasgroup.AliasGroup, error) {
	// Lookup in the denormalized member index
	memberDoc, err := r.client.Collection(collectionGroupMembers).Doc(vulnID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get member index for %s: %w", vulnID, err)
	}

	data := memberDoc.Data()
	groupID, ok := data["group_id"].(string)
	if !ok || groupID == "" {
		return nil, nil
	}

	return r.GetByGroupID(ctx, groupID)
}

// GetByGroupID retrieves an AliasGroup by its group ID.
func (r *AliasGroupRepo) GetByGroupID(ctx context.Context, groupID string) (*aliasgroup.AliasGroup, error) {
	doc, err := r.client.Collection(collectionAliasGroups).Doc(groupID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get alias group %s: %w", groupID, err)
	}

	return r.fromDoc(groupID, doc.Data())
}

// Save persists an AliasGroup and its member index entries.
func (r *AliasGroupRepo) Save(ctx context.Context, group *aliasgroup.AliasGroup) error {
	bugIDs := group.BugIDs()

	groupData := map[string]interface{}{
		"bug_ids":          bugIDs,
		"canonical_id":     group.CanonicalID(),
		"last_modified":    group.LastModified(),
		"detection_method": string(group.DetectionMethod()),
	}

	batch := r.client.Batch()

	// Save the group document
	groupRef := r.client.Collection(collectionAliasGroups).Doc(group.ID())
	batch.Set(groupRef, groupData)

	// Update the member index for each bug ID
	for _, bugID := range bugIDs {
		memberRef := r.client.Collection(collectionGroupMembers).Doc(bugID)
		batch.Set(memberRef, map[string]interface{}{"group_id": group.ID()})
	}

	_, err := batch.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit alias group save: %w", err)
	}
	return nil
}

// Delete removes an AliasGroup by ID.
func (r *AliasGroupRepo) Delete(ctx context.Context, groupID string) error {
	_, err := r.client.Collection(collectionAliasGroups).Doc(groupID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("delete alias group %s: %w", groupID, err)
	}
	return nil
}

// DeleteMember removes the member index entry for a vuln ID.
func (r *AliasGroupRepo) DeleteMember(ctx context.Context, vulnID string) error {
	_, err := r.client.Collection(collectionGroupMembers).Doc(vulnID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("delete member index for %s: %w", vulnID, err)
	}
	return nil
}

func (r *AliasGroupRepo) fromDoc(groupID string, data map[string]interface{}) (*aliasgroup.AliasGroup, error) {
	bugIDsRaw, _ := data["bug_ids"].([]interface{})
	bugIDs := make([]string, 0, len(bugIDsRaw))
	for _, v := range bugIDsRaw {
		if s, ok := v.(string); ok {
			bugIDs = append(bugIDs, s)
		}
	}

	lastModified, _ := data["last_modified"].(time.Time)
	detectionMethod := valueobject.DetectionMethod(fmt.Sprintf("%v", data["detection_method"]))

	return aliasgroup.ReconstitueAliasGroup(groupID, bugIDs, lastModified, detectionMethod), nil
}
