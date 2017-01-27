// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchstat

import "fmt"

// A Scaler is a function that scales and formats a measurement.
// All measurements within a given table row are formatted
// using the same scaler, so that the units are consistent
// across the row.
type Scaler func(float64) string

// NewScaler returns a Scaler appropriate for formatting
// the measurement val, which has the given unit.
func NewScaler(val float64, unit string) Scaler {
	if unit == "ns/op" {
		return timeScaler(val)
	}

	var format string
	var scale float64
	var suffix string

	prescale := 1.0
	if unit == "MB/s" {
		prescale = 1e6
	}

	switch x := val * prescale; {
	case x >= 99500000000000:
		format, scale, suffix = "%.0f", 1e12, "T"
	case x >= 9950000000000:
		format, scale, suffix = "%.1f", 1e12, "T"
	case x >= 995000000000:
		format, scale, suffix = "%.2f", 1e12, "T"
	case x >= 99500000000:
		format, scale, suffix = "%.0f", 1e9, "G"
	case x >= 9950000000:
		format, scale, suffix = "%.1f", 1e9, "G"
	case x >= 995000000:
		format, scale, suffix = "%.2f", 1e9, "G"
	case x >= 99500000:
		format, scale, suffix = "%.0f", 1e6, "M"
	case x >= 9950000:
		format, scale, suffix = "%.1f", 1e6, "M"
	case x >= 995000:
		format, scale, suffix = "%.2f", 1e6, "M"
	case x >= 99500:
		format, scale, suffix = "%.0f", 1e3, "k"
	case x >= 9950:
		format, scale, suffix = "%.1f", 1e3, "k"
	case x >= 995:
		format, scale, suffix = "%.2f", 1e3, "k"
	case x >= 99.5:
		format, scale, suffix = "%.0f", 1, ""
	case x >= 9.95:
		format, scale, suffix = "%.1f", 1, ""
	default:
		format, scale, suffix = "%.2f", 1, ""
	}

	if unit == "B/op" {
		suffix += "B"
	}
	if unit == "MB/s" {
		suffix += "B/s"
	}
	scale /= prescale

	return func(val float64) string {
		return fmt.Sprintf(format+suffix, val/scale)
	}
}

func timeScaler(ns float64) Scaler {
	var format string
	var scale float64
	switch x := ns / 1e9; {
	case x >= 99.5:
		format, scale = "%.0fs", 1
	case x >= 9.95:
		format, scale = "%.1fs", 1
	case x >= 0.995:
		format, scale = "%.2fs", 1
	case x >= 0.0995:
		format, scale = "%.0fms", 1000
	case x >= 0.00995:
		format, scale = "%.1fms", 1000
	case x >= 0.000995:
		format, scale = "%.2fms", 1000
	case x >= 0.0000995:
		format, scale = "%.0fµs", 1000*1000
	case x >= 0.00000995:
		format, scale = "%.1fµs", 1000*1000
	case x >= 0.000000995:
		format, scale = "%.2fµs", 1000*1000
	case x >= 0.0000000995:
		format, scale = "%.0fns", 1000*1000*1000
	case x >= 0.00000000995:
		format, scale = "%.1fns", 1000*1000*1000
	default:
		format, scale = "%.2fns", 1000*1000*1000
	}
	return func(ns float64) string {
		return fmt.Sprintf(format, ns/1e9*scale)
	}
}
