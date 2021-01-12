// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmath

import "fmt"

// AssumeExact is an assumption that a value can be measured exactly
// and thus has no distribution and does not require repeated sampling.
// It reports a warning if not all values in a sample are equal.
var AssumeExact = assumeExact{}

type assumeExact struct{}

var _ Assumption = assumeExact{}

func (assumeExact) SummaryLabel() string {
	// Really the summary is the mode, but the point of this
	// assumption is that the summary is the exact value.
	return "exact"
}

func (assumeExact) Summary(s *Sample, confidence float64) Summary {
	// Find the sample's mode. This checks if all samples are the
	// same, and lets us return a reasonable summary even if they
	// aren't all the same.
	val, count := s.Values[0], 1
	modeVal, modeCount := val, count
	for _, v := range s.Values[1:] {
		if v == val {
			count++
			if count > modeCount {
				modeVal, modeCount = val, count
			}
		} else {
			val, count = v, 1
		}
	}
	summary := Summary{Center: modeVal, Lo: s.Values[0], Hi: s.Values[len(s.Values)-1], Confidence: 1}

	if modeCount != len(s.Values) {
		// They're not all the same. Report a warning.
		summary.Warnings = []error{fmt.Errorf("exact distribution expected, but values range from %v to %v", s.Values[0], s.Values[len(s.Values)-1])}
	}
	return summary
}

func (assumeExact) Compare(s1, s2 *Sample) Comparison {
	return Comparison{P: 0, N1: len(s1.Values), N2: len(s2.Values)}
}
