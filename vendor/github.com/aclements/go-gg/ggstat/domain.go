// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"math"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
)

// A FunctionDomainer computes the domain over which to evaluate a
// statistical function.
type FunctionDomainer interface {
	// FunctionDomain computes the domain of a particular column
	// within a table. It takes a Grouping and a column in that
	// Grouping to compute the domain of and returns a function
	// that computes the domain for a specific group in the
	// Grouping. This makes it possible for FunctionDomain to
	// easily compute either Grouping-wide domains, or per-Table
	// domains.
	//
	// The returned domain may be (NaN, NaN) to indicate that
	// there is no data and the domain is vacuous.
	FunctionDomain(g table.Grouping, col string) func(gid table.GroupID) (min, max float64)
}

// DomainFixed is a FunctionDomainer that returns a fixed domain.
type DomainFixed struct {
	Min, Max float64
}

var _ FunctionDomainer = DomainFixed{}

func (r DomainFixed) FunctionDomain(g table.Grouping, col string) func(gid table.GroupID) (min, max float64) {
	return func(table.GroupID) (min, max float64) {
		return r.Min, r.Max
	}
}

// DomainData is a FunctionDomainer that computes domains based on the
// bounds of the data.
type DomainData struct {
	// Widen expands the domain by Widen times the span of the
	// data.
	//
	// A value of 1.0 means to use exactly the bounds of the data.
	// If Widen is 0, it is treated as 1.1 (that is, widen the
	// domain by 10%, or 5% on the left and 5% on the right).
	Widen float64

	// SplitGroups indicates that each group in the table should
	// have a separate domain based on the data in that group
	// alone. The default, false, indicates that the domain should
	// be based on all of the data in the table combined. This
	// makes it possible to stack functions and easier to compare
	// them across groups.
	SplitGroups bool
}

var _ FunctionDomainer = DomainData{}

const defaultWiden = 1.1

func (r DomainData) FunctionDomain(g table.Grouping, col string) func(gid table.GroupID) (min, max float64) {
	widen := r.Widen
	if widen <= 0 {
		widen = defaultWiden
	}

	var xs []float64
	if !r.SplitGroups {
		// Compute combined bounds.
		gmin, gmax := math.NaN(), math.NaN()
		for _, gid := range g.Tables() {
			t := g.Table(gid)
			slice.Convert(&xs, t.MustColumn(col))
			xmin, xmax := stats.Bounds(xs)
			if xmin < gmin || math.IsNaN(gmin) {
				gmin = xmin
			}
			if xmax > gmax || math.IsNaN(gmax) {
				gmax = xmax
			}
		}

		// Widen bounds.
		span := gmax - gmin
		gmin, gmax = gmin-span*(widen-1)/2, gmax+span*(widen-1)/2

		return func(table.GroupID) (min, max float64) {
			return gmin, gmax
		}
	}

	return func(gid table.GroupID) (min, max float64) {
		// Compute bounds.
		slice.Convert(&xs, g.Table(gid).MustColumn(col))
		min, max = stats.Bounds(xs)

		// Widen bounds.
		span := max - min
		min, max = min-span*(widen-1)/2, max+span*(widen-1)/2
		return
	}
}
