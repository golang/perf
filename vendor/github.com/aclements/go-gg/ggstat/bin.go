// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"math"
	"reflect"
	"sort"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/vec"
)

// XXX If this is just based on the number of bins, it can come up
// with really ugly boundary numbers. If the bin width is specified,
// then you could also specify the left edge and bins will be placed
// at [align+width*N, align+width*(N+1)]. ggplot2 also lets you
// specify the center alignment.
//
// XXX In Matlab and NumPy, bins are open on the right *except* for
// the last bin, which is closed on both.
//
// XXX Number of bins/bin width/specify boundaries, same bins across
// all groups/separate for each group/based on shared scales (don't
// have that information here), relative or absolute histogram (Matlab
// has lots more).
//
// XXX Scale transform.
//
// The result of Bin has two columns in addition to constant columns from the input:
//
// - Column X is the left edge of the bin.
//
// - Column W is the sum of the rows' weights, or column "count" is
//   the number of rows in the bin.
type Bin struct {
	// X is the name of the column to use for samples.
	X string

	// W is the optional name of the column to use for sample
	// weights. It may be "" to weight each sample as 1.
	W string

	// Width controls how wide each bin should be. If not provided
	// or 0, a width will be chosen to produce 30 bins. If X is an
	// integer column, this width will be treated as an integer as
	// well.
	Width float64

	// Center controls the center point of each bin. To center on
	// integers, for example, you could use {Width: 1, Center:
	// 0}.
	// XXX What does center mean for integers? Should an unspecified center yield an autochosen one, or 0?
	//Center float64

	// Breaks is the set of break points to use as boundaries
	// between bins. The interval of each bin is [Breaks[i],
	// Breaks[i+1]). Data points before the first break are
	// dropped. If provided, Width and Center are ignored.
	Breaks table.Slice

	// SplitGroups indicates that each group in the table should
	// have separate bounds based on the data in that group alone.
	// The default, false, indicates that the binning function
	// should use the bounds of all of the data combined. This
	// makes it easier to compare bins across groups.
	SplitGroups bool
}

func (b Bin) F(g table.Grouping) table.Grouping {
	breaks := reflect.ValueOf(b.Breaks)
	agg := AggCount("count")
	if b.W != "" {
		agg = aggFn(vec.Sum, "", b.W)
	}
	if !breaks.IsValid() && !b.SplitGroups {
		breaks = b.computeBreaks(g)
	}
	// Change b.X to the start of the bin.
	g = table.MapTables(g, func(_ table.GroupID, t *table.Table) *table.Table {
		breaks := breaks
		if !breaks.IsValid() {
			breaks = b.computeBreaks(t)
		}
		nbreaks := breaks.Len()

		in := reflect.ValueOf(t.MustColumn(b.X))
		nin := in.Len()

		out := reflect.MakeSlice(breaks.Type(), nin, nin)
		var found []int
		for i := 0; i < nin; i++ {
			elt := in.Index(i)
			bin := sort.Search(nbreaks, func(j int) bool {
				return generic.OrderR(elt, breaks.Index(j)) < 0
			})
			// 0 means the row doesn't fit on the front
			// XXX Allow configuring the first and last bin as infinite or not.
			bin = bin - 1
			if bin >= 0 {
				found = append(found, i)
				out.Index(i).Set(breaks.Index(bin))
			}
		}
		var nt table.Builder
		for _, col := range t.Columns() {
			if col == b.X {
				nt.Add(col, slice.Select(out.Interface(), found))
			} else if c, ok := t.Const(col); ok {
				nt.AddConst(col, c)
			} else {
				nt.Add(col, slice.Select(t.Column(col), found))
			}
		}
		return nt.Done()
	})
	// Group by the found bin
	return Agg(b.X)(agg).F(g)
}

func (b Bin) computeBreaks(g table.Grouping) reflect.Value {
	var cols []slice.T
	for _, gid := range g.Tables() {
		cols = append(cols, g.Table(gid).MustColumn(b.X))
	}
	data := slice.Concat(cols...)

	min := slice.Min(data)
	max := slice.Max(data)

	rv := reflect.ValueOf(min)
	switch rv.Type().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		min, max := rv.Int(), reflect.ValueOf(max).Int()
		width := int64(b.Width)
		if width == 0 {
			width = (max - min) / 30
			if width < 1 {
				width = 1
			}
		}
		// XXX: This assumes boundaries should be aligned with
		// 0. We should support explicit Center or Boundary
		// requests.
		min -= (min % width)
		var breaks []int64
		for i := min; i < max; i += width {
			breaks = append(breaks, i)
		}
		outs := reflect.New(reflect.ValueOf(cols[0]).Type())
		slice.Convert(outs.Interface(), breaks)
		return outs.Elem()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		min, max := rv.Uint(), reflect.ValueOf(max).Uint()
		width := uint64(b.Width)
		if width == 0 {
			width = (max - min) / 30
			if width < 1 {
				width = 1
			}
		}
		min -= (min % width)
		var breaks []uint64
		for i := min; i < max; i += width {
			breaks = append(breaks, i)
		}
		outs := reflect.New(reflect.ValueOf(cols[0]).Type())
		slice.Convert(outs.Interface(), breaks)
		return outs.Elem()
	case reflect.Float32, reflect.Float64:
		min, max := rv.Float(), reflect.ValueOf(max).Float()
		width := b.Width
		if width == 0 {
			width = (max - min) / 30
			if width == 0 {
				width = 1
			}
		}
		min -= math.Mod(min, width)
		var breaks []float64
		for i := min; i < max; i += width {
			breaks = append(breaks, i)
		}
		outs := reflect.New(reflect.ValueOf(cols[0]).Type())
		slice.Convert(outs.Interface(), breaks)
		return outs.Elem()
	default:
		panic("can't compute breaks for unknown type")
	}
}

// TODO: Count for categorical data.
