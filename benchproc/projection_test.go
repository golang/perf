// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchproc/internal/parse"
)

// mustParse parses a single projection.
func mustParse(t *testing.T, proj string) (*Projection, *Filter) {
	f, err := NewFilter("*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, err := (&ProjectionParser{}).Parse(proj, f)
	if err != nil {
		t.Fatalf("unexpected error parsing %q: %v", proj, err)
	}
	return s, f
}

// r constructs a benchfmt.Result with the given full name and file
// config, which is specified as alternating key/value pairs. The
// result has 1 iteration and no values.
func r(t *testing.T, fullName string, fileConfig ...string) *benchfmt.Result {
	res := &benchfmt.Result{
		Name:  benchfmt.Name(fullName),
		Iters: 1,
	}

	if len(fileConfig)%2 != 0 {
		t.Fatal("fileConfig must be alternating key/value pairs")
	}
	for i := 0; i < len(fileConfig); i += 2 {
		cfg := benchfmt.Config{Key: fileConfig[i], Value: []byte(fileConfig[i+1]), File: true}
		res.Config = append(res.Config, cfg)
	}

	return res
}

// p constructs a benchfmt.Result like r, then projects it using p.
func p(t *testing.T, p *Projection, fullName string, fileConfig ...string) Key {
	res := r(t, fullName, fileConfig...)
	return p.Project(res)
}

func TestProjectionBasic(t *testing.T) {
	check := func(key Key, want string) {
		t.Helper()
		got := key.String()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}

	var s *Projection

	// Sub-name config.
	s, _ = mustParse(t, ".fullname")
	check(p(t, s, "Name/a=1"), ".fullname:Name/a=1")
	s, _ = mustParse(t, "/a")
	check(p(t, s, "Name/a=1"), "/a:1")
	s, _ = mustParse(t, ".name")
	check(p(t, s, "Name/a=1"), ".name:Name")

	// Fixed file config.
	s, _ = mustParse(t, "a")
	check(p(t, s, "", "a", "1", "b", "2"), "a:1")
	check(p(t, s, "", "b", "2"), "") // Missing values are omitted
	check(p(t, s, "", "a", "", "b", "2"), "")

	// Variable file config.
	s, _ = mustParse(t, ".config")
	check(p(t, s, "", "a", "1", "b", "2"), "a:1 b:2")
	check(p(t, s, "", "c", "3"), "c:3")
	check(p(t, s, "", "c", "3", "a", "2"), "a:2 c:3")
}

func TestProjectionIntern(t *testing.T) {
	s, _ := mustParse(t, "a,b")

	c12 := p(t, s, "", "a", "1", "b", "2")

	if c12 != p(t, s, "", "a", "1", "b", "2") {
		t.Errorf("Keys should be equal")
	}

	if c12 == p(t, s, "", "a", "1", "b", "3") {
		t.Errorf("Keys should not be equal")
	}

	if c12 != p(t, s, "", "a", "1", "b", "2", "c", "3") {
		t.Errorf("Keys should be equal")
	}
}

func fieldNames(fields []*Field) string {
	names := new(strings.Builder)
	for i, f := range fields {
		if i > 0 {
			names.WriteByte(' ')
		}
		names.WriteString(f.Name)
	}
	return names.String()
}

func TestProjectionParsing(t *testing.T) {
	// Basic parsing is tested by the syntax package. Here we test
	// additional processing done by this package.

	check := func(proj string, want, wantFlat string) {
		t.Helper()
		s, _ := mustParse(t, proj)
		got := fieldNames(s.Fields())
		if got != want {
			t.Errorf("%s: got fields %v, want %v", proj, got, want)
		}
		gotFlat := fieldNames(s.FlattenedFields())
		if gotFlat != wantFlat {
			t.Errorf("%s: got flat fields %v, want %v", proj, gotFlat, wantFlat)
		}
	}
	checkErr := func(proj, error string, pos int) {
		t.Helper()
		f, _ := NewFilter("*")
		_, err := (&ProjectionParser{}).Parse(proj, f)
		if se, _ := err.(*parse.SyntaxError); se == nil || se.Msg != error || se.Off != pos {
			t.Errorf("%s: want error %s at %d; got %s", proj, error, pos, err)
		}
	}

	check("a,b,c", "a b c", "a b c")
	check("a,.config,c", "a .config c", "a c") // .config hasn't been populated with anything yet
	check("a,.fullname,c", "a .fullname c", "a .fullname c")
	check("a,.name,c", "a .name c", "a .name c")
	check("a,/b,c", "a /b c", "a /b c")

	checkErr("a@foo", "unknown order \"foo\"", 2)

	checkErr(".config@(1 2)", "fixed order not allowed for .config", 8)
}

func TestProjectionFiltering(t *testing.T) {
	_, f := mustParse(t, "a@(a b c)")
	check := func(val string, want bool) {
		t.Helper()
		res := r(t, "", "a", val)
		got, _ := f.Apply(res)
		if want != got {
			t.Errorf("%s: want %v, got %v", val, want, got)
		}
	}
	check("a", true)
	check("b", true)
	check("aa", false)
	check("z", false)
}

func TestProjectionExclusion(t *testing.T) {
	// The underlying name normalization has already been tested
	// thoroughly in benchfmt/extract_test.go, so here we just
	// have to test that it's being plumbed right.

	check := func(key Key, want string) {
		t.Helper()
		got := key.String()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}

	// Create the main projection.
	var pp ProjectionParser
	f, _ := NewFilter("*")
	s, err := pp.Parse(".fullname,.config", f)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// Parse specific keys that should be excluded from fullname
	// and config.
	_, err = pp.Parse(".name,/a,exc", f)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	check(p(t, s, "Name"), ".fullname:*")
	check(p(t, s, "Name/a=1"), ".fullname:*")
	check(p(t, s, "Name/a=1/b=2"), ".fullname:*/b=2")

	check(p(t, s, "Name", "exc", "1"), ".fullname:*")
	check(p(t, s, "Name", "exc", "1", "abc", "2"), ".fullname:* abc:2")
	check(p(t, s, "Name", "abc", "2"), ".fullname:* abc:2")
}

func TestProjectionResidue(t *testing.T) {
	check := func(mainProj string, want string) {
		t.Helper()

		// Get the residue of mainProj.
		var pp ProjectionParser
		f, _ := NewFilter("*")
		_, err := pp.Parse(mainProj, f)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		s := pp.Residue()

		// Project a test result.
		key := p(t, s, "Name/a=1/b=2", "x", "3", "y", "4")
		got := key.String()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}

	// Full residue.
	check("", "x:3 y:4 .fullname:Name/a=1/b=2")
	// Empty residue.
	check(".config,.fullname", "")
	// Partial residues.
	check("x,/a", "y:4 .fullname:Name/b=2")
	check(".config", ".fullname:Name/a=1/b=2")
	check(".fullname", "x:3 y:4")
	check(".name", "x:3 y:4 .fullname:*/a=1/b=2")
}

// ExampleProjectionParser_Residue demonstrates residue projections.
//
// This example groups a set of results by .fullname and goos, but the
// two results for goos:linux have two different goarch values,
// indicating the user probably unintentionally grouped uncomparable
// results together. The example uses ProjectionParser.Residue and
// NonSingularFields to warn the user about this.
func ExampleProjectionParser_Residue() {
	var pp ProjectionParser
	p, _ := pp.Parse(".fullname,goos", nil)
	residue := pp.Residue()

	// Aggregate each result by p and track the residue of each group.
	type group struct {
		values   []float64
		residues []Key
	}
	groups := make(map[Key]*group)
	var keys []Key

	for _, result := range results(`
goos: linux
goarch: amd64
BenchmarkAlloc 1 128 ns/op

goos: linux
goarch: arm64
BenchmarkAlloc 1 137 ns/op

goos: darwin
goarch: amd64
BenchmarkAlloc 1 130 ns/op`) {
		// Map result to a group.
		key := p.Project(result)
		g, ok := groups[key]
		if !ok {
			g = new(group)
			groups[key] = g
			keys = append(keys, key)
		}

		// Add value to the group.
		speed, _ := result.Value("sec/op")
		g.values = append(g.values, speed)

		// Add residue to the group.
		g.residues = append(g.residues, residue.Project(result))
	}

	// Report aggregated results.
	SortKeys(keys)
	for _, k := range keys {
		g := groups[k]
		// Report the result.
		fmt.Println(k, mean(g.values), "sec/op")
		// Check if the grouped results vary in some unexpected way.
		nonsingular := NonSingularFields(g.residues)
		if len(nonsingular) > 0 {
			// Report a potential issue.
			fmt.Printf("warning: results vary in %s and may be uncomparable\n", nonsingular)
		}
	}

	// Output:
	// .fullname:Alloc goos:linux 1.325e-07 sec/op
	// warning: results vary in [goarch] and may be uncomparable
	// .fullname:Alloc goos:darwin 1.3e-07 sec/op
}

func results(data string) []*benchfmt.Result {
	var out []*benchfmt.Result
	r := benchfmt.NewReader(strings.NewReader(data), "<string>")
	for r.Scan() {
		switch rec := r.Result(); rec := rec.(type) {
		case *benchfmt.Result:
			out = append(out, rec.Clone())
		case *benchfmt.SyntaxError:
			panic("unexpected error in test data: " + rec.Error())
		}
	}
	return out
}

func mean(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func TestProjectionValues(t *testing.T) {
	s, unit, err := (&ProjectionParser{}).ParseWithUnit("x", nil)
	if err != nil {
		t.Fatalf("unexpected error parsing %q: %v", "x", err)
	}

	check := func(key Key, want, wantUnit string) {
		t.Helper()
		got := key.String()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
		gotUnit := key.Get(unit)
		if gotUnit != wantUnit {
			t.Errorf("got unit %s, want %s", gotUnit, wantUnit)
		}
	}

	res := r(t, "Name", "x", "1")
	res.Values = []benchfmt.Value{{Value: 100, Unit: "ns/op"}, {Value: 1.21, Unit: "gigawatts"}}
	keys := s.ProjectValues(res)
	if len(keys) != len(res.Values) {
		t.Fatalf("got %d Keys, want %d", len(keys), len(res.Values))
	}

	check(keys[0], "x:1 .unit:ns/op", "ns/op")
	check(keys[1], "x:1 .unit:gigawatts", "gigawatts")
}
