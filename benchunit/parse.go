// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package benchunit manipulates benchmark units and formats numbers
// in those units.
package benchunit

import (
	"fmt"
	"unicode"
)

// A Class specifies what class of unit prefixes are in use.
type Class int

const (
	// Decimal indicates values of a given unit should be scaled
	// by powers of 1000. Decimal units use the International
	// System of Units SI prefixes, such as "k", and "M".
	Decimal Class = iota
	// Binary indicates values of a given unit should be scaled by
	// powers of 1024. Binary units use the International
	// Electrotechnical Commission (IEC) binary prefixes, such as
	// "Ki" and "Mi".
	Binary
)

func (c Class) String() string {
	switch c {
	case Decimal:
		return "Decimal"
	case Binary:
		return "Binary"
	}
	return fmt.Sprintf("Class(%d)", int(c))
}

// ClassOf returns the Class of unit. If unit contains some measure of
// bytes in the numerator, this is Binary. Otherwise, it is Decimal.
func ClassOf(unit string) Class {
	p := newParser(unit)
	for p.next() {
		if (p.tok == "B" || p.tok == "MB" || p.tok == "bytes") && !p.denom {
			return Binary
		}
	}
	return Decimal
}

type parser struct {
	rest string // unparsed unit
	rpos int    // byte consumed from original unit

	// Current token
	tok   string
	pos   int  // byte offset of tok in original unit
	denom bool // current token is in denominator
}

func newParser(unit string) *parser {
	return &parser{rest: unit}
}

func (p *parser) next() bool {
	// Consume separators.
	for i, r := range p.rest {
		if r == '*' {
			p.denom = false
		} else if r == '/' {
			p.denom = true
		} else if !(r == '-' || unicode.IsSpace(r)) {
			p.rpos += i
			p.rest = p.rest[i:]
			goto tok
		}
	}
	// End of string.
	p.rest = ""
	return false

tok:
	// Consume until separator.
	end := len(p.rest)
	for i, r := range p.rest {
		if r == '*' || r == '/' || r == '-' || unicode.IsSpace(r) {
			end = i
			break
		}
	}
	p.tok = p.rest[:end]
	p.pos = p.rpos
	p.rpos += end
	p.rest = p.rest[end:]
	return true
}
