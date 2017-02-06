// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
	"github.com/aclements/go-moremath/vec"
)

// TODO: AggFirst, AggTukey. StdDev?

// Agg constructs an Aggregate transform from a grouping column and a
// set of Aggregators.
//
// TODO: Does this belong in ggstat? The specific aggregator functions
// probably do, but the concept could go in package table.
func Agg(xs ...string) func(aggs ...Aggregator) Aggregate {
	return func(aggs ...Aggregator) Aggregate {
		return Aggregate{xs, aggs}
	}
}

// Aggregate computes aggregate functions of a table grouped by
// distinct values of a column or set of columns.
//
// Aggregate first groups the table by the Xs columns. Each of these
// groups produces a single row in the output table, where the unique
// value of each of the Xs columns appears in the output row, along
// with constant columns from the input, as well as any columns that
// have a unique value within every group (they're "effectively"
// constant). Additional columns in the output row are produced by
// applying the Aggregator functions to the group.
type Aggregate struct {
	// Xs is the list column names to group values by before
	// computing aggregate functions.
	Xs []string

	// Aggregators is the set of Aggregator functions to apply to
	// each group of values.
	Aggregators []Aggregator
}

// An Aggregator is a function that aggregates each group of input
// into one row and adds it to output. It may be based on multiple
// columns from input and may add multiple columns to output.
type Aggregator func(input table.Grouping, output *table.Builder)

func (s Aggregate) F(g table.Grouping) table.Grouping {
	isConst := make([]bool, len(g.Columns()))
	for i := range isConst {
		isConst[i] = true
	}

	subgroups := map[table.GroupID]table.Grouping{}
	for _, gid := range g.Tables() {
		g := table.GroupBy(g.Table(gid), s.Xs...)
		subgroups[gid] = g

		for i, col := range g.Columns() {
			if !isConst[i] {
				continue
			}
			// Can this column be promoted to constant?
			for _, gid2 := range g.Tables() {
				t := g.Table(gid2)
				isConst[i] = isConst[i] && checkConst(t, col)
			}
		}
	}

	return table.MapTables(g, func(_ table.GroupID, t *table.Table) *table.Table {
		g := table.GroupBy(t, s.Xs...)
		var nt table.Builder

		// Construct X columns.
		rows := len(g.Tables())
		for colidx, xcol := range s.Xs {
			xs := reflect.MakeSlice(table.ColType(t, xcol), rows, rows)
			for i, gid := range g.Tables() {
				for j := 0; j < len(s.Xs)-colidx-1; j++ {
					gid = gid.Parent()
				}
				xs.Index(i).Set(reflect.ValueOf(gid.Label()))
			}

			nt.Add(xcol, xs.Interface())
		}

		// Apply Aggregators.
		for _, agg := range s.Aggregators {
			agg(g, &nt)
		}

		// Keep constant and effectively constant columns.
		for i := range isConst {
			col := t.Columns()[i]
			if !isConst[i] || nt.Has(col) {
				continue
			}
			if cv, ok := t.Const(col); ok {
				nt.AddConst(col, cv)
				continue
			}

			ncol := reflect.MakeSlice(table.ColType(t, col), len(g.Tables()), len(g.Tables()))
			for i, gid := range g.Tables() {
				v := reflect.ValueOf(g.Table(gid).Column(col))
				ncol.Index(i).Set(v.Index(0))
			}
			nt.Add(col, ncol.Interface())
		}
		return nt.Done()
	})
}

func checkConst(t *table.Table, col string) bool {
	if _, ok := t.Const(col); ok {
		return true
	}
	v := reflect.ValueOf(t.Column(col))
	if v.Len() <= 1 {
		return true
	}
	if !v.Type().Elem().Comparable() {
		return false
	}
	elem := v.Index(0).Interface()
	for i, l := 1, v.Len(); i < l; i++ {
		if elem != v.Index(i).Interface() {
			return false
		}
	}
	return true
}

// AggCount returns an aggregate function that computes the number of
// rows in each group. The resulting column will be named label, or
// "count" if label is "".
func AggCount(label string) Aggregator {
	if label == "" {
		label = "count"
	}

	return func(input table.Grouping, b *table.Builder) {
		counts := make([]int, 0, len(input.Tables()))
		for _, gid := range input.Tables() {
			counts = append(counts, input.Table(gid).Len())
		}
		b.Add(label, counts)
	}
}

// AggMean returns an aggregate function that computes the mean of
// each of cols. The resulting columns will be named "mean <col>" and
// will have the same type as <col>.
func AggMean(cols ...string) Aggregator {
	return aggFn(stats.Mean, "mean ", cols...)
}

