// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"sort"

	"github.com/aclements/go-gg/generic/slice"
)

// SortBy sorts each group of g by the named columns. If a column's
// type implements sort.Interface, rows will be sorted according to
// that order. Otherwise, the values in the column must be naturally
// ordered (their types must be orderable by the Go specification). If
// neither is true, SortBy panics with a *generic.TypeError. If more
// than one column is given, SortBy sorts by the tuple of the columns;
// that is, if two values in the first column are equal, they are
// sorted by the second column, and so on.
func SortBy(g Grouping, cols ...string) Grouping {
	// Sort each group.
	sorters := make([]sort.Interface, len(cols))
	return MapTables(g, func(_ GroupID, t *Table) *Table {
		// Create sorters for each column.
		sorters = sorters[:0]
		for _, col := range cols {
			if _, ok := t.Const(col); ok {
				continue
			}
			seq := t.MustColumn(col)
			sorter := slice.Sorter(seq)
			if sort.IsSorted(sorter) {
				continue
			}
			sorters = append(sorters, sorter)
		}

		if len(sorters) == 0 {
			// Avoid shuffling everything by the identity
			// permutation.
			return t
		}

		// Generate an initial permutation sequence.
		perm := make([]int, t.Len())
		for i := range perm {
			perm[i] = i
		}

		// Sort the permutation sequence.
		sort.Stable(&permSort{perm, sorters})

		// Permute all columns.
		var nt Builder
		for _, name := range t.Columns() {
			if cv, ok := t.Const(name); ok {
				nt.AddConst(name, cv)
				continue
			}
			seq := t.Column(name)
			seq = slice.Select(seq, perm)
			nt.Add(name, seq)
		}
		return nt.Done()
	})
}

type permSort struct {
	perm []int
	keys []sort.Interface
}

func (s *permSort) Len() int {
	return len(s.perm)
}

func (s *permSort) Less(i, j int) bool {
	// Since there's no way to ask about equality, we have to do
	// extra work for all of the keys except the last.
	for _, key := range s.keys[:len(s.keys)-1] {
		if key.Less(s.perm[i], s.perm[j]) {
			return true
		} else if key.Less(s.perm[j], s.perm[i]) {
			return false
		}
	}
	return s.keys[len(s.keys)-1].Less(s.perm[i], s.perm[j])
}

func (s *permSort) Swap(i, j int) {
	s.perm[i], s.perm[j] = s.perm[j], s.perm[i]
}
