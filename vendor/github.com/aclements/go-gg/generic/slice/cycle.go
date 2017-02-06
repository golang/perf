// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import "reflect"

// Cycle constructs a slice of length length by repeatedly
// concatenating s to itself. If len(s) >= length, it returns
// s[:length]. Otherwise, it allocates a new slice. If len(s) == 0 and
// length != 0, Cycle panics.
func Cycle(s T, length int) T {
	rv := reflectSlice(s)
	if rv.Len() >= length {
		return rv.Slice(0, length).Interface()
	}

	if rv.Len() == 0 {
		panic("empty slice")
	}

	// Allocate a new slice of the appropriate length.
	out := reflect.MakeSlice(rv.Type(), length, length)

	// Copy elements to out.
	for pos := 0; pos < length; {
		pos += reflect.Copy(out.Slice(pos, length), rv)
	}

	return out.Interface()
}

// Repeat returns a slice consisting of length copies of v.
func Repeat(v interface{}, length int) T {
	if length < 0 {
		length = 0
	}
	rv := reflect.ValueOf(v)
	out := reflect.MakeSlice(reflect.SliceOf(rv.Type()), length, length)
	for i := 0; i < length; i++ {
		out.Index(i).Set(rv)
	}
	return out.Interface()
}
