// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic/slice"
)

var boolType = reflect.TypeOf(false)

// Filter filters g to only rows where pred returns true. pred must be
// a function that returns bool and takes len(cols) arguments where
// the type of col[i] is assignable to argument i.
//
// TODO: Create a faster batch variant where pred takes slices.
func Filter(g Grouping, pred interface{}, cols ...string) Grouping {
	// TODO: Use generic.TypeError.
	predv := reflect.ValueOf(pred)
	predt := predv.Type()
	if predt.Kind() != reflect.Func || predt.NumIn() != len(cols) || predt.NumOut() != 1 || predt.Out(0) != boolType {
		panic("predicate function must be func(col[0], col[1], ...) bool")
	}
	if len(cols) == 0 {
		return g
	}
	if len(g.Tables()) == 0 {
		panic(fmt.Sprintf("unknown column %q", cols[0]))
	}
	// Type check arguments.
	for i, col := range cols {
		colt := ColType(g, col)
		if !colt.Elem().AssignableTo(predt.In(i)) {
			panic(fmt.Sprintf("column %d (type %s) is not assignable to predicate argument %d (type %s)", i, colt.Elem(), i, predt.In(i)))
		}
	}

	args := make([]reflect.Value, len(cols))
	colvs := make([]reflect.Value, len(cols))
	match := make([]int, 0)
	return MapTables(g, func(_ GroupID, t *Table) *Table {
		// Get columns.
		for i, col := range cols {
			colvs[i] = reflect.ValueOf(t.MustColumn(col))
		}

		// Find the set of row indexes that satisfy pred.
		match = match[:0]
		for r, len := 0, t.Len(); r < len; r++ {
			for c, colv := range colvs {
				args[c] = colv.Index(r)
			}
			if predv.Call(args)[0].Bool() {
				match = append(match, r)
			}
		}

		// Create the new table.
		if len(match) == t.Len() {
			return t
		}
		var nt Builder
		for _, col := range t.Columns() {
			nt.Add(col, slice.Select(t.Column(col), match))
		}
		return nt.Done()
	})
}

// FilterEq filters g to only rows where the value in col equals val.
func FilterEq(g Grouping, col string, val interface{}) Grouping {
	match := make([]int, 0)
	return MapTables(g, func(_ GroupID, t *Table) *Table {
		// Find the set of row indexes that match val.
		seq := t.MustColumn(col)
		match = match[:0]
		rv := reflect.ValueOf(seq)
		for i, len := 0, rv.Len(); i < len; i++ {
			if rv.Index(i).Interface() == val {
				match = append(match, i)
			}
		}

		var nt Builder
		for _, col := range t.Columns() {
			nt.Add(col, slice.Select(t.Column(col), match))
		}
		return nt.Done()
	})
}
