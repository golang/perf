// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
	"github.com/aclements/go-moremath/vec"
)

// TODO: Default to first (and second) column for X (and Y)?

// Density constructs a probability density estimate from a set of
// samples using kernel density estimation.
//
// X is the only required field. All other fields have reasonable
// default zero values.
//
// The result of Density has three columns in addition to constant
// columns from the input:
//
// - Column X is the points at which the density estimate is sampled.
//
// - Column "probability density" is the density estimate.
//
// - Column "cumulative density" is the cumulative density estimate.
type Density struct {
	// X is the name of the column to use for samples.
	X string

	// W is the optional name of the column to use for sample
	// weights. It may be "" to uniformly weight samples.
	W string

	// N is the number of points to sample the KDE at. If N is 0,
	// a reasonable default is used.
	//
	// TODO: This is particularly sensitive to the scale
	// transform.
	//
	// TODO: Base the default on the bandwidth. If the bandwidth
	// is really narrow, we may need a lot of samples to exceed
	// the Nyquist rate.
	N int

	// Domain specifies the domain at which to sample this function.
	// If Domain is nil, it defaults to DomainData{}.
	Domain FunctionDomainer

	// Kernel is the kernel to use for the KDE.
	Kernel stats.KDEKernel

	// Bandwidth is the bandwidth to use for the KDE.
	//
	// If this is zero, the bandwidth is computed from the data
	// using a default bandwidth estimator (currently
	// stats.BandwidthScott).
	Bandwidth float64

	// BoundaryMethod is the boundary correction method to use for
	// the KDE. The default value is BoundaryReflect; however, the
	// default bounds are effectively +/-inf, which is equivalent
	// to performing no boundary correction.
	BoundaryMethod stats.KDEBoundaryMethod

	// [BoundaryMin, BoundaryMax) specify a bounded support for
	// the KDE. If both are 0 (their default values), they are
	// treated as +/-inf.
	//
	// To specify a half-bounded support, set Min to math.Inf(-1)
	// or Max to math.Inf(1).
	BoundaryMin float64
	BoundaryMax float64
}

func (d Density) F(g table.Grouping) table.Grouping {
	kde := stats.KDE{
		Kernel:         d.Kernel,
		Bandwidth:      d.Bandwidth,
		BoundaryMethod: d.BoundaryMethod,
		BoundaryMin:    d.BoundaryMin,
		BoundaryMax:    d.BoundaryMax,
	}
	dname, cname := "probability density", "cumulative density"

	addEmpty := func(out *table.Builder) {
		out.Add(dname, []float64{})
		out.Add(cname, []float64{})
	}

	return Function{
		X: d.X, N: d.N, Domain: d.Domain,
		Fn: func(gid table.GroupID, in *table.Table, sampleAt []float64, out *table.Builder) {
			if len(sampleAt) == 0 {
				addEmpty(out)
				return
			}

			// Get input sample.
			var sample stats.Sample
			slice.Convert(&sample.Xs, in.MustColumn(d.X))
			if d.W != "" {
				slice.Convert(&sample.Weights, in.MustColumn(d.W))
				if sample.Weight() == 0 {
					addEmpty(out)
					return
				}
			}

			// Compute KDE.
			kde.Sample = sample
			if d.Bandwidth == 0 {
				kde.Bandwidth = stats.BandwidthScott(sample)
			}

			out.Add(dname, vec.Map(kde.PDF, sampleAt))
			out.Add(cname, vec.Map(kde.CDF, sampleAt))
		},
	}.F(g)
}
