// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"math"
	"reflect"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/vec"
)

// Function samples a continuous univariate function at N points in
// the domain computed by Domain.
//
// The result of Function binds column X to the X values at which the
// function is sampled and retains constant columns from the input.
// The computed function can add arbitrary columns for its output.
type Function struct {
	// X is the name of the column to use for input domain of this
	// function.
	X string

	// N is the number of points to sample the function at. If N
	// is 0, a reasonable default is used.
	N int

	// Domain specifies the domain of which to sample this function.
	// If Domain is nil, it defaults to DomainData{}.
	Domain FunctionDomainer

	// Fn is the continuous univariate function to sample. Fn will
	// be called with each table in the grouping and the X values
	// at which it should be sampled. Fn must add its output
	// columns to out. The output table will already contain the
	// sample points bound to the X column.
	Fn func(gid table.GroupID, in *table.Table, sampleAt []float64, out *table.Builder)
}

const defaultFunctionSamples = 200

func (f Function) F(g table.Grouping) table.Grouping {
	// Set defaults.
	if f.N <= 0 {
		f.N = defaultFunctionSamples
	}
	if f.Domain == nil {
		f.Domain = DomainData{}
	}

	domain := f.Domain.FunctionDomain(g, f.X)
	return table.MapTables(g, func(gid table.GroupID, t *table.Table) *table.Table {
		min, max := domain(gid)

		// Compute sample points. If there's no data, there
		// are no sample points, but we still have to run the
		// function to get the right output columns.
		var ss []float64
		if math.IsNaN(min) {
			ss = []float64{}
		} else {
			ss = vec.Linspace(min, max, f.N)
		}

		var nt table.Builder
		ctype := table.ColType(t, f.X)
		if ctype == float64Type {
			// Bind output X column.
			nt.Add(f.X, ss)
		} else {
			// Convert to the column type.
			vsp := reflect.New(ctype)
			slice.Convert(vsp.Interface(), ss)
			vs := vsp.Elem()
			// This may have produced duplicate values.
			// Eliminate those.
			if vs.Len() > 0 {
				prev, i := vs.Index(0).Interface(), 1
				for j := 1; j < vs.Len(); j++ {
					next := vs.Index(j).Interface()
					if prev == next {
						// Skip duplicate.
						continue
					}

					if i != j {
						vs.Index(i).Set(vs.Index(j))
					}
					i++
					prev = next
				}
				vs.SetLen(i)
			}
			// Bind column-typed values to output X.
			nt.Add(f.X, vs.Interface())
			// And convert back to []float64 so we can
			// apply the function.
			slice.Convert(&ss, vs.Interface())
		}

		// Apply the function to the sample points.
		f.Fn(gid, t, ss, &nt)

		preserveConsts(&nt, t)
		return nt.Done()
	})
}

// preserveConsts copies the constant columns from t into nt.
func preserveConsts(nt *table.Builder, t *table.Table) {
	for _, col := range t.Columns() {
		if nt.Has(col) {
			// Don't overwrite existing columns in nt.
			continue
		}
		if cv, ok := t.Const(col); ok {
			nt.AddConst(col, cv)
		}
	}
}
