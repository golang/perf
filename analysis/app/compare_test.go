// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/perf/storage/benchfmt"
)

func TestResultGroup(t *testing.T) {
	data := `key: value
BenchmarkName 1 ns/op
key: value2
BenchmarkName 1 ns/op`
	var results []*benchfmt.Result
	br := benchfmt.NewReader(strings.NewReader(data))
	g := &resultGroup{}
	for br.Next() {
		results = append(results, br.Result())
		g.add(br.Result())
	}
	if err := br.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if !reflect.DeepEqual(g.results, results) {
		t.Errorf("g.results = %#v, want %#v", g.results, results)
	}
	if want := map[string]map[string]int{"key": {"value": 1, "value2": 1}}; !reflect.DeepEqual(g.LabelValues, want) {
		t.Errorf("g.LabelValues = %#v, want %#v", g.LabelValues, want)
	}
	groups := g.splitOn("key")
	if len(groups) != 2 {
		t.Fatalf("g.splitOn returned %d groups, want 2", len(groups))
	}
	for i, results := range [][]*benchfmt.Result{
		{results[0]},
		{results[1]},
	} {
		if !reflect.DeepEqual(groups[i].results, results) {
			t.Errorf("groups[%d].results = %#v, want %#v", i, groups[i].results, results)
		}
	}
}
