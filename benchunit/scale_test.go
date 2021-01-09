// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchunit

import (
	"math"
	"testing"
)

func TestScale(t *testing.T) {
	var cls Class
	test := func(num float64, want, wantPred string) {
		t.Helper()

		got := Scale(num, cls)
		if got != want {
			t.Errorf("for %v, got %s, want %s", num, got, want)
		}

		// Check what happens when this number is exactly on
		// the crux between two scale factors.
		pred := math.Nextafter(num, 0)
		got = Scale(pred, cls)
		if got != wantPred {
			dir := "-ε"
			if num < 0 {
				dir = "+ε"
			}
			t.Errorf("for %v%s, got %s, want %s", num, dir, got, wantPred)
		}
	}

	cls = Decimal
	// Smoke tests
	test(0, "0.000", "0.000")
	test(1, "1.000", "1.000")
	test(-1, "-1.000", "-1.000")
	// Full range
	test(9999500000000000, "9999.5T", "9999.5T")
	test(999950000000000, "1000.0T", "999.9T")
	test(99995000000000, "100.0T", "99.99T")
	test(9999500000000, "10.00T", "9.999T")
	test(999950000000, "1.000T", "999.9G")
	test(99995000000, "100.0G", "99.99G")
	test(9999500000, "10.00G", "9.999G")
	test(999950000, "1.000G", "999.9M")
	test(99995000, "100.0M", "99.99M")
	test(9999500, "10.00M", "9.999M")
	test(999950, "1.000M", "999.9k")
	test(99995, "100.0k", "99.99k")
	test(9999.5, "10.00k", "9.999k")
	test(999.95, "1.000k", "999.9")
	test(99.995, "100.0", "99.99")
	test(9.9995, "10.00", "9.999")
	test(.99995, "1.000", "999.9m")
	test(.099995, "100.0m", "99.99m")
	test(.0099995, "10.00m", "9.999m")
	test(.00099995, "1.000m", "999.9µ")
	test(.000099995, "100.0µ", "99.99µ")
	test(.0000099995, "10.00µ", "9.999µ")
	test(.00000099995, "1.000µ", "999.9n")
	test(.000000099995, "100.0n", "99.99n")
	test(.0000000099995, "10.00n", "9.999n")
	test(.00000000099995, "1.000n", "0.9999n") // First pred we won't up-scale

	// Below the smallest scale unit rounding gets imperfect, but
	// it's off from the ideal by at most one ulp, so we accept it.
	test(math.Nextafter(.000000000099995, 1), "0.1000n", "0.09999n")
	test(.0000000000099995, "0.01000n", "0.009999n")
	test(math.Nextafter(.00000000000099995, 1), "0.001000n", "0.0009999n")
	test(.000000000000099995, "0.0001000n", "0.00009999n")
	test(.0000000000000099995, "0.00001000n", "0.000009999n")
	test(math.Nextafter(.00000000000000099995, 1), "0.000001000n", "0.0000009999n")

	// Misc
	test(-99995000000000, "-100.0T", "-99.99T")
	test(-.0000000099995, "-10.00n", "-9.999n")

	cls = Binary
	// Smoke tests
	test(0, "0.000", "0.000")
	test(1, "1.000", "1.000")
	test(-1, "-1.000", "-1.000")
	// Full range
	test(.99995*(1<<50), "1023.9Ti", "1023.9Ti")
	test(99.995*(1<<40), "100.0Ti", "99.99Ti")
	test(9.9995*(1<<40), "10.00Ti", "9.999Ti")
	test(.99995*(1<<40), "1.000Ti", "1023.9Gi")
	test(99.995*(1<<30), "100.0Gi", "99.99Gi")
	test(9.9995*(1<<30), "10.00Gi", "9.999Gi")
	test(.99995*(1<<30), "1.000Gi", "1023.9Mi")
	test(99.995*(1<<20), "100.0Mi", "99.99Mi")
	test(9.9995*(1<<20), "10.00Mi", "9.999Mi")
	test(.99995*(1<<20), "1.000Mi", "1023.9Ki")
	test(99.995*(1<<10), "100.0Ki", "99.99Ki")
	test(9.9995*(1<<10), "10.00Ki", "9.999Ki")
	test(.99995*(1<<10), "1.000Ki", "1023.9")
	test(99.995*(1<<0), "100.0", "99.99")
	test(9.9995*(1<<0), "10.00", "9.999")
	test(.99995, "1.000", "0.9999")
	test(.099995, "0.1000", "0.09999")
	test(.0099995, "0.01000", "0.009999")
	test(.00099995, "0.001000", "0.0009999")
	test(.000099995, "0.0001000", "0.00009999")
	test(.0000099995, "0.00001000", "0.000009999")
	test(.00000099995, "0.000001000", "0.0000009999")
	// We stop at 10 digits after the decimal. Again, rounding
	// gets a little weird.
	test(.00000009995, "0.0000001000", "0.0000000999")
	test(math.Nextafter(.00000000995, 1), "0.0000000100", "0.0000000099")
	test(.00000000095, "0.0000000010", "0.0000000009")
	test(.00000000005, "0.0000000001", "0.0000000000")
	test(.000000000009, "0.0000000000", "0.0000000000")
}

func TestNoOpScaler(t *testing.T) {
	test := func(val float64, want string) {
		t.Helper()
		got := NoOpScaler.Format(val)
		if got != want {
			t.Errorf("for %v, got %s, want %s", val, got, want)
		}
	}

	test(1, "1")
	test(123456789, "123456789")
	test(123.456789, "123.456789")
}
