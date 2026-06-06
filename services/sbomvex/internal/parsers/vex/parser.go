// Package vex provides VEX parser implementations.
package vex

import "github.com/osv/sbomvex/internal/domain/entity"

// VEXParser is the interface implemented by all VEX format parsers.
type VEXParser interface {
	// Parse parses raw VEX bytes and returns a structured VEXDocument.
	Parse(content []byte) (*entity.VEXDocument, error)
	// Format returns the VEXFormat this parser handles.
	Format() entity.VEXFormat
}
