// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

// Mktables pre-computes statistical tables.
package main

import (
	"fmt"

	"github.com/aclements/go-moremath/stats"
)

func main() {
	var s1, s2 []float64

	// Compute minimal P-value for the U-test given different
	// sample sizes.
	fmt.Printf("var uTestMinP = []float64{\n")
	for n := 1; n < 10; n++ {
		// The P-value is minimized when the order statistic
		// is maximally separated.
		s1 = append(s1, -1)
		s2 = append(s2, 1)
		res, err := stats.MannWhitneyUTest(s1, s2, stats.LocationDiffers)
		if err != nil {
			panic(err)
		}
		fmt.Printf("\t%d: %v,\n", n, res.P)
	}
	fmt.Printf("}\n")
}
