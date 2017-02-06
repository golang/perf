// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// Index returns the index of the first instance of val in s, or -1 if
// val is not present in s. val's type must be s's element type.
func Index(s T, val interface{}) int {
	rs := reflectSlice(s)
	if vt := reflect.TypeOf(val); rs.Type().Elem() != vt {
		// TODO: Better "<seq> is not a sequence of <val>".
		panic(&generic.TypeError{rs.Type(), vt, "cannot find"})
	}

	for i, l := 0, rs.Len(); i < l; i++ {
		if rs.Index(i).Interface() == val {
			return i
		}
	}
	return -1
}

// LastIndex returns the index of the last instance of val in s, or -1
// if val is not present in s. val's type must be s's element type.
func LastIndex(s T, val interface{}) int {
	rs := reflectSlice(s)
	if vt := reflect.TypeOf(val); rs.Type().Elem() != vt {
		// TODO: Better "<seq> is not a sequence of <val>".
		panic(&generic.TypeError{rs.Type(), vt, "cannot find"})
	}

	for i := rs.Len() - 1; i >= 0; i-- {
		if rs.Index(i).Interface() == val {
			return i
		}
	}
	return -1
}

// Contains reports whether val is within s. val's type must be s's
// element type.
func Contains(s T, val interface{}) bool {
	return Index(s, val) >= 0
}
