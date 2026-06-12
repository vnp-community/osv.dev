// Package all imports all checker source files to trigger registration.
// Tests and main.go should import this package.
package all

import (
	// import each checker group to register them via init()
	_ "github.com/osv/scan-service/internal/parsers/checkers"
)
