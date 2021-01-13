// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"reflect"
	"testing"
)

func TestParseProjection(t *testing.T) {
	check := func(proj string, want ...string) {
		t.Helper()
		p, err := ParseProjection(proj)
		if err != nil {
			t.Errorf("%s: unexpected error %s", proj, err)
			return
		}
		var got []string
		for _, part := range p {
			got = append(got, part.String())
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", proj, got, want)
		}
	}
	checkErr := func(proj, error string, pos int) {
		t.Helper()
		_, err := ParseProjection(proj)
		if se, _ := err.(*SyntaxError); se == nil || se.Msg != error || se.Off != pos {
			t.Errorf("%s: want error %s at %d; got %s", proj, error, pos, err)
		}
	}

	check("")
	check("a,b", "a", "b")
	check("a, b", "a", "b")
	check("a b", "a", "b")
	checkErr("a,,b", "expected key", 2)
	check("a,.name", "a", ".name")
	check("a,/b", "a", "/b")

	check("a@alpha, b@num", "a@alpha", "b@num")
	checkErr("a@", "expected named sort order or parenthesized list", 2)
	checkErr("a@,b", "expected named sort order or parenthesized list", 2)

	check("a@(1 2), b@(3 4)", "a@(1 2)", "b@(3 4)")
	checkErr("a@(", "missing )", 3)
	checkErr("a@(,", "missing )", 3)
	checkErr("a@()", "nothing to match", 3)
}
