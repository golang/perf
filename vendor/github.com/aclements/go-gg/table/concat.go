// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"fmt"

	"github.com/aclements/go-gg/generic/slice"
)

// Concat returns the concatenation of the rows in each matching group
// across gs. All Groupings in gs must have the same set of columns
// (though they need not be in the same order; the column order from
// gs[0] will be used). The GroupIDs in the returned Grouping will be
// the union of the GroupIDs in gs.
func Concat(gs ...Grouping) Grouping {
	if len(gs) == 0 {
		return new(Table)
	}

	// Check that all Groupings have the same set of columns. They
	// can be in different orders.
	colSet := map[string]bool{}
	for _, col := range gs[0].Columns() {
		colSet[col] = true
	}
	for i, g2 := range gs[1:] {
		diff := len(g2.Columns()) != len(colSet)
		if !diff {
			for _, col := range g2.Columns() {
				if !colSet[col] {
					diff = true
					break
				}
			}
		}
		if diff {
			panic(fmt.Sprintf("columns in Groupings 0 and %d differ: %q vs %q", i+1, gs[0].Columns(), g2.Columns()))
		}
	}

	// Collect group IDs.
	haveGID := map[GroupID]bool{}
	gids := []GroupID{}
	for _, g := range gs {
		for _, gid := range g.Tables() {
			if haveGID[gid] {
				continue
			}
			haveGID[gid] = true
			gids = append(gids, gid)
		}
	}

	// Build output groups.
	var ng GroupingBuilder
	for _, gid := range gids {
		// Build output table.
		var nt Builder
		var cols []slice.T
		for _, col := range gs[0].Columns() {
			// Is it constant?
			isConst := false
			var cv interface{}
			for _, g := range gs {
				t := g.Table(gid)
				if t == nil {
					continue
				}
				if cv1, ok := t.Const(col); ok {
					if !isConst {
						isConst = true
						cv = cv1
					} else if cv != cv1 {
						isConst = false
						break
					}
				} else {
					isConst = false
					break
				}
			}
			if isConst {
				nt.AddConst(col, cv)
				continue
			}

			// Not a constant. Collect slices.
			for _, g := range gs {
				t := g.Table(gid)
				if t == nil {
					continue
				}
				cols = append(cols, t.Column(col))
			}
			nt.Add(col, slice.Concat(cols...))
			cols = cols[:0]
		}
		ng.Add(gid, nt.Done())
	}
	return ng.Done()
}
