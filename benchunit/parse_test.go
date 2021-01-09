// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchunit

import "testing"

func TestClassOf(t *testing.T) {
	test := func(unit string, cls Class) {
		t.Helper()
		got := ClassOf(unit)
		if got != cls {
			t.Errorf("for %s, want %s, got %s", unit, cls, got)
		}
	}
	test("ns/op", Decimal)
	test("sec/op", Decimal)
	test("sec/B", Decimal)
	test("sec/B/B", Decimal)
	test("sec/disk-B", Decimal)

	test("B/op", Binary)
	test("bytes/op", Binary)
	test("B/s", Binary)
	test("B/sec", Binary)
	test("sec/B*B", Binary) // Discouraged
	test("disk-B/sec", Binary)
	test("disk-B/sec", Binary)
}
