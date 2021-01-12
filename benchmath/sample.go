// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package benchmath provides tools for computing statistics over
// distributions of benchmark measurements.
//
// This package is opinionated. For example, it doesn't provide
// specific statistical tests. Instead, callers state distributional
// assumptions and this package chooses appropriate tests.
//
// All analysis results contain a list of warnings, captured as an
// []error value. These aren't errors that prevent analysis, but
// should be presented to the user along with analysis results.
package benchmath

import (
	"fmt"
	"math"
	"sort"

	"github.com/aclements/go-moremath/mathx"
	"github.com/aclements/go-moremath/stats"
)

// A Sample is a set of repeated measurements of a given benchmark.
type Sample struct {
	// Values are the measured values, in ascending order.
	Values []float64

	// Thresholds stores the statistical thresholds used by tests
	// on this sample.
	Thresholds *Thresholds

	// Warnings is a list of warnings about this sample that
	// should be reported to the user.
	Warnings []error
}

// NewSample constructs a Sample from a set of measurements.
func NewSample(values []float64, t *Thresholds) *Sample {
	// TODO: Analyze stationarity and put results in Warnings.
	// Consider Augmented Dickey–Fuller (based on Maricq et al.)

	// Sort values for fast order statistics.
	sort.Float64s(values)
	return &Sample{values, t, nil}
}

func (s *Sample) sample() stats.Sample {
	return stats.Sample{Xs: s.Values, Sorted: true}
}

// A Thresholds configures various thresholds used by statistical tests.
//
// This should be initialized to DefaultThresholds because it may be
// extended with other fields in the future.
type Thresholds struct {
	// CompareAlpha is the alpha level below which
	// Assumption.Compare rejects the null hypothesis that two
	// samples come from the same distribution.
	//
	// This is typically 0.05.
	CompareAlpha float64
}

// Note: Thresholds exists so we can extend it in the future with
// things like the stationarity and normality test thresholds without
// having to add function arguments in the future.

// DefaultThresholds contains a reasonable set of defaults for Thresholds.
var DefaultThresholds = Thresholds{
	CompareAlpha: 0.05,
}

// An Assumption indicates a distributional assumption about a sample.
type Assumption interface {
	// SummaryLabel returns the string name for the summary
	// statistic under this assumption. For example, "median" or
	// "mean".
	SummaryLabel() string

	// Summary returns a summary statistic and its confidence
	// interval at the given confidence level for Sample s.
	//
	// Confidence is given in the range [0,1], e.g., 0.95 for 95%
	// confidence.
	Summary(s *Sample, confidence float64) Summary

	// Compare tests whether s1 and s2 come from the same
	// distribution.
	Compare(s1, s2 *Sample) Comparison
}

// A Summary summarizes a Sample.
type Summary struct {
	// Center is some measure of the central tendency of a sample.
	Center float64

	// Lo and Hi give the bounds of the confidence interval around
	// Center.
	Lo, Hi float64

	// Confidence is the actual confidence level of the confidence
	// interval given by Lo, Hi. It will be >= the requested
	// confidence level.
	Confidence float64

	// Warnings is a list of warnings about this summary or its
	// confidence interval.
	Warnings []error
}

// PctRangeString returns a string representation of the range of this
// Summary's confidence interval as a percentage.
func (s Summary) PctRangeString() string {
	if math.IsInf(s.Lo, 0) || math.IsInf(s.Hi, 0) {
		return "∞"
	}

	// If the signs of the bounds differ from the center, we can't
	// render it as a percent.
	var csign = mathx.Sign(s.Center)
	if csign != mathx.Sign(s.Lo) || csign != mathx.Sign(s.Hi) {
		return "?"
	}

	// If center is 0, avoid dividing by zero. But we can only get
	// here if lo and hi are also 0, in which case is seems
	// reasonable to call this 0%.
	if s.Center == 0 {
		return "0%"
	}

	// Phew. Compute the range percent.
	v := math.Max(s.Hi/s.Center-1, 1-s.Lo/s.Center)
	return fmt.Sprintf("%.0f%%", 100*v)
}

// A Comparison is the result of comparing two samples to test if they
// come from the same distribution.
type Comparison struct {
	// P is the p-value of the null hypothesis that two samples
	// come from the same distribution. If P is less than a
	// threshold alpha (typically 0.05), then we reject the null
	// hypothesis.
	//
	// P can be 0, which indicates this is an exact result.
	P float64

	// N1 and N2 are the sizes of the two samples.
	N1, N2 int

	// Alpha is the alpha threshold for this test. If P < Alpha,
	// we reject the null hypothesis that the two samples come
	// from the same distribution.
	Alpha float64

	// Warnings is a list of warnings about this comparison
	// result.
	Warnings []error
}

// String summarizes the comparison. The general form of this string
// is "p=0.PPP n=N1+N2" but can be shortened.
func (c Comparison) String() string {
	var s string
	if c.P != 0 {
		s = fmt.Sprintf("p=%0.3f ", c.P)
	}
	if c.N1 == c.N2 {
		// Slightly shorter form for a common case.
		return s + fmt.Sprintf("n=%d", c.N1)
	}
	return s + fmt.Sprintf("n=%d+%d", c.N1, c.N2)
}

// FormatDelta formats the difference in the centers of two distributions.
// The old and new values must be the center summaries of the two
// compared samples. If the Comparison accepts the null hypothesis
// that the samples come from the same distribution, FormatDelta
// returns "~" to indicate there's no meaningful difference.
// Otherwise, it returns the percent difference between the centers.
func (c Comparison) FormatDelta(old, new float64) string {
	if c.P > c.Alpha {
		return "~"
	}
	if old == new {
		return "0.00%"
	}
	if old == 0 {
		return "?"
	}
	pct := ((new / old) - 1.0) * 100.0
	return fmt.Sprintf("%+.2f%%", pct)
}
