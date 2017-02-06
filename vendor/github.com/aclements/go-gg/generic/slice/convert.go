// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// Convert converts each element in from and assigns it to *to. to
// must be a pointer to a slice. Convert slices or extends *to to
// len(from) and then assigns to[i] = T(from[i]) where T is the type
// of *to's elements. If from and *to have the same element type, it
// simply assigns *to = from.
func Convert(to interface{}, from T) {
	fv := reflectSlice(from)
	tv := reflect.ValueOf(to)
	if tv.Kind() != reflect.Ptr {
		panic(&generic.TypeError{tv.Type(), nil, "is not a *[]T"})
	}
	tst := tv.Type().Elem()
	if tst.Kind() != reflect.Slice {
		panic(&generic.TypeError{tv.Type(), nil, "is not a *[]T"})
	}

	if fv.Type().AssignableTo(tst) {
		tv.Elem().Set(fv)
		return
	}

	eltt := tst.Elem()
	if !fv.Type().Elem().ConvertibleTo(eltt) {
		panic(&generic.TypeError{fv.Type(), tst, "cannot be converted"})
	}

	switch to := to.(type) {
	case *[]float64:
		// This is extremely common.
		*to = (*to)[:0]
		for i, len := 0, fv.Len(); i < len; i++ {
			*to = append(*to, fv.Index(i).Convert(eltt).Float())
		}

	default:
		tsv := tv.Elem()
		tsv.SetLen(0)
		for i, len := 0, fv.Len(); i < len; i++ {
			tsv = reflect.Append(tsv, fv.Index(i).Convert(eltt))
		}
		tv.Elem().Set(tsv)
	}
}
