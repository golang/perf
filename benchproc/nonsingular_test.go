// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"reflect"
	"testing"
)

func TestNonSingularFields(t *testing.T) {
	s, _ := mustParse(t, ".config")

	var keys []Key
	check := func(want ...string) {
		t.Helper()
		got := NonSingularFields(keys)
		var gots []string
		for _, f := range got {
			gots = append(gots, f.Name)
		}
		if !reflect.DeepEqual(want, gots) {
			t.Errorf("want %v, got %v", want, gots)
		}
	}

	keys = []Key{}
	check()

	keys = []Key{
		p(t, s, "", "a", "1", "b", "1"),
	}
	check()

	keys = []Key{
		p(t, s, "", "a", "1", "b", "1"),
		p(t, s, "", "a", "1", "b", "1"),
		p(t, s, "", "a", "1", "b", "1"),
	}
	check()

	keys = []Key{
		p(t, s, "", "a", "1", "b", "1"),
		p(t, s, "", "a", "2", "b", "1"),
	}
	check("a")

	keys = []Key{
		p(t, s, "", "a", "1", "b", "1"),
		p(t, s, "", "a", "2", "b", "1"),
		p(t, s, "", "a", "1", "b", "2"),
	}
	check("a", "b")
}
