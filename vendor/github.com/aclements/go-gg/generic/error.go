// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import "reflect"

type TypeError struct {
	Type1, Type2 reflect.Type
	Extra        string
}

func (e TypeError) Error() string {
	msg := e.Type1.String()
	if e.Type2 != nil {
		msg += " and " + e.Type2.String()
	}
	msg += " " + e.Extra
	return msg
}
