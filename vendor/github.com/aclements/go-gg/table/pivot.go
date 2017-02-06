// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// Pivot converts rows of g into columns. label and value must name
// columns in g, and the label column must have type []string. Pivot
// returns a Grouping with a new column named after each distinct
// value in the label column, where the values in that column
// correspond to the values from the value column. All other columns
// (besides label and value) are copied to the output. If, for a given
// column in an output row, no input row has that column in the label
// column, the output cell will have the zero value for its type.
func Pivot(g Grouping, label, value string) Grouping {
	// Find all unique values of label. These are the new columns.
	labels := []string{}
	lset := map[string]int{}
	for _, gid := range g.Tables() {
		for _, l := range g.Table(gid).MustColumn(label).([]string) {
			if _, ok := lset[l]; !ok {
				lset[l] = len(lset)
				labels = append(labels, l)
			}
		}
	}

	// Get all columns that are not label or value.
	groupCols := []string{}
	for _, col := range g.Columns() {
		if col != label && col != value {
			groupCols = append(groupCols, col)
		}
	}

	return MapTables(g, func(_ GroupID, t *Table) *Table {
		var nt Builder

		// Group by all other columns. Each group in gg
		// becomes an output row.
		gg := GroupBy(t, groupCols...)

		// Copy grouped-by values.
		for _, groupCol := range groupCols {
			cv := reflect.MakeSlice(reflect.TypeOf(t.Column(groupCol)), len(gg.Tables()), len(gg.Tables()))
			for i, gid := range gg.Tables() {
				sub := gg.Table(gid)
				cv.Index(i).Set(reflect.ValueOf(sub.Column(groupCol)).Index(0))
			}
			nt.Add(groupCol, cv.Interface())
		}

		// Initialize new columns.
		newCols := make([]reflect.Value, len(lset))
		vt := reflect.TypeOf(t.MustColumn(value))
		for i := range newCols {
			newCols[i] = reflect.MakeSlice(vt, len(gg.Tables()), len(gg.Tables()))
		}

		// Fill in new columns.
		for i, gid := range gg.Tables() {
			sub := gg.Table(gid)

			vcol := reflect.ValueOf(sub.MustColumn(value))
			for j, l := range sub.MustColumn(label).([]string) {
				val := vcol.Index(j)
				newCols[lset[l]].Index(i).Set(val)
			}
		}

		// Add new columns to output table.
		for i, newCol := range newCols {
			nt.Add(labels[i], newCol.Interface())
		}

		return nt.Done()
	})
}

// Unpivot converts columns of g into rows. The returned Grouping
// consists of the columns of g *not* listed in cols, plus two columns
// named by the label and value arguments. For each input row in g,
// the returned Grouping will have len(cols) output rows. The i'th
// such output row corresponds to column cols[i] in the input row. The
// label column will contain the name of the unpivoted column,
// cols[i], and the value column will contain that column's value from
// the input row. The values of all other columns in the input row
// will be repeated across the output rows. All columns in cols must
// have the same type.
func Unpivot(g Grouping, label, value string, cols ...string) Grouping {
	if len(cols) == 0 {
		panic("Unpivot requires at least 1 column")
	}

	colSet := map[string]bool{}
	for _, col := range cols {
		colSet[col] = true
	}

	return MapTables(g, func(_ GroupID, t *Table) *Table {
		var nt Builder

		// Repeat all other columns len(cols) times.
		ntlen := t.Len() * len(cols)
		for _, name := range t.Columns() {
			if colSet[name] || name == label || name == value {
				continue
			}

			col := reflect.ValueOf(t.Column(name))
			ncol := reflect.MakeSlice(col.Type(), ntlen, ntlen)
			for i, l := 0, col.Len(); i < l; i++ {
				v := col.Index(i)
				for j := range cols {
					ncol.Index(i*len(cols) + j).Set(v)
				}
			}

			nt.Add(name, ncol.Interface())
		}

		// Get input columns.
		var vt reflect.Type
		colvs := make([]reflect.Value, len(cols))
		for i, col := range cols {
			colvs[i] = reflect.ValueOf(t.MustColumn(col))
			if i == 0 {
				vt = colvs[i].Type()
			} else if vt != colvs[i].Type() {
				panic(&generic.TypeError{vt, colvs[i].Type(), "; cannot Unpivot columns with different types"})
			}
		}

		// Create label and value columns.
		lcol := make([]string, 0, ntlen)
		vcol := reflect.MakeSlice(vt, ntlen, ntlen)
		for i := 0; i < t.Len(); i++ {
			lcol = append(lcol, cols...)
			for j, colv := range colvs {
				vcol.Index(i*len(cols) + j).Set(colv.Index(i))
			}
		}
		nt.Add(label, lcol).Add(value, vcol.Interface())

		return nt.Done()
	})
}
