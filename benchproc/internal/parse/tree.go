// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"regexp"
	"strings"
)

// A Filter is a node in the boolean filter. It can either be a
// FilterOp or a FilterMatch.
type Filter interface {
	isFilter()
	String() string
}

// A FilterMatch is a leaf in a Filter tree that tests a specific key
// for a match.
type FilterMatch struct {
	Key string

	// Regexp is the regular expression to match against the
	// value. This may be nil, in which case this is a literal
	// match against Lit.
	Regexp *regexp.Regexp
	// Lit is the literal value to match against the value if Regexp
	// is nil.
	Lit string

	// Off is the byte offset of the key in the original query,
	// for error reporting.
	Off int
}

func (q *FilterMatch) isFilter() {}
func (q *FilterMatch) String() string {
	if q.Regexp != nil {
		return quoteWord(q.Key) + ":/" + q.Regexp.String() + "/"
	}
	return quoteWord(q.Key) + ":" + quoteWord(q.Lit)
}

// Match returns whether q matches the given value of q.Key.
func (q *FilterMatch) Match(value []byte) bool {
	if q.Regexp != nil {
		return q.Regexp.Match(value)
	}
	return q.Lit == string(value)
}

// MatchString returns whether q matches the given value of q.Key.
func (q *FilterMatch) MatchString(value string) bool {
	if q.Regexp != nil {
		return q.Regexp.MatchString(value)
	}
	return q.Lit == value
}

// A FilterOp is a boolean operator in the Filter tree. OpNot must have
// exactly one child node. OpAnd and OpOr may have zero or more child nodes.
type FilterOp struct {
	Op    Op
	Exprs []Filter
}

func (q *FilterOp) isFilter() {}
func (q *FilterOp) String() string {
	var op string
	switch q.Op {
	case OpNot:
		return fmt.Sprintf("-%s", q.Exprs[0])
	case OpAnd:
		if len(q.Exprs) == 0 {
			return "*"
		}
		op = " AND "
	case OpOr:
		if len(q.Exprs) == 0 {
			return "-*"
		}
		op = " OR "
	}
	var buf strings.Builder
	buf.WriteByte('(')
	for i, e := range q.Exprs {
		if i > 0 {
			buf.WriteString(op)
		}
		buf.WriteString(e.String())
	}
	buf.WriteByte(')')
	return buf.String()
}

// Op specifies a type of boolean operator.
type Op int

const (
	OpAnd Op = 1 + iota
	OpOr
	OpNot
)
