// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// MapTables applies f to each Table in g and returns a new Grouping
// with the same group structure as g, but with the Tables returned by
// f.
func MapTables(g Grouping, f func(gid GroupID, table *Table) *Table) Grouping {
	var out GroupingBuilder
	for _, gid := range g.Tables() {
		out.Add(gid, f(gid, g.Table(gid)))
	}
	return out.Done()
}

// MapCols applies f to a set of input columns to construct a set of
// new output columns.
//
// For each Table in g, MapCols calls f(in[0], in[1], ..., out[0],
// out[1], ...) where in[i] is column incols[i]. f should process the
// values in the input column slices and fill output columns slices
// out[j] accordingly. MapCols returns a new Grouping that adds each
// outcols[j] bound to out[j].
//
// If all of the input columns are constant for a given table, MapCols
// will call f with all slices of length 1. The input column slices
// will contain the constant column values and MapCols will bind each
// output column value out[i][0] as a constant.
func MapCols(g Grouping, f interface{}, incols ...string) func(outcols ...string) Grouping {
	return func(outcols ...string) Grouping {
		fv := reflect.ValueOf(f)
		if fv.Kind() != reflect.Func {
			panic(&generic.TypeError{fv.Type(), nil, "must be a function"})
		}
		ft := fv.Type()
		if ft.NumIn() != len(incols)+len(outcols) {
			panic(&generic.TypeError{ft, nil, fmt.Sprintf("has the wrong number of arguments; expected %d", len(incols)+len(outcols))})
		}
		if ft.NumOut() != 0 {
			panic(&generic.TypeError{ft, nil, "has the wrong number of results; expected 0"})
		}

		// Create output column slices.
		totalRows := 0
		for _, gid := range g.Tables() {
			t := g.Table(gid)
		colloop:
			for _, incol := range incols {
				if _, ok := t.Const(incol); !ok {
					totalRows += g.Table(gid).Len()
					break colloop
				}
			}
		}
		ocols := make([]reflect.Value, len(outcols))
		for i := range ocols {
			ocols[i] = reflect.MakeSlice(ft.In(i+len(incols)), totalRows, totalRows)
		}

		// Apply f to each group.
		var out GroupingBuilder
		args := make([]reflect.Value, len(incols)+len(outcols))
		opos := 0
		for _, gid := range g.Tables() {
			t := g.Table(gid)

			// Are all inputs are constants?
			allConst := true
			for _, incol := range incols {
				if _, ok := t.Const(incol); !ok {
					allConst = false
					break
				}
			}
			if allConst {
				for i, incol := range incols {
					cv, _ := t.Const(incol)
					args[i] = reflect.MakeSlice(ColType(t, incol), 1, 1)
					args[i].Index(0).Set(reflect.ValueOf(cv))
				}
				for i, ocol := range ocols {
					args[i+len(incols)] = reflect.MakeSlice(ocol.Type(), 1, 1)
				}

				fv.Call(args)

				tb := NewBuilder(t)
				for i, outcol := range outcols {
					tb.AddConst(outcol, args[i+len(incols)].Index(0).Interface())
				}
				out.Add(gid, tb.Done())
				continue
			}

			// Prepare arguments.
			for i, incol := range incols {
				args[i] = reflect.ValueOf(t.MustColumn(incol))
			}
			for i, ocol := range ocols {
				args[i+len(incols)] = ocol.Slice(opos, opos+t.Len())
			}
			opos += t.Len()

			// Call f.
			fv.Call(args)

			// Add output columns.
			tb := NewBuilder(t)
			for i, outcol := range outcols {
				tb.Add(outcol, args[i+len(incols)].Interface())
			}
			out.Add(gid, tb.Done())
		}
		return out.Done()
	}
}

// Rename returns g with column 'from' renamed to 'to'. The column
// retains its position.
func Rename(g Grouping, from, to string) Grouping {
	return MapTables(g, func(_ GroupID, t *Table) *Table {
		t.MustColumn(from)
		var nt Builder
		for _, col := range t.Columns() {
			if col == to {
				continue
			}

			ncol := col
			if col == from {
				ncol = to
			}

			if cv, ok := t.Const(col); ok {
				nt.AddConst(ncol, cv)
			} else {
				nt.Add(ncol, t.Column(col))
			}
		}
		return nt.Done()
	})
}

// Remove returns g with column 'col' removed.
func Remove(g Grouping, col string) Grouping {
	return MapTables(g, func(_ GroupID, t *Table) *Table {
		t.MustColumn(col)
		var nt Builder
		for _, col2 := range t.Columns() {
			if col == col2 {
				continue
			}
			if cv, ok := t.Const(col2); ok {
				nt.AddConst(col2, cv)
			} else {
				nt.Add(col2, t.Column(col2))
			}
		}
		return nt.Done()
	})
}
