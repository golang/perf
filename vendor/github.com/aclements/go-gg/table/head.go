// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import "reflect"

// Head returns the first n rows in each Table of g.
func Head(g Grouping, n int) Grouping {
	return headTail(g, n, false)
}

// Tail returns the last n rows in each Table of g.
func Tail(g Grouping, n int) Grouping {
	return headTail(g, n, true)
}

func headTail(g Grouping, n int, tail bool) Grouping {
	return MapTables(g, func(_ GroupID, t *Table) *Table {
		if t.Len() <= n {
			return t
		}

		var nt Builder
		for _, col := range t.Columns() {
			if cv, ok := t.Const(col); ok {
				nt.AddConst(col, cv)
				continue
			}

			cv := reflect.ValueOf(t.Column(col))
			if tail {
				cv = cv.Slice(t.Len()-n, t.Len())
			} else {
				cv = cv.Slice(0, n)
			}
			nt.Add(col, cv.Interface())
		}
		return nt.Done()
	})
}

// HeadTables returns the first n tables in g.
func HeadTables(g Grouping, n int) Grouping {
	return headTailTables(g, n, false)
}

// TailTables returns the first n tables in g.
func TailTables(g Grouping, n int) Grouping {
	return headTailTables(g, n, true)
}

func headTailTables(g Grouping, n int, tail bool) Grouping {
	tables := g.Tables()
	if len(tables) <= n {
		return g
	} else if tail {
		tables = tables[len(tables)-n:]
	} else {
		tables = tables[:n]
	}

	var ng GroupingBuilder
	for _, gid := range tables {
		ng.Add(gid, g.Table(gid))
	}
	return ng.Done()
}
