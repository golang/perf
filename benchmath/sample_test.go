// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmath

import (
	"math"
	"testing"
)

func TestSummaryFormat(t *testing.T) {
	check := func(center, lo, hi float64, want string) {
		t.Helper()
		s := Summary{Center: center, Lo: lo, Hi: hi}
		got := s.PctRangeString()
		if got != want {
			t.Errorf("for %v CI [%v, %v], got %s, want %s", center, lo, hi, got, want)
		}
	}
	inf := math.Inf(1)

	check(1, 0.5, 1.1, "50%")
	check(1, 0.9, 1.5, "50%")
	check(1, 1, 1, "0%")

	check(-1, -0.5, -1.1, "50%")
	check(-1, -0.9, -1.5, "50%")
	check(-1, -1, -1, "0%")

	check(1, -inf, 1, "∞")
	check(1, 1, inf, "∞")

	check(1, -1, 1, "?")
	check(1, -1, -1, "?")
	check(-1, -1, 1, "?")
	check(-1, 1, -1, "?")
	check(0, -1, 1, "?")

	check(0, 0, 0, "0%")
}

func TestComparisonFormat(t *testing.T) {
	check := func(p float64, n1, n2 int, want string) {
		t.Helper()
		got := Comparison{P: p, N1: n1, N2: n2}.String()
		if got != want {
			t.Errorf("for %v,%v,%v, got %s, want %s", p, n1, n2, got, want)
		}
	}
	check(0.5, 1, 2, "p=0.500 n=1+2")
	check(0.5, 2, 2, "p=0.500 n=2")
	check(0, 1, 2, "n=1+2")
	check(0, 2, 2, "n=2")

	checkD := func(p, old, new, alpha float64, want string) {
		got := Comparison{P: p, Alpha: alpha}.FormatDelta(old, new)
		if got != want {
			t.Errorf("for p=%v %v=>%v @%v, got %s, want %s", p, old, new, alpha, got, want)
		}
	}
	checkD(0.5, 0, 0, 0.05, "~")
	checkD(0.01, 0, 0, 0.05, "0.00%")
	checkD(0.01, 1, 1, 0.05, "0.00%")
	checkD(0.01, 0, 1, 0.05, "?")
	checkD(0.01, 1, 1.5, 0.05, "+50.00%")
	checkD(0.01, 1, 0.5, 0.05, "-50.00%")
}
