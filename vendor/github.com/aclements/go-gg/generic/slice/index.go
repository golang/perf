// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// Select returns a slice w such that w[i] = v[indexes[i]].
func Select(v T, indexes []int) T {
	switch v := v.(type) {
	case []int:
		res := make([]int, len(indexes))
		for i, x := range indexes {
			res[i] = v[x]
		}
		return res

	case []float64:
		res := make([]float64, len(indexes))
		for i, x := range indexes {
			res[i] = v[x]
		}
		return res

	case []string:
		res := make([]string, len(indexes))
		for i, x := range indexes {
			res[i] = v[x]
		}
		return res
	}

	rv := reflectSlice(v)
	res := reflect.MakeSlice(rv.Type(), len(indexes), len(indexes))
	for i, x := range indexes {
		res.Index(i).Set(rv.Index(x))
	}
	return res.Interface()
}

// SelectInto assigns out[i] = in[indexes[i]]. in and out must have
// the same types and len(out) must be >= len(indexes). If in and out
// overlap, the results are undefined.
func SelectInto(out, in T, indexes []int) {
	// TODO: Maybe they should only have to be assignable?
	if it, ot := reflect.TypeOf(in), reflect.TypeOf(out); it != ot {
		panic(&generic.TypeError{it, ot, "must be the same type"})
	}

	switch in := in.(type) {
	case []int:
		out := out.([]int)
		for i, x := range indexes {
			out[i] = in[x]
		}
		return

	case []float64:
		out := out.([]float64)
		for i, x := range indexes {
			out[i] = in[x]
		}
		return

	case []string:
		out := out.([]string)
		for i, x := range indexes {
			out[i] = in[x]
		}
		return
	}

	inv, outv := reflectSlice(in), reflectSlice(out)
	for i, x := range indexes {
		outv.Index(i).Set(inv.Index(x))
	}
}
