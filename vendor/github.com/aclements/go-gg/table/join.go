// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"reflect"

	"github.com/aclements/go-gg/generic/slice"
)

// Join joins g1 and g2 on tables with identical group IDs where col1
// in g1 equals col2 in g2. It maintains the group order of g1, except
// that groups that aren't in g2 are removed, and maintains the row
// order of g1, followed by the row order of g2.
//
// TODO: Support join on more than one column.
func Join(g1 Grouping, col1 string, g2 Grouping, col2 string) Grouping {
	var ng GroupingBuilder
	for _, gid := range g1.Tables() {
		t1, t2 := g1.Table(gid), g2.Table(gid)
		if t2 == nil {
			continue
		}

		// TODO: Optimize for cases where col1 and/or col2 are
		// constant.

		// Index col2 in t2.
		ridx := make(map[interface{}][]int)
		rv := reflect.ValueOf(t2.MustColumn(col2))
		for i, l := 0, rv.Len(); i < l; i++ {
			v := rv.Index(i).Interface()
			ridx[v] = append(ridx[v], i)
		}

		// For each row in t1, find the matching rows in col2
		// and build up the row indexes for t1 and t2.
		idx1, idx2 := []int{}, []int{}
		lv := reflect.ValueOf(t1.MustColumn(col1))
		for i, l := 0, lv.Len(); i < l; i++ {
			r := ridx[lv.Index(i).Interface()]
			for range r {
				idx1 = append(idx1, i)
			}
			idx2 = append(idx2, r...)
		}

		// Build the joined table.
		var nt Builder
		for _, col := range t1.Columns() {
			if cv, ok := t1.Const(col); ok {
				nt.Add(col, cv)
				continue
			}
			nt.Add(col, slice.Select(t1.Column(col), idx1))
		}
		for _, col := range t2.Columns() {
			// Often the join column is the same in both
			// and we can skip it because we added it from
			// the first table.
			if col == col1 && col == col2 {
				continue
			}

			if cv, ok := t2.Const(col); ok {
				nt.Add(col, cv)
				continue
			}
			nt.Add(col, slice.Select(t2.Column(col), idx2))
		}

		ng.Add(gid, nt.Done())
	}
	return ng.Done()
}
