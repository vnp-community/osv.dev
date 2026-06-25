package finding

import (
	"time"

	"github.com/google/uuid"
	findingpb "github.com/osv/shared/proto/gen/go/finding/v1"
)

// helper functions used by state_machine.go

func nowUTC() time.Time { return time.Now().UTC() }

func mustParseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

// FromProto converts a protobuf FindingInput to a domain Finding.
// productID, engagementID, testID are passed separately from the batch request context.
func FromProto(fi *findingpb.FindingInput, productID, engagementID, testID string) *Finding {
	f := &Finding{
		ID:               uuid.New(),
		Title:            fi.Title,
		Description:      fi.Description,
		Mitigation:       fi.Mitigation,
		Impact:           fi.Impact,
		References:       fi.References,
		Severity:         Severity(fi.Severity),
		CVE:              fi.Cve,
		CWE:              int(fi.Cwe),
		VulnIDFromTool:   fi.VulnIdFromTool,
		CVSSv3:           fi.CvssV3,
		Active:           fi.Active,
		Verified:         fi.Verified,
		FalsePositive:    fi.FalsePositive,
		Duplicate:        fi.Duplicate,
		ComponentName:    fi.ComponentName,
		ComponentVersion: fi.ComponentVersion,
		FilePath:         fi.FilePath,
		LineNumber:       lineNumberPtr(int(fi.LineNumber)),
		Service:          fi.Service,
		HashCode:         fi.HashCode,
		Tags:             fi.Tags,
		ProductID:        mustParseUUID(productID),
		EngagementID:     mustParseUUID(engagementID),
		TestID:           mustParseUUID(testID),
		CreatedAt:        nowUTC(),
		UpdatedAt:        nowUTC(),
	}
	if fi.CvssV3Score != nil {
		f.CVSSv3Score = fi.CvssV3Score
	}
	if fi.Date != nil {
		t := fi.Date.AsTime()
		f.Date = t
	} else {
		f.Date = nowUTC()
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return f
}

// lineNumberPtr returns a pointer to n, or nil if n is zero.
func lineNumberPtr(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}
