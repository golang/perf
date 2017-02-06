// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"reflect"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
)

// Normalize normalizes each group such that some data point is 1.
//
// Either X or Index is required (though 0 is a reasonable value of
// Index).
//
// The result of Normalize is the same as the input table, plus
// additional columns for each normalized column. These columns will
// be named "normalized <col>" where <col> is the name of the original
// column and will have type []float64.
type Normalize struct {
	// X is the name of the column to use to find the denominator
	// row. If X is "", Index is used instead.
	X string

	// Index is the row index of the denominator row if X is ""
	// (otherwise it is ignored). Index may be negative, in which
	// case it is added to the number of rows (e.g., -1 is the
	// last row).
	Index int

	// By is a function func([]T) int that returns the index of
	// the denominator row given column X. By may be nil, in which
	// case it defaults to generic.ArgMin.
	By interface{}

	// Cols is a slice of the names of columns to normalize
	// relative to the corresponding DenomCols value in the
	// denominator row. Cols may be nil, in which case it defaults
	// to all integral and floating point columns.
	Cols []string

	// DenomCols is a slice of the names of columns used as the
	// demoninator. DenomCols may be nil, in which case it
	// defaults to Cols (i.e. each column will be normalized to
	// the value from that column in the denominator row.)
	// Otherwise, DenomCols must be the same length as Cols.
	DenomCols []string
}

func (s Normalize) F(g table.Grouping) table.Grouping {
	// Find the columns to normalize.
	if s.Cols == nil {
		cols := []string{}
		for i, ct := range colTypes(g) {
			if canNormalize(ct.Elem().Kind()) {
				cols = append(cols, g.Columns()[i])
			}
		}
		s.Cols = cols
	}
	if len(s.Cols) == 0 {
		return g
	}

	// Construct new column names.
	newcols := make([]string, len(s.Cols))
	for i, col := range s.Cols {
		newcols[i] = "normalized " + col
	}

	// Get "by" function.
	var byv reflect.Value
	byargs := make([]reflect.Value, 1)
	if s.By != nil {
		byv = reflect.ValueOf(s.By)
		// TODO: Type check byv better.
	}

	return table.MapTables(g, func(_ table.GroupID, t *table.Table) *table.Table {
		if t.Len() == 0 {
			return t
		}

		// Find the denominator row.
		var drow int
		if s.X == "" {
			drow = s.Index
			if drow < 0 {
				drow += t.Len()
			}
		} else {
			xs := t.MustColumn(s.X)
			if s.By == nil {
				drow = slice.ArgMin(xs)
			} else {
				byargs[0] = reflect.ValueOf(xs)
				byout := byv.Call(byargs)
				drow = int(byout[0].Int())
			}
		}

		// Normalize columns.
		newt := table.NewBuilder(t)
		denomCols := s.DenomCols
		if denomCols == nil {
			denomCols = s.Cols
		}
		for coli, col := range s.Cols {
			denom := denomValue(t.MustColumn(denomCols[coli]), drow)
			out := normalizeTo(t.MustColumn(col), denom)
			newt.Add(newcols[coli], out)
		}

		return newt.Done()
	})
}

func colTypes(g table.Grouping) []reflect.Type {
	cts := make([]reflect.Type, len(g.Columns()))
	for i, col := range g.Columns() {
		cts[i] = table.ColType(g, col)
	}
	return cts
}

var canNormalizeKinds = map[reflect.Kind]bool{
	reflect.Float32: true,
	reflect.Float64: true,
	reflect.Int:     true,
	reflect.Int8:    true,
	reflect.Int16:   true,
	reflect.Int32:   true,
	reflect.Int64:   true,
	reflect.Uint:    true,
	reflect.Uintptr: true,
	reflect.Uint8:   true,
	reflect.Uint16:  true,
	reflect.Uint32:  true,
	reflect.Uint64:  true,
}

func canNormalize(k reflect.Kind) bool {
	return canNormalizeKinds[k]
}

func denomValue(s interface{}, index int) float64 {
	switch s := s.(type) {
	case []float64:
		return s[index]
	}
	return reflect.ValueOf(s).Index(index).Convert(float64Type).Float()
}

func normalizeTo(s interface{}, denom float64) interface{} {
	switch s := s.(type) {
	case []float64:
		out := make([]float64, len(s))
		for i, numer := range s {
			out[i] = numer / denom
		}
		return out
	}

	sv := reflect.ValueOf(s)

	out := reflect.MakeSlice(float64SliceType, sv.Len(), sv.Len())
	for i, len := 0, sv.Len(); i < len; i++ {
		numer := sv.Index(i).Convert(float64Type).Float()
		out.Index(i).SetFloat(numer / denom)
	}
	return out.Interface()
}
