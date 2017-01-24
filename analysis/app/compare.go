// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"

	"golang.org/x/perf/analysis/internal/benchstat"
	"golang.org/x/perf/storage/benchfmt"
)

// A resultGroup holds a list of results and tracks the distinct labels found in that list.
type resultGroup struct {
	// Raw list of results.
	results []*benchfmt.Result
	// LabelValues is the count of results found with each distinct (key, value) pair found in labels.
	// A value of "" counts results missing that key.
	LabelValues map[string]valueSet
}

// add adds res to the resultGroup.
func (g *resultGroup) add(res *benchfmt.Result) {
	g.results = append(g.results, res)
	if g.LabelValues == nil {
		g.LabelValues = make(map[string]valueSet)
	}
	for k, v := range res.Labels {
		if g.LabelValues[k] == nil {
			g.LabelValues[k] = make(valueSet)
			if len(g.results) > 1 {
				g.LabelValues[k][""] = len(g.results) - 1
			}
		}
		g.LabelValues[k][v]++
	}
	for k := range g.LabelValues {
		if res.Labels[k] == "" {
			g.LabelValues[k][""]++
		}
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

// valueSet is a set of values and the number of results with each value.
type valueSet map[string]int

// valueCount and byCount are used for sorting a valueSet
type valueCount struct {
	Value string
	Count int
}
type byCount []valueCount

func (s byCount) Len() int      { return len(s) }
func (s byCount) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s byCount) Less(i, j int) bool {
	if s[i].Count != s[j].Count {
		return s[i].Count > s[j].Count
	}
	return s[i].Value < s[j].Value
}

// TopN returns a slice containing n valueCount entries, and if any labels were omitted, an extra entry with value "…".
func (vs valueSet) TopN(n int) []valueCount {
	var s []valueCount
	var total int
	for v, count := range vs {
		s = append(s, valueCount{v, count})
		total += count
	}
	sort.Sort(byCount(s))
	out := s
	if len(out) > n {
		out = s[:n]
	}
	if len(out) < len(s) {
		var outTotal int
		for _, vc := range out {
			outTotal += vc.Count
		}
		out = append(out, valueCount{"…", total - outTotal})
	}
	return out
}

// addToQuery returns a new query string with add applied as a filter.
func addToQuery(query, add string) string {
	if strings.Contains(query, "|") {
		return add + " " + query
	}
	return add + " | " + query
}

// compare handles queries that require comparison of the groups in the query.
func (a *App) compare(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")

	tmpl, err := ioutil.ReadFile("template/compare.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	t, err := template.New("main").Funcs(template.FuncMap{
		"addToQuery": addToQuery,
	}).Parse(string(tmpl))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	data := a.compareQuery(q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

type compareData struct {
	Q            string
	Error        string
	Benchstat    template.HTML
	Groups       []*resultGroup
	Labels       map[string]bool
	CommonLabels benchfmt.Labels
}

func (a *App) compareQuery(q string) *compareData {
	// Parse query
	queries := parseQueryString(q)

	// Send requests
	// TODO(quentin): Issue requests in parallel?
	var groups []*resultGroup
	var found int
	for _, qPart := range queries {
		group := &resultGroup{}
		res := a.StorageClient.Query(qPart)
		for res.Next() {
			group.add(res.Result())
			found++
		}
		err := res.Err()
		res.Close()
		if err != nil {
			// TODO: If the query is invalid, surface that to the user.
			return &compareData{
				Q:     q,
				Error: err.Error(),
			}
		}
		groups = append(groups, group)
	}

	if found == 0 {
		return &compareData{
			Q:     q,
			Error: "No results matched the query string.",
		}
	}

	// Attempt to automatically split results.
	if len(groups) == 1 {
		group := groups[0]
		// Matching a single upload with multiple files -> split by file
		if len(group.LabelValues["upload"]) == 1 && len(group.LabelValues["upload-part"]) > 1 {
			groups = group.splitOn("upload-part")
		}
	}

	// Compute benchstat
	var buf bytes.Buffer
	var results [][]*benchfmt.Result
	for _, g := range groups {
		results = append(results, g.results)
	}
	benchstat.Run(&buf, results, &benchstat.Options{
		HTML: true,
	})

	// Prepare struct for template.
	labels := make(map[string]bool)
	// commonLabels are the key: value of every label that has an
	// identical value on every result.
	commonLabels := make(benchfmt.Labels)
	// Scan the first group for common labels.
	for k, vs := range groups[0].LabelValues {
		if len(vs) == 1 {
			for v := range vs {
				commonLabels[k] = v
			}
		}
	}
	// Remove any labels not common in later groups.
	for _, g := range groups[1:] {
		for k, v := range commonLabels {
			if len(g.LabelValues[k]) != 1 || g.LabelValues[k][v] == 0 {
				delete(commonLabels, k)
			}
		}
	}
	// List all labels present and not in commonLabels.
	for _, g := range groups {
		for k := range g.LabelValues {
			if commonLabels[k] != "" {
				continue
			}
			labels[k] = true
		}
	}
	data := &compareData{
		Q:            q,
		Benchstat:    template.HTML(buf.String()),
		Groups:       groups,
		Labels:       labels,
		CommonLabels: commonLabels,
	}
	return data
}
