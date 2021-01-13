// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"fmt"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchproc/internal/parse"
)

// A Filter filters benchmarks and benchmark observations.
type Filter struct {
	// match is the filter function that implements this filter.
	match filterFn
}

// filterFn is a filter function. If it matches individual measurements,
// it returns a non-nil mask (and the bool result is ignored). If it
// matches whole results, it returns a nil mask and a single boolean
// match result.
type filterFn func(res *benchfmt.Result) (mask, bool)

// NewFilter constructs a result filter from a boolean filter
// expression, such as ".name:Copy /size:4k". See "go doc
// golang.org/x/perf/benchproc/syntax" for a description of filter
// syntax.
//
// To create a filter that matches everything, pass "*" for query.
func NewFilter(query string) (*Filter, error) {
	q, err := parse.ParseFilter(query)
	if err != nil {
		return nil, err
	}

	// Recursively walk the filter expression, "compiling" it into
	// a filterFn.
	//
	// We cache extractor functions since it's common to see the
	// same key multiple times.
	extractors := make(map[string]extractor)
	var walk func(q parse.Filter) (filterFn, error)
	walk = func(q parse.Filter) (filterFn, error) {
		var err error
		switch q := q.(type) {
		case *parse.FilterOp:
			subs := make([]filterFn, len(q.Exprs))
			for i, sub := range q.Exprs {
				subs[i], err = walk(sub)
				if err != nil {
					return nil, err
				}
			}
			return filterOp(q.Op, subs), nil

		case *parse.FilterMatch:
			if q.Key == ".unit" {
				return func(res *benchfmt.Result) (mask, bool) {
					// Find the units this matches.
					m := newMask(len(res.Values))
					for i := range res.Values {
						if q.MatchString(res.Values[i].Unit) || (res.Values[i].OrigUnit != "" && q.MatchString(res.Values[i].OrigUnit)) {
							m.set(i)
						}
					}
					return m, false
				}, nil
			}

			if q.Key == ".config" {
				return nil, &parse.SyntaxError{query, q.Off, ".config is only allowed in projections"}
			}

			// Construct the extractor.
			ext := extractors[q.Key]
			if ext == nil {
				ext, err = newExtractor(q.Key)
				if err != nil {
					return nil, &parse.SyntaxError{query, q.Off, err.Error()}
				}
				extractors[q.Key] = ext
			}

			// Make the filter function.
			return func(res *benchfmt.Result) (mask, bool) {
				return nil, q.Match(ext(res))
			}, nil
		}
		panic(fmt.Sprintf("unknown query node type %T", q))
	}
	f, err := walk(q)
	if err != nil {
		return nil, err
	}
	return &Filter{f}, nil
}

func filterOp(op parse.Op, subs []filterFn) filterFn {
	switch op {
	case parse.OpNot:
		sub := subs[0]
		return func(res *benchfmt.Result) (mask, bool) {
			m, x := sub(res)
			if m == nil {
				return nil, !x
			}
			m.not()
			return m, false
		}

	case parse.OpAnd:
		return func(res *benchfmt.Result) (mask, bool) {
			var m mask
			for _, sub := range subs {
				m2, x := sub(res)
				if m2 == nil {
					if !x {
						// Short-circuit
						return nil, false
					}
				} else if m == nil {
					m = m2
				} else {
					m.and(m2)
				}
			}
			return m, true
		}

	case parse.OpOr:
		return func(res *benchfmt.Result) (mask, bool) {
			var m mask
			for _, sub := range subs {
				m2, x := sub(res)
				if m2 == nil {
					if x {
						// Short-circuit
						return nil, true
					}
				} else if m == nil {
					m = m2
				} else {
					m.or(m2)
				}
			}
			return m, false
		}
	}
	panic(fmt.Sprintf("unknown query op %v", op))
}

// Note: Right now the two methods below always return a nil error, but
// the intent is to add more complicated types to projection expressions
// (such as "commit") that may filter out results they can't parse with
// an error (e.g., "unknown commit hash").

// Apply rewrites res.Values to keep only the measurements that match
// the Filter f and reports whether any measurements remain.
//
// Apply returns true if all or part of res.Values is kept by the filter.
// Otherwise, it sets res.Values to an empty slice and returns false
// to indicate res was completely filtered out.
//
// If Apply returns false, it may return a non-nil error
// indicating why the result was filtered out.
func (f *Filter) Apply(res *benchfmt.Result) (bool, error) {
	m, err := f.Match(res)
	return m.Apply(res), err
}

// Match returns the set of res.Values that match f.
//
// In contrast with the Apply method, this does not modify the Result.
//
// If the Match is empty, it may return a non-nil error
// indicating why the result was filtered out.
func (f *Filter) Match(res *benchfmt.Result) (Match, error) {
	m, x := f.match(res)
	return Match{len(res.Values), m, x}, nil
}

type mask []uint32

func newMask(n int) mask {
	return mask(make([]uint32, (n+31)/32))
}

func (m mask) set(i int) {
	m[i/32] |= 1 << (i % 32)
}

func (m mask) and(n mask) {
	for i := range m {
		m[i] &= n[i]
	}
}

func (m mask) or(n mask) {
	for i := range m {
		m[i] |= n[i]
	}
}

func (m mask) not() {
	for i := range m {
		m[i] = ^m[i]
	}
}

// A Match records the set of result measurements that matched a filter
// query.
type Match struct {
	// n is the number of bits in this match.
	n int

	// m and x record the result of a filterFn. See filterFn for
	// their meaning.
	m mask
	x bool
}

// All reports whether all measurements in a result matched the query.
func (m *Match) All() bool {
	if m.m == nil {
		return m.x
	}
	for i, x := range m.m {
		// Set all bits above m.n.
		if x|(0xffffffff<<(m.n-i*32)) != 0xffffffff {
			return false
		}
	}
	return true
}

// Any reports whether any measurements in a result matched the query.
func (m *Match) Any() bool {
	if m.m == nil {
		return m.x
	}
	for i, x := range m.m {
		// Zero all bits above m.n.
		if x&^(0xffffffff<<(m.n-i*32)) != 0 {
			return true
		}
	}
	return false
}

// Test reports whether measurement i of a result matched the query.
func (m *Match) Test(i int) bool {
	if i < 0 || i >= m.n {
		return false
	} else if m.m == nil {
		return m.x
	}
	return m.m[i/32]&(1<<(i%32)) != 0
}

// Apply rewrites res.Values to keep only the measurements that match m.
// It reports whether any Values remain.
func (m *Match) Apply(res *benchfmt.Result) bool {
	if m.All() {
		return true
	}
	if !m.Any() {
		res.Values = res.Values[:0]
		return false
	}

	j := 0
	for i, val := range res.Values {
		if m.Test(i) {
			res.Values[j] = val
			j++
		}
	}
	res.Values = res.Values[:j]
	return j > 0
}
