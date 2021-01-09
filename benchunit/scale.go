// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchunit

import (
	"fmt"
	"math"
	"strconv"
)

// A Scaler represents a scaling factor for a number and
// its scientific representation.
type Scaler struct {
	Prec   int     // Digits after the decimal point
	Factor float64 // Unscaled value of 1 Prefix (e.g., 1 k => 1000)
	Prefix string  // Unit prefix ("k", "M", "Ki", etc)
}

// Format formats val and appends the unit prefix according to the given scale.
// For example, if the Scaler has class Decimal, Format(123456789)
// returns "123.4M".
//
// If the value has units, be sure to tidy it first (see Tidy).
// Otherwise, this can result in nonsense units when the result is
// displayed with its units; for example, representing 123456789 ns as
// 123.4M ns ("megananoseconds", aka milliseconds).
func (s Scaler) Format(val float64) string {
	buf := make([]byte, 0, 20)
	buf = strconv.AppendFloat(buf, val/s.Factor, 'f', s.Prec, 64)
	buf = append(buf, s.Prefix...)
	return string(buf)
}

// NoOpScaler is a Scaler that formats numbers with the smallest
// number of digits necessary to capture the exact value, and no
// prefix. This is intended for when the output will be consumed by
// another program, such as when producing CSV format.
var NoOpScaler = Scaler{-1, 1, ""}

type factor struct {
	factor float64
	prefix string
	// Thresholds for 100.0, 10.00, 1.000.
	t100, t10, t1 float64
}

var siFactors = mkSIFactors()
var iecFactors = mkIECFactors()
var sigfigs, sigfigsBase = mkSigfigs()

func mkSIFactors() []factor {
	// To ensure that the thresholds for printing values with
	// various factors exactly match how printing itself will
	// round, we construct the thresholds by parsing the printed
	// representation.
	var factors []factor
	exp := 12
	for _, p := range []string{"T", "G", "M", "k", "", "m", "µ", "n"} {
		t100, _ := strconv.ParseFloat(fmt.Sprintf("99.995e%d", exp), 64)
		t10, _ := strconv.ParseFloat(fmt.Sprintf("9.9995e%d", exp), 64)
		t1, _ := strconv.ParseFloat(fmt.Sprintf(".99995e%d", exp), 64)
		factors = append(factors, factor{math.Pow(10, float64(exp)), p, t100, t10, t1})
		exp -= 3
	}
	return factors
}

func mkIECFactors() []factor {
	var factors []factor
	exp := 40
	// ISO/IEC 80000 doesn't specify fractional prefixes. They're
	// meaningful for things like "B/sec", but "1 /KiB/s" looks a
	// lot more confusing than bottoming out at "B" and printing
	// with more precision, such as "0.001 B/s".
	//
	// Because t1 of one factor represents 1024 of the next
	// smaller factor, we'll render values in the range [1000,
	// 1024) using the smaller factor. E.g., 1020 * 1024 will be
	// rendered as 1020 KiB, not 0.996 MiB. This is intentional as
	// it achieves higher precision, avoids rendering scaled
	// values below 1, and seems generally less confusing for
	// binary factors.
	for _, p := range []string{"Ti", "Gi", "Mi", "Ki", ""} {
		t100, _ := strconv.ParseFloat(fmt.Sprintf("0x1.8ffae147ae148p%d", 6+exp), 64) // 99.995
		t10, _ := strconv.ParseFloat(fmt.Sprintf("0x1.3ffbe76c8b439p%d", 3+exp), 64)  // 9.9995
		t1, _ := strconv.ParseFloat(fmt.Sprintf("0x1.fff972474538fp%d", -1+exp), 64)  // .99995
		factors = append(factors, factor{math.Pow(2, float64(exp)), p, t100, t10, t1})
		exp -= 10
	}
	return factors
}

func mkSigfigs() ([]float64, int) {
	var sigfigs []float64
	// Print up to 10 digits after the decimal place.
	for exp := -1; exp > -9; exp-- {
		thresh, _ := strconv.ParseFloat(fmt.Sprintf("9.9995e%d", exp), 64)
		sigfigs = append(sigfigs, thresh)
	}
	// sigfigs[0] is the threshold for 3 digits after the decimal.
	return sigfigs, 3
}

// Scale formats val using at least three significant digits,
// appending an SI or binary prefix. See Scaler.Format for details.
func Scale(val float64, cls Class) string {
	return CommonScale([]float64{val}, cls).Format(val)
}

// CommonScale returns a common Scaler to apply to all values in vals.
// This scale will show at least three significant digits for every
// value.
func CommonScale(vals []float64, cls Class) Scaler {
	// The common scale is determined by the non-zero value
	// closest to zero.
	var min float64
	for _, v := range vals {
		v = math.Abs(v)
		if v != 0 && (min == 0 || v < min) {
			min = v
		}
	}
	if min == 0 {
		return Scaler{3, 1, ""}
	}

	var factors []factor
	switch cls {
	default:
		panic(fmt.Sprintf("bad Class %v", cls))
	case Decimal:
		factors = siFactors
	case Binary:
		factors = iecFactors
	}

	for _, factor := range factors {
		switch {
		case min >= factor.t100:
			return Scaler{1, factor.factor, factor.prefix}
		case min >= factor.t10:
			return Scaler{2, factor.factor, factor.prefix}
		case min >= factor.t1:
			return Scaler{3, factor.factor, factor.prefix}
		}
	}

	// The value is less than the smallest factor. Print it using
	// the smallest factor and more precision to achieve the
	// desired sigfigs.
	factor := factors[len(factors)-1]
	val := min / factor.factor
	for i, thresh := range sigfigs {
		if val >= thresh || i == len(sigfigs)-1 {
			return Scaler{i + sigfigsBase, factor.factor, factor.prefix}
		}
	}

	panic("not reachable")
}
