// Package checkers — global auto-registration registry.
// Each checker file calls Register() from its init() function.
package checkers

import (
	"fmt"

	"github.com/osv/scanner/internal/domain/entity"
)

var globalDefs []CheckerDef

// Register adds a checker definition to the global registry.
// Called by each checker file's init() function.
func Register(def CheckerDef) {
	globalDefs = append(globalDefs, def)
}

// BuildAll compiles all registered checker definitions into *entity.Checker instances.
// Must be called once at startup after all init() functions have run.
// Returns an error if any definition is invalid.
func BuildAll() ([]*entity.Checker, error) {
	checkers := make([]*entity.Checker, 0, len(globalDefs))
	for _, def := range globalDefs {
		c, err := def.Build()
		if err != nil {
			return nil, fmt.Errorf("BuildAll: %w", err)
		}
		checkers = append(checkers, c)
	}
	return checkers, nil
}

// Count returns the number of registered checker definitions.
func Count() int { return len(globalDefs) }

// ResetForTesting clears all registered definitions. FOR TESTING ONLY.
func ResetForTesting() { globalDefs = nil }
