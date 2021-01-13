// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strings"
)

// A Field is one element in a projection expression. It represents
// extracting a single dimension of a benchmark result and applying an
// order to it.
type Field struct {
	Key string

	// Order is the sort order for this field. This can be
	// "first", meaning to sort by order of first appearance;
	// "fixed", meaning to use the explicit value order in Fixed;
	// or a named sort order.
	Order string

	// Fixed gives the explicit value order for "fixed" ordering.
	// If a record's value is not in this list, the record should
	// be filtered out. Otherwise, values should be sorted
	// according to their order in this list.
	Fixed []string

	// KeyOff and OrderOff give the byte offsets of the key and
	// order, for error reporting.
	KeyOff, OrderOff int
}

// String returns Projection as a valid projection expression.
func (p Field) String() string {
	switch p.Order {
	case "first":
		return quoteWord(p.Key)
	case "fixed":
		words := make([]string, 0, len(p.Fixed))
		for _, word := range p.Fixed {
			words = append(words, quoteWord(word))
		}
		return fmt.Sprintf("%s@(%s)", quoteWord(p.Key), strings.Join(words, " "))
	}
	return fmt.Sprintf("%s@%s", quoteWord(p.Key), quoteWord(p.Order))
}

// ParseProjection parses a projection expression into a tuple of
// Fields.
func ParseProjection(q string) ([]Field, error) {
	// Parse each projection field.
	var fields []Field
	toks := newTokenizer(q)
	for {
		// Peek at the next token.
		tok, toks2 := toks.keyOrOp()
		if tok.Kind == 0 {
			// No more fields.
			break
		} else if tok.Kind == ',' && len(fields) > 0 {
			// Consume optional separating comma.
			toks = toks2
		}

		var f Field
		f, toks = parseField(toks)
		fields = append(fields, f)
	}
	toks.end()
	if toks.errt.err != nil {
		return nil, toks.errt.err
	}
	return fields, nil
}

func parseField(toks tokenizer) (Field, tokenizer) {
	var f Field

	// Consume key.
	key, toks2 := toks.keyOrOp()
	if !(key.Kind == 'w' || key.Kind == 'q') {
		_, toks = toks.error("expected key")
		return f, toks
	}
	toks = toks2
	f.Key = key.Tok
	f.KeyOff = key.Off

	// Consume optional sort order.
	f.Order = "first"
	f.OrderOff = key.Off + len(key.Tok)
	sep, toks2 := toks.keyOrOp()
	if sep.Kind != '@' {
		// No sort order.
		return f, toks
	}
	toks = toks2

	// Is it a named sort order?
	order, toks2 := toks.keyOrOp()
	f.OrderOff = order.Off
	if order.Kind == 'w' || order.Kind == 'q' {
		f.Order = order.Tok
		return f, toks2
	}
	// Or a fixed sort order?
	if order.Kind == '(' {
		f.Order = "fixed"
		toks = toks2
		for {
			t, toks2 := toks.keyOrOp()
			if t.Kind == 'w' || t.Kind == 'q' {
				toks = toks2
				f.Fixed = append(f.Fixed, t.Tok)
			} else if t.Kind == ')' {
				if len(f.Fixed) == 0 {
					_, toks = toks.error("nothing to match")
				} else {
					toks = toks2
				}
				break
			} else {
				_, toks = toks.error("missing )")
				break
			}
		}
		return f, toks
	}
	// Bad sort order syntax.
	_, toks = toks.error("expected named sort order or parenthesized list")
	return f, toks
}
