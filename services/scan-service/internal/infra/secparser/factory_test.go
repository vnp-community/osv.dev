package secparser

import (
	"testing"
)

func TestParserFactory(t *testing.T) {
	pf := NewFactory()

	// ListScanTypes should not be empty
	types := pf.ListScanTypes()
	if len(types) == 0 {
		t.Fatal("expected parsers to be registered")
	}

	// Should be able to get a known parser
	// For instance "Snyk Scan" or whatever is actually registered
	found := false
	for _, pt := range types {
		if pt != "" {
			p, err := pf.GetParser(pt)
			if err != nil {
				t.Errorf("failed to get parser %q: %v", pt, err)
			}
			if p == nil {
				t.Errorf("parser %q is nil", pt)
			}
			found = true
			break
		}
	}
	
	if !found {
		t.Skip("no parsers registered")
	}

	// Unknown parser should return error
	_, err := pf.GetParser("Unknown Parser xyz")
	if err == nil {
		t.Error("expected error for unknown parser")
	}
}
