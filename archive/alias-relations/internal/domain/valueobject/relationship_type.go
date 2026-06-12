// domain/valueobject/relationship_type.go
package valueobject

import "errors"

// RelationshipType classifies the kind of relationship between vulnerability IDs.
type RelationshipType string

const (
	RelationshipAlias    RelationshipType = "ALIAS"    // Same vulnerability, different ID
	RelationshipUpstream RelationshipType = "UPSTREAM"  // This is downstream of another
	RelationshipRelated  RelationshipType = "RELATED"   // Related but distinct vulnerability
)

var ErrInvalidRelationshipType = errors.New("invalid relationship type")

// ParseRelationshipType converts a string to a RelationshipType.
func ParseRelationshipType(s string) (RelationshipType, error) {
	switch RelationshipType(s) {
	case RelationshipAlias, RelationshipUpstream, RelationshipRelated:
		return RelationshipType(s), nil
	default:
		return "", ErrInvalidRelationshipType
	}
}

func (r RelationshipType) String() string { return string(r) }

// DetectionMethod describes how an alias was detected.
type DetectionMethod string

const (
	DetectionManual         DetectionMethod = "MANUAL"
	DetectionSourceDeclared DetectionMethod = "SOURCE_DECLARED"
	DetectionAIDetected     DetectionMethod = "AI_DETECTED"
)

// Aliases for convenience
const (
	DetectionSourceDeclaredAlias = DetectionSourceDeclared
)
