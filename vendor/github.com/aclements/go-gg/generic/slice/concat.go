// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"

	"github.com/aclements/go-gg/generic"
)

// Concat returns the concatenation of all of ss. The types of all of
// the arguments must be identical or Concat will panic with a
// *generic.TypeError. The returned slice will have the same type as the
// inputs. If there are 0 arguments, Concat returns nil. Concat does
// not modify any of the input slices.
func Concat(ss ...T) T {
	if len(ss) == 0 {
		return nil
	}

	rvs := make([]reflect.Value, len(ss))
	total := 0
	var typ reflect.Type
	for i, s := range ss {
		rvs[i] = reflectSlice(s)
		total += rvs[i].Len()
		if i == 0 {
			typ = rvs[i].Type()
		} else if rvs[i].Type() != typ {
			panic(&generic.TypeError{typ, rvs[i].Type(), "have different types"})
		}
	}

	out := reflect.MakeSlice(typ, 0, total)
	for _, rv := range rvs {
		out = reflect.AppendSlice(out, rv)
	}
	return out.Interface()
}
