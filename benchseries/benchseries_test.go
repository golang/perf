package benchseries

import (
	"fmt"
	"reflect"
	"testing"

	"golang.org/x/perf/benchfmt"
)

func makeConfig(key, value string) benchfmt.Config {
	return benchfmt.Config{
		Key:   key,
		Value: []byte(value),
		File:  true, // N.B. Residue ignores non-File config!
	}
}

func TestBasic(t *testing.T) {
	results := []*benchfmt.Result{
		{
			Config: []benchfmt.Config{
				makeConfig("compare", "numerator"),
				makeConfig("runstamp", "2020-01-01T00:00:00Z"),
				makeConfig("numerator-hash", "abcdef0123456789"),
				makeConfig("denominator-hash", "9876543219fedcba"),
				makeConfig("numerator-hash-time", "2020-02-02T00:00:00Z"),
				makeConfig("residue", "foo"),
			},
			Name:  []byte("Foo"),
			Iters: 10,
			Values: []benchfmt.Value{
				{Value: 1, Unit: "sec"},
			},
		},
		{
			Config: []benchfmt.Config{
				makeConfig("compare", "denominator"),
				makeConfig("runstamp", "2020-01-01T00:00:00Z"),
				makeConfig("numerator-hash", "abcdef0123456789"),
				makeConfig("denominator-hash", "9876543219fedcba"),
				makeConfig("numerator-hash-time", "2020-02-02T00:00:00Z"),
				makeConfig("residue", "foo"),
			},
			Name:  []byte("Foo"),
			Iters: 10,
			Values: []benchfmt.Value{
				{Value: 10, Unit: "sec"},
			},
		},
	}

	opts := &BuilderOptions{
		Filter:          ".unit:/.*/",
		Series:          "numerator-hash-time",
		Table:           "", // .unit only
		Experiment:      "runstamp",
		Compare:         "compare",
		Numerator:       "numerator",
		Denominator:     "denominator",
		NumeratorHash:   "numerator-hash",
		DenominatorHash: "denominator-hash",
		Warn: func(format string, args ...interface{}) {
			s := fmt.Sprintf(format, args...)
			t.Errorf("benchseries warning: %s", s)
		},
	}
	builder, err := NewBuilder(opts)
	if err != nil {
		t.Fatalf("NewBuilder get err %v, want err nil", err)
	}

	for _, r := range results {
		builder.Add(r)
	}

	comparisons, err := builder.AllComparisonSeries(nil, DUPE_REPLACE)
	if err != nil {
		t.Fatalf("AllComparisonSeries got err %v want err nil", err)
	}

	if len(comparisons) != 1 {
		t.Fatalf("len(comparisons) got %d want 1; comparisons: %+v", len(comparisons), comparisons)
	}

	got := comparisons[0]

	if got.Unit != "sec" {
		t.Errorf(`Unit got %q, want "sec"`, got.Unit)
	}

	wantBenchmarks := []string{"Foo"}
	if !reflect.DeepEqual(got.Benchmarks, wantBenchmarks) {
		t.Errorf("Benchmarks got %v want %v", got.Benchmarks, wantBenchmarks)
	}

	wantSeries := []string{"2020-02-02T00:00:00+00:00"}
	if !reflect.DeepEqual(got.Series, wantSeries) {
		t.Errorf("Series got %v want %v", got.Series, wantSeries)
	}

	wantHashPairs := map[string]ComparisonHashes{
		"2020-02-02T00:00:00+00:00": ComparisonHashes{
			NumHash: "abcdef0123456789",
			DenHash: "9876543219fedcba",
		},
	}
	if !reflect.DeepEqual(got.HashPairs, wantHashPairs) {
		t.Errorf("HashPairs got %+v want %+v", got.HashPairs, wantHashPairs)
	}

	wantResidues := []StringAndSlice{{S: "residue", Slice: []string{"foo"}}}
	if !reflect.DeepEqual(got.Residues, wantResidues) {
		t.Errorf("Residues got %v want %v", got.Residues, wantResidues)
	}
}

// If the compare key is missing, then there is nothing to compare.
func TestMissingCompare(t *testing.T) {
	results := []*benchfmt.Result{
		{
			Config: []benchfmt.Config{
				makeConfig("runstamp", "2020-01-01T00:00:00Z"),
				makeConfig("numerator-hash", "abcdef0123456789"),
				makeConfig("denominator-hash", "9876543219fedcba"),
				makeConfig("numerator-hash-time", "2020-02-02T00:00:00Z"),
				makeConfig("residue", "foo"),
			},
			Name:  []byte("Foo"),
			Iters: 10,
			Values: []benchfmt.Value{
				{Value: 1, Unit: "sec"},
			},
		},
		{
			Config: []benchfmt.Config{
				makeConfig("runstamp", "2020-01-01T00:00:00Z"),
				makeConfig("numerator-hash", "abcdef0123456789"),
				makeConfig("denominator-hash", "9876543219fedcba"),
				makeConfig("numerator-hash-time", "2020-02-02T00:00:00Z"),
				makeConfig("residue", "foo"),
			},
			Name:  []byte("Foo"),
			Iters: 10,
			Values: []benchfmt.Value{
				{Value: 10, Unit: "sec"},
			},
		},
	}

	opts := &BuilderOptions{
		Filter:          ".unit:/.*/",
		Series:          "numerator-hash-time",
		Table:           "", // .unit only
		Experiment:      "runstamp",
		Compare:         "compare",
		Numerator:       "numerator",
		Denominator:     "denominator",
		NumeratorHash:   "numerator-hash",
		DenominatorHash: "denominator-hash",
		Warn: func(format string, args ...interface{}) {
			s := fmt.Sprintf(format, args...)
			t.Errorf("benchseries warning: %s", s)
		},
	}
	builder, err := NewBuilder(opts)
	if err != nil {
		t.Fatalf("NewBuilder get err %v, want err nil", err)
	}

	for _, r := range results {
		builder.Add(r)
	}

	comparisons, err := builder.AllComparisonSeries(nil, DUPE_REPLACE)
	if err != nil {
		t.Fatalf("AllComparisonSeries got err %v want err nil", err)
	}

	if len(comparisons) != 1 {
		t.Fatalf("len(comparisons) got %d want 1; comparisons: %+v", len(comparisons), comparisons)
	}

	got := comparisons[0]

	if len(got.Series) != 0 {
		t.Errorf("len(Series) got %d want 0; Series: %+v", len(got.Series), got.Series)
	}
}
