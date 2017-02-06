// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// T is a Go slice value of type []U.
//
// This is primarily for documentation. There is no way to statically
// enforce this in Go; however, functions that expect a slice will
// panic with a *generic.TypeError if passed a non-slice value.
type T interface{}

// reflectSlice checks that s is a slice and returns its
// reflect.Value. It panics with a *generic.TypeError if s is not a slice.
func reflectSlice(s T) reflect.Value {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Slice {
		panic(&generic.TypeError{rv.Type(), nil, "is not a slice"})
	}
	return rv
}