// AggGeoMean returns an aggregate function that computes the
// geometric mean of each of cols. The resulting columns will be named
// "geomean <col>" and will have the same type as <col>.
func AggGeoMean(cols ...string) Aggregator {
	return aggFn(stats.GeoMean, "geomean ", cols...)
}

// AggMin returns an aggregate function that computes the minimum of
// each of cols. The resulting columns will be named "min <col>" and
// will have the same type as <col>.
func AggMin(cols ...string) Aggregator {
	min := func(xs []float64) float64 {
		x, _ := stats.Bounds(xs)
		return x
	}
	return aggFn(min, "min ", cols...)
}

// AggMax returns an aggregate function that computes the maximum of
// each of cols. The resulting columns will be named "max <col>" and
// will have the same type as <col>.
func AggMax(cols ...string) Aggregator {
	max := func(xs []float64) float64 {
		_, x := stats.Bounds(xs)
		return x
	}
	return aggFn(max, "max ", cols...)
}

// AggSum returns an aggregate function that computes the sum of each
// of cols. The resulting columns will be named "sum <col>" and will
// have the same type as <col>.
func AggSum(cols ...string) Aggregator {
	return aggFn(vec.Sum, "sum ", cols...)
}

// AggQuantile returns an aggregate function that computes a quantile
// of each of cols. quantile has a range of [0,1]. The resulting
// columns will be named "<prefix> <col>" and will have the same type
// as <col>.
func AggQuantile(prefix string, quantile float64, cols ...string) Aggregator {
	// "prefix" could be autogenerated (e.g. fmt.Sprintf("p%g ",
	// quantile * 100)), but then the caller would need to do the
	// same fmt.Sprintf to compute the column name they had just
	// created. Perhaps Aggregator should provide a way to find
	// the generated column names.
	return aggFn(func(data []float64) float64 {
		return stats.Sample{Xs: data}.Quantile(quantile)
	}, prefix+" ", cols...)
}

func aggFn(f func([]float64) float64, prefix string, cols ...string) Aggregator {
	ocols := make([]string, len(cols))
	for i, col := range cols {
		ocols[i] = prefix + col
	}

	return func(input table.Grouping, b *table.Builder) {
		for coli, col := range cols {
			means := make([]float64, 0, len(input.Tables()))

			var xs []float64
			var ct reflect.Type
			for i, gid := range input.Tables() {
				v := input.Table(gid).MustColumn(col)
				if i == 0 {
					ct = reflect.TypeOf(v)
				}
				slice.Convert(&xs, v)
				means = append(means, f(xs))
			}

			if ct == float64SliceType {
				b.Add(ocols[coli], means)
			} else {
				// Convert means back to the type of col.
				outptr := reflect.New(ct)
				slice.Convert(outptr.Interface(), means)
				b.Add(ocols[coli], outptr.Elem().Interface())
			}
		}
	}
}

// AggUnique returns an aggregate function retains the unique value of
// each of cols within each aggregate group, or panics if some group
// contains more than one value for one of these columns.
//
// Note that Aggregate will automatically retain columns that happen
// to be unique. AggUnique can be used to enforce at aggregation time
// that certain columns *must* be unique (and get a nice error if they
// are not).
func AggUnique(cols ...string) Aggregator {
	return func(input table.Grouping, b *table.Builder) {
		if len(cols) == 0 {
			return
		}
		if len(input.Tables()) == 0 {
			panic(fmt.Sprintf("unknown column: %q", cols[0]))
		}

		for _, col := range cols {
			ctype := table.ColType(input, col)
			rows := len(input.Tables())
			vs := reflect.MakeSlice(ctype, rows, rows)
			for i, gid := range input.Tables() {
				// Get values in this column.
				xs := reflect.ValueOf(input.Table(gid).MustColumn(col))

				// Check for uniqueness.
				if xs.Len() == 0 {
					panic(fmt.Sprintf("cannot AggUnique empty column %q", col))
				}
				uniquev := xs.Index(0)
				unique := uniquev.Interface()
				for i, len := 1, xs.Len(); i < len; i++ {
					other := xs.Index(i).Interface()
					if unique != other {
						panic(fmt.Sprintf("column %q is not unique; contains at least %v and %v", col, unique, other))
					}
				}

				// Store unique value.
				vs.Index(i).Set(uniquev)
			}

			// Add unique values slice to output table.
			b.Add(col, vs.Interface())
		}
	}
}
