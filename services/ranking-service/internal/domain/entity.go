// Package domain — Ranking service domain model.
package domain

import "fmt"

// RankingEntry maps a CPE string fragment to group priority values.
// MongoDB document stored in "ranking" collection.
//
// Example document:
//
//	{
//	  "cpe": "sap:netweaver",
//	  "rank": [
//	    {"group": "it", "rank": 3},
//	    {"group": "accounting", "rank": 5}
//	  ]
//	}
type RankingEntry struct {
	ID   string      `bson:"_id,omitempty" json:"id,omitempty"`
	CPE  string      `bson:"cpe"           json:"cpe"`
	Rank []GroupRank `bson:"rank"          json:"rank"`
}

// GroupRank associates a group name with a priority integer.
// Higher rank = more important for that group.
type GroupRank struct {
	Group string `bson:"group" json:"group"` // e.g. "it", "accounting", "security"
	Rank  int    `bson:"rank"  json:"rank"`  // 0 = lowest priority
}

// LookupResult is returned by loosy CPE ranking lookup.
type LookupResult struct {
	CPE         string      `json:"cpe"`
	Ranks       []GroupRank `json:"ranks"`
	MatchedPart string      `json:"matched_part"` // which CPE fragment matched
}

// Validate checks RankingEntry for required fields.
func (r *RankingEntry) Validate() error {
	if r.CPE == "" {
		return fmt.Errorf("cpe is required")
	}
	if len(r.Rank) == 0 {
		return fmt.Errorf("at least one rank entry is required")
	}
	for i, gr := range r.Rank {
		if gr.Group == "" {
			return fmt.Errorf("rank[%d].group is required", i)
		}
		if gr.Rank < 0 {
			return fmt.Errorf("rank[%d].rank must be >= 0", i)
		}
	}
	return nil
}
