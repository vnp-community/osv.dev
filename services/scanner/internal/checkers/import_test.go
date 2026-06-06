// Package checkers_test — this file ensures all checker init() functions run.
// Since checkers_test is an external test package, we need explicit blank imports
// for each checker source file's package (same package = same init chain).
// Note: Go automatically runs init() for ALL .go files in the same package,
// so the test binary includes crypto.go, network.go, web.go, etc. automatically.
// No explicit import needed here.
package checkers_test
