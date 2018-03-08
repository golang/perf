// Copyright 2018 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchstat

import (
	"math"
	"sort"
)

// A SortFunc abstracts the sorting interface to compare two rows of a Table
type SortFunc func(*Table, int, int) bool

// ByName sorts tables by the Benchmark name column
func ByName(t *Table, i, j int) bool {
	return t.Rows[i].Benchmark < t.Rows[j].Benchmark
}

// ByDelta sorts tables by the Delta column (comparing the numerical value
// rather than the lexical value)
// The sort takes into account the Change value as well, which indicates
// whether a given delta is "good" or "bad"
func ByDelta(t *Table, i, j int) bool {
	return math.Abs(t.Rows[i].PctDelta)*float64(t.Rows[i].Change) <
		math.Abs(t.Rows[j].PctDelta)*float64(t.Rows[j].Change)
}

// ByChange sorts tables by the unprinted Change column which indicates
// whether a delta is negative, zero, or positive
func ByChange(t *Table, i, j int) bool {
	return t.Rows[i].Change < t.Rows[j].Change
}

// SortReverse returns a SortFunc that is the reverse of the input SortFunc
func SortReverse(sortFunc SortFunc) SortFunc {
	return func(t *Table, i, j int) bool { return !sortFunc(t, i, j) }
}

// SortTable sorts a Table t (in place) by the given SortFunc
func SortTable(t *Table, sortFunc SortFunc) {
	sort.Slice(t.Rows, func(i, j int) bool { return sortFunc(t, i, j) })
}
