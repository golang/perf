// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"golang.org/x/perf/benchmath"
	"golang.org/x/perf/benchunit"
)

// Unit metadata is a single piece of unit metadata.
//
// Unit metadata gives information that's useful to interpreting
// values in a given unit. The following metadata keys are predefined:
//
// better={higher,lower} indicates whether higher or lower values of
// this unit are better (indicate an improvement).
//
// assume={nothing,exact} indicates what statistical assumption to
// make when considering distributions of values.
// `nothing` means to make no statistical assumptions (e.g., use
// non-parametric methods) and `exact` means to assume measurements are
// exact (repeated measurement does not increase confidence).
// The default is `nothing`.
type UnitMetadata struct {
	UnitMetadataKey

	// OrigUnit is the original, untidied unit as written in the input.
	OrigUnit string

	Value string

	fileName string
	line     int
}

func (u *UnitMetadata) Pos() (fileName string, line int) {
	return u.fileName, u.line
}

// UnitMetadataKey identifies a single piece of unit metadata by unit
// and metadata name.
type UnitMetadataKey struct {
	Unit string
	Key  string // Metadata key (e.g., "better" or "assume")
}

// UnitMetadataMap stores the accumulated metadata for several units.
// This is indexed by tidied units, while the values store the original
// units from the benchmark file.
type UnitMetadataMap map[UnitMetadataKey]*UnitMetadata

// Get returns the unit metadata for the specified unit and metadata key.
// It tidies unit if necessary.
func (m UnitMetadataMap) Get(unit, key string) *UnitMetadata {
	_, tidyUnit := benchunit.Tidy(1, unit)
	return m[UnitMetadataKey{tidyUnit, key}]
}

// GetAssumption returns the appropriate statistical Assumption to make
// about distributions of values in the given unit.
func (m UnitMetadataMap) GetAssumption(unit string) benchmath.Assumption {
	dist := m.Get(unit, "assume")
	if dist != nil && dist.Value == "exact" {
		return benchmath.AssumeExact
	}
	// The default is to assume nothing.
	return benchmath.AssumeNothing
}

// GetBetter returns whether higher or lower values of the given unit
// indicate an improvement. It returns +1 if higher values are better,
// -1 if lower values are better, or 0 if unknown.
func (m UnitMetadataMap) GetBetter(unit string) int {
	better := m.Get(unit, "better")
	if better != nil {
		switch better.Value {
		case "higher":
			return 1
		case "lower":
			return -1
		}
		return 0
	}
	// Fall back to some built-in defaults.
	switch unit {
	case "ns/op", "sec/op":
		// This measures "duration", so lower is better.
		return -1
	case "MB/s", "B/s":
		// This measures "speed", so higher is better.
		return 1
	case "B/op", "allocs/op":
		// These measure amount of allocation, so lower is better.
		return -1
	}
	return 0
}
