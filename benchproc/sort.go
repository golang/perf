// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Less reports whether k comes before o in the sort order implied by
// their projection. It panics if k and o have different Projections.
func (k Key) Less(o Key) bool {
	if k.k.proj != o.k.proj {
		panic("cannot compare Keys from different Projections")
	}
	return less(k.k.proj.FlattenedFields(), k.k.vals, o.k.vals)
}

func less(flat []*Field, a, b []string) bool {
	// Walk the tuples in flattened order.
	for _, node := range flat {
		var aa, bb string
		if node.idx < len(a) {
			aa = a[node.idx]
		}
		if node.idx < len(b) {
			bb = b[node.idx]
		}
		if aa != bb {
			cmp := node.cmp(aa, bb)
			if cmp != 0 {
				return cmp < 0
			}
			// The values are equal/unordered according to
			// the comparison function, but the strings
			// differ. Because Keys are only == if
			// their string representations are ==, this
			// means we have to fall back to a secondary
			// comparison that is only == if the strings
			// are ==.
			return aa < bb
		}
	}

	// Tuples are equal.
	return false
}

// SortKeys sorts a slice of Keys using Key.Less.
// All Keys must have the same Projection.
//
// This is equivalent to using Key.Less with the sort package but
// more efficient.
func SortKeys(keys []Key) {
	// Check all the Projections so we don't have to do this on every
	// comparison.
	if len(keys) == 0 {
		return
	}
	s := commonProjection(keys)
	flat := s.FlattenedFields()

	sort.Slice(keys, func(i, j int) bool {
		return less(flat, keys[i].k.vals, keys[j].k.vals)
	})
}

// builtinOrders is the built-in comparison functions.
var builtinOrders = map[string]func(a, b string) int{
	"alpha": func(a, b string) int {
		return strings.Compare(a, b)
	},
	"num": func(a, b string) int {
		aa, erra := parseNum(a)
		bb, errb := parseNum(b)
		if erra == nil && errb == nil {
			// Sort numerically, and put NaNs after other
			// values.
			if aa < bb || (!math.IsNaN(aa) && math.IsNaN(bb)) {
				return -1
			}
			if aa > bb || (math.IsNaN(aa) && !math.IsNaN(bb)) {
				return 1
			}
			// The values are unordered.
			return 0
		}
		if erra != nil && errb != nil {
			// The values are unordered.
			return 0
		}
		// Put floats before non-floats.
		if erra == nil {
			return -1
		}
		return 1
	},
}

const numPrefixes = `KMGTPEZY`

var numRe = regexp.MustCompile(`([0-9.]+)([k` + numPrefixes + `]i?)?[bB]?`)

// parseNum is a fuzzy number parser. It supports common patterns,
// such as SI prefixes.
func parseNum(x string) (float64, error) {
	// Try parsing as a regular float.
	v, err := strconv.ParseFloat(x, 64)
	if err == nil {
		return v, nil
	}

	// Try a suffixed number.
	subs := numRe.FindStringSubmatch(x)
	if subs != nil {
		v, err := strconv.ParseFloat(subs[1], 64)
		if err == nil {
			exp := 0
			if len(subs[2]) > 0 {
				pre := subs[2][0]
				if pre == 'k' {
					pre = 'K'
				}
				exp = 1 + strings.IndexByte(numPrefixes, pre)
			}
			iec := strings.HasSuffix(subs[2], "i")
			if iec {
				return v * math.Pow(1024, float64(exp)), nil
			}
			return v * math.Pow(1000, float64(exp)), nil
		}
	}

	return 0, strconv.ErrSyntax
}
