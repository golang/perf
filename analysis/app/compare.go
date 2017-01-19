// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"fmt"
	"net/http"
	"sort"

	"golang.org/x/perf/storage/benchfmt"
)

// A resultGroup holds a list of results and tracks the distinct labels found in that list.
type resultGroup struct {
	// Raw list of results.
	results []*benchfmt.Result
	// LabelValues is the count of results found with each distinct (key, value) pair found in labels.
	LabelValues map[string]map[string]int
}

// add adds res to the resultGroup.
func (g *resultGroup) add(res *benchfmt.Result) {
	g.results = append(g.results, res)
	if g.LabelValues == nil {
		g.LabelValues = make(map[string]map[string]int)
	}
	for k, v := range res.Labels {
		if g.LabelValues[k] == nil {
			g.LabelValues[k] = make(map[string]int)
		}
		g.LabelValues[k][v]++
	}
}

// splitOn returns a new set of groups sharing a common value for key.
func (g *resultGroup) splitOn(key string) []*resultGroup {
	groups := make(map[string]*resultGroup)
	var values []string
	for _, res := range g.results {
		value := res.Labels[key]
		if groups[value] == nil {
			groups[value] = &resultGroup{}
			values = append(values, value)
		}
		groups[value].add(res)
	}

	sort.Strings(values)
	var out []*resultGroup
	for _, value := range values {
		out = append(out, groups[value])
	}
	return out
}

// compare handles queries that require comparison of the groups in the query.
func (a *App) compare(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")

	// Parse query
	queries := parseQueryString(q)

	// Send requests
	// TODO(quentin): Issue requests in parallel?
	var groups []*resultGroup
	for _, q := range queries {
		group := &resultGroup{}
		res := a.StorageClient.Query(q)
		defer res.Close() // TODO: Should happen each time through the loop
		for res.Next() {
			group.add(res.Result())
		}
		if err := res.Err(); err != nil {
			// TODO: If the query is invalid, surface that to the user.
			http.Error(w, err.Error(), 500)
			return
		}
		groups = append(groups, group)
	}

	// Attempt to automatically split results.
	if len(groups) == 1 {
		group := groups[0]
		// Matching a single upload with multiple files -> split by file
		if len(group.LabelValues["upload"]) == 1 && len(group.LabelValues["upload-part"]) > 1 {
			groups = group.splitOn("upload-part")
		}
	}

	// TODO: Compute benchstat

	// TODO: Render template. This is just temporary output to confirm the above works.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for i, g := range groups {
		fmt.Fprintf(w, "Group #%d: %d results\n", i, len(g.results))
		for k, vs := range g.labelValues {
			fmt.Fprintf(w, "\t%s: %#v\n", k, vs)
		}
		fmt.Fprintf(w, "\n")
	}
}
