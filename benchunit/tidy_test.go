// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchunit

import "testing"

func TestTidy(t *testing.T) {
	test := func(unit, tidied string, factor float64) {
		t.Helper()
		gotFactor, got := Tidy(1, unit)
		if got != tidied || gotFactor != factor {
			t.Errorf("for %s, want *%f %s, got *%f %s", unit, factor, tidied, gotFactor, got)
		}
	}

	test("ns/op", "sec/op", 1e-9)
	test("x-ns/op", "x-sec/op", 1e-9)
	test("MB/s", "B/s", 1e6)
	test("x-MB/s", "x-B/s", 1e6)
	test("B/op", "B/op", 1)
	test("x-B/op", "x-B/op", 1)
	test("x-allocs/op", "x-allocs/op", 1)

	test("op/ns", "op/ns", 1)
	test("MB*MB/s", "B*B/s", 1e6*1e6)
	test("MB/MB", "B/MB", 1e6)
}
