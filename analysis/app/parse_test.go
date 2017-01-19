// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"reflect"
	"testing"
)

func TestParseQueryString(t *testing.T) {
	tests := []struct {
		q    string
		want []string
	}{
		{"prefix | one vs two", []string{"prefix one", "prefix two"}},
		{"prefix one vs two", []string{"prefix one", "two"}},
		{"anything else", []string{"anything else"}},
		{`one vs "two vs three"`, []string{"one", `"two vs three"`}},
		{"mixed\ttabs \"and\tspaces\"", []string{"mixed tabs \"and\tspaces\""}},
	}
	for _, test := range tests {
		t.Run(test.q, func(t *testing.T) {
			have := parseQueryString(test.q)
			if !reflect.DeepEqual(have, test.want) {
				t.Fatalf("parseQueryString = %#v, want %#v", have, test.want)
			}
		})
	}
}
