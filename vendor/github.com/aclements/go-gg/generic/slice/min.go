// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"
	"sort"

	"github.com/aclements/go-gg/generic"
)

// Min returns the minimum value in v. v must either implement
// sort.Interface or its elements must be orderable. Min panics if v
// is empty.
func Min(v T) interface{} {
	x, _ := minmax(v, -1, true)
	return x.Interface()
}

// ArgMin returns the index of the minimum value in v. If there are
// multiple indexes equal to the minimum value, ArgMin returns the
// lowest of them. v must be a slice whose elements are orderable, or
// must implement sort.Interface. ArgMin panics if v is empty.
func ArgMin(v interface{}) int {
	_, i := minmax(v, -1, false)
	return i
}

// Max returns the maximum value in v. v must either implement
// sort.Interface or its elements must be orderable. Max panics if v
// is empty.
func Max(v T) interface{} {
	x, _ := minmax(v, 1, true)
	return x.Interface()
}

// ArgMax returns the index of the maximum value in v. If there are
// multiple indexes equal to the maximum value, ArgMax returns the
// lowest of them. v must be a slice whose elements are orderable, or
// must implement sort.Interface. ArgMax panics if v is empty.
func ArgMax(v interface{}) int {
	_, i := minmax(v, 1, false)
	return i
}

func minmax(v interface{}, keep int, val bool) (reflect.Value, int) {
	switch v := v.(type) {
	case sort.Interface:
		if v.Len() == 0 {
			if keep < 0 {
				panic("zero-length sequence has no minimum")
			} else {
				panic("zero-length sequence has no maximum")
			}
		}
		maxi := 0
		if keep < 0 {
			for i, len := 0, v.Len(); i < len; i++ {
				if v.Less(i, maxi) {
					maxi = i
				}
			}
		} else {
			for i, len := 0, v.Len(); i < len; i++ {
				if v.Less(maxi, i) {
					maxi = i
				}
			}
		}

		if !val {
			return reflect.Value{}, maxi
		}

		rv := reflectSlice(v)
		return rv.Index(maxi), maxi
	}

	rv := reflectSlice(v)
	if !generic.CanOrderR(rv.Type().Elem().Kind()) {
		panic(&generic.TypeError{rv.Type().Elem(), nil, "is not orderable"})
	}
	if rv.Len() == 0 {
		if keep < 0 {
			panic("zero-length slice has no minimum")
		} else {
			panic("zero-length slice has no maximum")
		}
	}
	max, maxi := rv.Index(0), 0
	for i, len := 1, rv.Len(); i < len; i++ {
		if elt := rv.Index(i); generic.OrderR(elt, max) == keep {
			max, maxi = elt, i
		}
	}
	return max, maxi
}
