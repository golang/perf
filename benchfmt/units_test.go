// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"testing"

	"golang.org/x/perf/benchmath"
)

func TestReaderUnits(t *testing.T) {
	_, reader := parseAll(t, `
Unit ns/op a=1 b=2
Unit MB/s c=3 c=3
`)
	units := reader.Units()
	if len(units) != 3 {
		t.Fatalf("expected 3 unit metadata, got %d:\n%v", len(units), units)
	}
	check := func(unit, tidyUnit, key, value string) {
		t.Helper()
		// The map is indexed by tidy units.
		mkey := UnitMetadataKey{tidyUnit, key}
		m, ok := units[mkey]
		if !ok {
			t.Errorf("for %s:%s, want %s, got none", tidyUnit, key, value)
			return
		}
		want := UnitMetadata{UnitMetadataKey: mkey, OrigUnit: unit, Value: value}
		if m.UnitMetadataKey != want.UnitMetadataKey ||
			m.OrigUnit != want.OrigUnit ||
			m.Value != want.Value {
			t.Errorf("for %s:%s, want %+v, got %+v", unit, key, want, m)
		}
		// Check that Get works for both tidied and untidied units.
		if got := units.Get(unit, key); got != m {
			t.Errorf("for Get(%v, %v), want %+v, got %+v", unit, key, m, got)
		}
		if got := units.Get(tidyUnit, key); got != m {
			t.Errorf("for Get(%v, %v), want %+v, got %+v", tidyUnit, key, m, got)
		}
	}
	check("ns/op", "sec/op", "a", "1")
	check("ns/op", "sec/op", "b", "2")
	check("MB/s", "B/s", "c", "3")
}

func TestUnitsGetAssumption(t *testing.T) {
	_, reader := parseAll(t, `
Unit a assume=nothing
Unit ns/frob assume=exact
Unit c assume=blah`)
	units := reader.Units()

	check := func(unit string, want benchmath.Assumption) {
		got := units.GetAssumption(unit)
		if got != want {
			t.Errorf("for unit %s, want assumption %s, got %s", unit, want, got)
		}
	}
	check("a", benchmath.AssumeNothing)
	check("ns/frob", benchmath.AssumeExact)
	check("sec/frob", benchmath.AssumeExact)
	check("c", benchmath.AssumeNothing)
}

func TestUnitsGetBetter(t *testing.T) {
	_, reader := parseAll(t, `
Unit a better=higher
Unit b better=lower`)
	units := reader.Units()

	check := func(unit string, want int) {
		got := units.GetBetter(unit)
		if got != want {
			t.Errorf("for unit %s, want better=%+d, got %+d", unit, want, got)
		}
	}
	check("a", 1)
	check("b", -1)
	check("c", 0)

	check("ns/op", -1)
	check("sec/op", -1)

	check("MB/s", 1)
	check("B/s", 1)

	check("B/op", -1)
	check("allocs/op", -1)

	// Check that we can override the built-ins
	_, reader = parseAll(t, "Unit ns/op better=higher")
	units = reader.Units()
	check("ns/op", 1)
	check("sec/op", 1)
}
