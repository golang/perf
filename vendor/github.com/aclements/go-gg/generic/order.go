// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

// CanOrder returns whether the values a and b are orderable according
// to the Go language specification.
func CanOrder(a, b interface{}) bool {
	ak, bk := reflect.ValueOf(a).Kind(), reflect.ValueOf(b).Kind()
	if ak != bk {
		return false
	}
	return CanOrderR(ak)
}

var orderable = map[reflect.Kind]bool{
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
	reflect.Float32: true,
	reflect.Float64: true,
	reflect.String:  true,
}

// CanOrderR returns whether two values of kind k are orderable
// according to the Go language specification.
func CanOrderR(k reflect.Kind) bool {
	return orderable[k]
}

// Order returns the order of values a and b: -1 if a < b, 0 if a ==
// b, 1 if a > b. The results are undefined if either a or b is NaN.
//
// Order panics if a and b are not orderable according to the Go
// language specification.
func Order(a, b interface{}) int {
	return OrderR(reflect.ValueOf(a), reflect.ValueOf(b))
}

// OrderR is equivalent to Order, but operates on reflect.Values.
func OrderR(a, b reflect.Value) int {
	if a.Kind() != b.Kind() {
		panic(&TypeError{a.Type(), b.Type(), "are not orderable because they are different kinds"})
	}

	switch a.Kind() {
	case reflect.Float32, reflect.Float64:
		a, b := a.Float(), b.Float()
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		a, b := a.Int(), b.Int()
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0

	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		a, b := a.Uint(), b.Uint()
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0

	case reflect.String:
		a, b := a.String(), b.String()
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	}

	panic(&TypeError{a.Type(), nil, "is not orderable"})
}
