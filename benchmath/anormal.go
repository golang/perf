// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmath

import "github.com/aclements/go-moremath/stats"

// AssumeNormal is an assumption that a sample is normally distributed.
// The summary statistic is the sample mean and comparisons are done
// using the two-sample t-test.
var AssumeNormal = assumeNormal{}

type assumeNormal struct{}

var _ Assumption = assumeNormal{}

func (assumeNormal) SummaryLabel() string {
	return "mean"
}

func (assumeNormal) Summary(s *Sample, confidence float64) Summary {
	// TODO: Perform a normality test.

	sample := s.sample()
	mean, lo, hi := sample.MeanCI(confidence)

	return Summary{
		Center:     mean,
		Lo:         lo,
		Hi:         hi,
		Confidence: confidence,
	}
}

func (assumeNormal) Compare(s1, s2 *Sample) Comparison {
	t, err := stats.TwoSampleWelchTTest(s1.sample(), s2.sample(), stats.LocationDiffers)
	if err != nil {
		// The t-test failed. Report as if there's no
		// significant difference, along with the error.
		return Comparison{P: 1, N1: len(s1.Values), N2: len(s2.Values), Warnings: []error{err}}
	}
	return Comparison{P: t.P, N1: len(s1.Values), N2: len(s2.Values)}
}
