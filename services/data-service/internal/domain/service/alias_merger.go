// domain/service/alias_merger.go
package service

import (
	"context"
	"fmt"

	aliasgroup "github.com/osv/data-service/internal/domain/aggregate/alias_group"
	"github.com/osv/data-service/internal/domain/repository"
	"github.com/osv/data-service/internal/domain/valueobject"
	"github.com/rs/zerolog"
)

// AliasMerger processes declared aliases from OSV schema.
// Primary method: read aliases[] field and merge into AliasGroup.
type AliasMerger struct {
	groupRepo repository.AliasGroupRepository
	log       zerolog.Logger
}

// NewAliasMerger creates an AliasMerger with the given group repository.
func NewAliasMerger(repo repository.AliasGroupRepository, log zerolog.Logger) *AliasMerger {
	return &AliasMerger{groupRepo: repo, log: log}
}

// ProcessVulnerability merges a vulnerability's declared aliases into an AliasGroup.
// vulnID is the primary ID; declaredAliases are IDs listed in the aliases[] field.
func (m *AliasMerger) ProcessVulnerability(ctx context.Context, vulnID string, declaredAliases []string) error {
	if vulnID == "" {
		return fmt.Errorf("vulnID cannot be empty")
	}

	allIDs := make([]string, 0, 1+len(declaredAliases))
	allIDs = append(allIDs, vulnID)
	allIDs = append(allIDs, declaredAliases...)

	// 1. Find all existing groups containing any of these IDs
	seen := map[string]bool{}                    // groupID → true
	affectedGroups := []*aliasgroup.AliasGroup{} //nolint:prealloc

	for _, id := range allIDs {
		group, err := m.groupRepo.GetByMemberID(ctx, id)
		if err != nil {
			return fmt.Errorf("lookup group for %s: %w", id, err)
		}
		if group == nil {
			continue
		}
		if !seen[group.ID()] {
			seen[group.ID()] = true
			affectedGroups = append(affectedGroups, group)
		}
	}

	m.log.Debug().
		Str("vuln_id", vulnID).
		Int("declared_aliases", len(declaredAliases)).
		Int("affected_groups", len(affectedGroups)).
		Msg("processing alias merge")

	// 2. Merge all affected groups into one
	var merged *aliasgroup.AliasGroup
	if len(affectedGroups) == 0 {
		merged = aliasgroup.NewAliasGroup(allIDs, valueobject.DetectionSourceDeclared)
	} else {
		merged = affectedGroups[0]
		// Delete old groups before merge (except the primary)
		for _, g := range affectedGroups[1:] {
			if err := m.groupRepo.Delete(ctx, g.ID()); err != nil {
				m.log.Warn().Err(err).Str("group_id", g.ID()).Msg("failed to delete old group")
			}
			merged.Merge(g)
		}
		// Add all declared IDs to the merged group
		for _, id := range allIDs {
			merged.AddID(id)
		}
	}

	// 3. Save the merged group
	if err := m.groupRepo.Save(ctx, merged); err != nil {
		return fmt.Errorf("save merged group: %w", err)
	}

	m.log.Info().
		Str("group_id", merged.ID()).
		Str("canonical_id", merged.CanonicalID()).
		Int("total_ids", merged.Size()).
		Msg("alias group merged successfully")

	return nil
}
