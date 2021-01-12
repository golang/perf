// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmath

import (
	"fmt"
	"math"
	"testing"

	"github.com/aclements/go-moremath/stats"
)

func TestMedianSamples(t *testing.T) {
	if false {
		for n := 2; n <= 50; n++ {
			d := stats.BinomialDist{N: n, P: 0.5}
			t.Log(n, 1-(d.PMF(0)+d.PMF(float64(d.N))), d.PMF(0))
		}
	}

	check := func(confidence float64, wantOp string, wantN int) {
		t.Helper()
		gotOp, gotN := medianSamples(confidence)
		if gotOp != wantOp || gotN != wantN {
			t.Errorf("for confidence %v, want %s %d, got %s %d", confidence, wantOp, wantN, gotOp, gotN)
		}
	}

	// At n=6, the tails are 0.015625 * 2 => 0.03125
	check(0.95, ">=", 6)
	// At n=8, the tails are 0.00390625 * 2 => 0.0078125
	check(0.99, ">=", 8)
	// The hard-coded threshold is 50.
	check(1, ">", 50)
	// Check the other extreme. We always need at least two
	// samples to have an interval.
	check(0, ">=", 2)
}

func TestUTestSamples(t *testing.T) {
	check := func(alpha float64, wantOp string, wantN int) {
		t.Helper()
		gotOp, gotN := uTestSamples(alpha)
		if gotOp != wantOp || gotN != wantN {
			t.Errorf("for alpha %v, want %s %d, got %s %d", alpha, wantOp, wantN, gotOp, gotN)
		}
	}
	check(1, ">=", 1)
	check(0.05, ">=", 4)
	check(0.01, ">=", 5)
	check(1e-50, ">", 10)
	check(0, ">", 10)
}

func TestSummaryNone(t *testing.T) {
	// The following tests correspond to the tests in
	// TestMedianSamples.
	a := AssumeNothing
	var sample *Sample
	inf := math.Inf(1)
	sample = NewSample([]float64{-10, 2, 3, 4, 5, 6}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0.95),
		Summary{Center: 3.5, Lo: -10, Hi: 6, Confidence: 1 - 0.03125})
	checkSummary(t, a.Summary(sample, 0.99),
		Summary{Center: 3.5, Lo: -inf, Hi: inf, Confidence: 1},
		"need >= 8 samples for confidence interval at level 0.99")
	checkSummary(t, a.Summary(sample, 1),
		Summary{Center: 3.5, Lo: -inf, Hi: inf, Confidence: 1},
		"need > 50 samples for confidence interval at level 1")
	sample = NewSample([]float64{1, 2}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0),
		Summary{Center: 1.5, Lo: 1, Hi: 2, Confidence: 0.5})

	// And test very small samples.
	sample = NewSample([]float64{1}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0.95),
		Summary{Center: 1, Lo: -inf, Hi: inf, Confidence: 1},
		"need >= 6 samples for confidence interval at level 0.95")
}

func TestCompareNone(t *testing.T) {
	// Most of the complexity is in the sample size warning.
	a := AssumeNothing
	thr := DefaultThresholds
	thr.CompareAlpha = 0.05
	// Too-small samples.
	s1 := NewSample([]float64{-1, -1, -1}, &thr)
	s2 := NewSample([]float64{1, 1, 1}, &thr)
	checkComparison(t, a.Compare(s1, s2),
		Comparison{P: 0.1, N1: 3, N2: 3, Alpha: 0.05},
		"need >= 4 samples to detect a difference at alpha level 0.05")
	// Big enough samples with a difference.
	s1 = NewSample([]float64{-1, -1, -1, -1}, &thr)
	s2 = NewSample([]float64{1, 1, 1, 1}, &thr)
	checkComparison(t, a.Compare(s1, s2),
		Comparison{P: 0.02857142857142857, N1: 4, N2: 4, Alpha: 0.05})
	// Big enough samples, but not enough difference.
	s1 = NewSample([]float64{1, -1, -1, -1}, &thr)
	s2 = NewSample([]float64{-1, 1, 1, 1}, &thr)
	checkComparison(t, a.Compare(s1, s2),
		Comparison{P: 0.4857142857142857, N1: 4, N2: 4, Alpha: 0.05})

	// All samples equal, so the U-test is meaningless.
	s1 = NewSample([]float64{1, 1, 1, 1}, &thr)
	s2 = NewSample([]float64{1, 1, 1, 1}, &thr)
	checkComparison(t, a.Compare(s1, s2),
		Comparison{P: 1, N1: 4, N2: 4, Alpha: 0.05},
		"all samples are equal")

}

func TestSummaryNormal(t *testing.T) {
	// This is a thin wrapper around sample.MeanCI, so just do a
	// smoke test.
	a := AssumeNormal
	sample := NewSample([]float64{-8, 2, 3, 4, 5, 6}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0.95),
		Summary{Center: 2, Lo: -3.351092806089359, Hi: 7.351092806089359, Confidence: 0.95})
}

func TestSummaryExact(t *testing.T) {
	a := AssumeExact
	sample := NewSample([]float64{1, 1, 1, 1}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0.95),
		Summary{Center: 1, Lo: 1, Hi: 1, Confidence: 1})

	sample = NewSample([]float64{1}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0.95),
		Summary{Center: 1, Lo: 1, Hi: 1, Confidence: 1})

	sample = NewSample([]float64{1, 2, 2, 3}, &DefaultThresholds)
	checkSummary(t, a.Summary(sample, 0.95),
		Summary{Center: 2, Lo: 1, Hi: 3, Confidence: 1},
		"exact distribution expected, but values range from 1 to 3")
}

func aeq(x, y float64) bool {
	if x < 0 && y < 0 {
		x, y = -x, -y
	}
	// Check that x and y are equal to 8 digits.
	const factor = 1 - 1e-7
	return x*factor <= y && y*factor <= x
}

func checkSummary(t *testing.T, got, want Summary, warnings ...string) {
	t.Helper()
	for _, w := range warnings {
		want.Warnings = append(want.Warnings, fmt.Errorf("%s", w))
	}
	if !aeq(got.Center, want.Center) || !aeq(got.Lo, want.Lo) || !aeq(got.Hi, got.Hi) || got.Confidence != want.Confidence || !errorsEq(got.Warnings, want.Warnings) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func checkComparison(t *testing.T, got, want Comparison, warnings ...string) {
	t.Helper()
	for _, w := range warnings {
		want.Warnings = append(want.Warnings, fmt.Errorf("%s", w))
	}
	if !aeq(got.P, want.P) || got.N1 != want.N1 || got.N2 != want.N2 || got.Alpha != want.Alpha || !errorsEq(got.Warnings, want.Warnings) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func errorsEq(a, b []error) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Error() != b[i].Error() {
			return false
		}
	}
	return true
}
