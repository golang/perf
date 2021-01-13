// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A SyntaxError is an error produced by parsing a malformed expression.
type SyntaxError struct {
	Query string // The original query string
	Off   int    // Byte offset of the error in Query
	Msg   string // Error message
}

func (e *SyntaxError) Error() string {
	// Translate byte offset to a rune offset.
	pos := 0
	for i, r := range e.Query {
		if i >= e.Off {
			break
		}
		if unicode.IsGraphic(r) {
			pos++
		}
	}
	return fmt.Sprintf("syntax error: %s\n\t%s\n\t%*s^", e.Msg, e.Query, pos, "")
}

type errorTracker struct {
	qOrig string
	err   *SyntaxError
}

func (t *errorTracker) error(q string, msg string) {
	off := len(t.qOrig) - len(q)
	if t.err == nil {
		t.err = &SyntaxError{t.qOrig, off, msg}
	}
}

// A tok is a single token in the filter/projection lexical syntax.
type tok struct {
	// Kind specifies the category of this token. It is either 'w'
	// or 'q' for an unquoted or quoted word, respectively, 'r'
	// for a regexp, an operator character, or 0 for the
	// end-of-string token.
	Kind   byte
	Off    int    // Byte offset of the beginning of this token
	Tok    string // Literal token contents; quoted words are unescaped
	Regexp *regexp.Regexp
}

type tokenizer struct {
	q    string
	errt *errorTracker
}

func newTokenizer(q string) tokenizer {
	return tokenizer{q, &errorTracker{q, nil}}
}

func isOp(ch rune) bool {
	return ch == '(' || ch == ')' || ch == ':' || ch == '@' || ch == ','
}

// At the beginning of a word, we accept "-" and "*" as operators,
// but in the middle of words we treat them as part of the word.
func isStartOp(ch rune) bool {
	return isOp(ch) || ch == '-' || ch == '*'
}

func isSpace(q string) int {
	if q[0] == ' ' {
		return 1
	}
	r, size := utf8.DecodeRuneInString(q)
	if unicode.IsSpace(r) {
		return size
	}
	return 0
}

// keyOrOp returns the next key or operator token.
// A key may be a bare word or a quoted word.
func (t *tokenizer) keyOrOp() (tok, tokenizer) {
	return t.next(false)
}

// valueOrOp returns the next value or operator token.
// A value may be a bare word, a quoted word, or a regexp.
func (t *tokenizer) valueOrOp() (tok, tokenizer) {
	return t.next(true)
}

// end asserts that t has reached the end of the token stream. If it
// has not, it returns a tokenizer the reports an error.
func (t *tokenizer) end() tokenizer {
	if tok, _ := t.keyOrOp(); tok.Kind != 0 {
		_, t2 := t.error("unexpected " + strconv.Quote(tok.Tok))
		return t2
	}
	return *t
}

func (t *tokenizer) next(allowRegexp bool) (tok, tokenizer) {
	for len(t.q) > 0 {
		if isStartOp(rune(t.q[0])) {
			return t.tok(t.q[0], t.q[:1], t.q[1:])
		} else if n := isSpace(t.q); n > 0 {
			t.q = t.q[n:]
		} else if allowRegexp && t.q[0] == '/' {
			return t.regexp()
		} else if t.q[0] == '"' {
			return t.quotedWord()
		} else {
			return t.bareWord()
		}
	}
	// Add an EOF token. This eliminates the need for lots of
	// bounds checks in the parser and gives the EOF a position.
	return t.tok(0, "", "")
}

func (t *tokenizer) tok(kind byte, token string, rest string) (tok, tokenizer) {
	off := len(t.errt.qOrig) - len(t.q)
	return tok{kind, off, token, nil}, tokenizer{rest, t.errt}
}

func (t *tokenizer) error(msg string) (tok, tokenizer) {
	t.errt.error(t.q, msg)
	// Move to the end.
	return t.tok(0, "", "")
}

func (t *tokenizer) quotedWord() (tok, tokenizer) {
	pos := 1 // Skip initial "
	for pos < len(t.q) && (t.q[pos] != '"' || t.q[pos-1] == '\\') {
		pos++
	}
	if pos == len(t.q) {
		return t.error("missing end quote")
	}
	// Parse the quoted string.
	word, err := strconv.Unquote(t.q[:pos+1])
	if err != nil {
		return t.error("bad escape sequence")
	}
	return t.tok('q', word, t.q[pos+1:])
}

func (t *tokenizer) bareWord() (tok, tokenizer) {
	// Consume until a space or operator. We only take "-"
	// as an operator immediately following another space
	// or operator so things like "foo-bar" work as
	// expected.
	end := len(t.q)
	for i, r := range t.q {
		if unicode.IsSpace(r) || isOp(r) {
			end = i
			break
		}
	}
	word := t.q[:end]
	if word == "AND" {
		return t.tok('A', word, t.q[end:])
	} else if word == "OR" {
		return t.tok('O', word, t.q[end:])
	}
	return t.tok('w', word, t.q[end:])
}

// quoteWord returns a string that tokenizes as the word s.
func quoteWord(s string) string {
	if len(s) == 0 {
		return `""`
	}
	for i, r := range s {
		switch r {
		case '"', ' ', '\a', '\b':
			return strconv.Quote(s)
		}
		if isOp(r) || unicode.IsSpace(r) || (i == 0 && (r == '-' || r == '*')) {
			return strconv.Quote(s)
		}
	}
	// No quoting necessary.
	return s
}

func (t *tokenizer) regexp() (tok, tokenizer) {
	expr, rest, err := regexpParseUntil(t.q[1:], "/")
	if err == errNoDelim {
		return t.error("missing close \"/\"")
	} else if err != nil {
		return t.error(err.Error())
	}

	r, err := regexp.Compile(expr)
	if err != nil {
		return t.error(err.Error())
	}

	// To avoid confusion when "/" appears in the regexp itself,
	// we require space or an operator after the close "/".
	q2 := rest[1:]
	if !(q2 == "" || unicode.IsSpace(rune(q2[0])) || isStartOp(rune(q2[0]))) {
		t.q = q2
		return t.error("regexp must be followed by space or an operator (unescaped \"/\"?)")
	}

	tok, next := t.tok('r', expr, q2)
	tok.Regexp = r
	return tok, next
}

var errNoDelim = errors.New("unterminated regexp")

// regexpParseUntil parses a regular expression from the beginning of str
// until the string delim appears at the top level of the expression.
// It returns the regular expression prefix of str and the remainder of str.
// If successful, rest will always begin with delim.
// If delim does not appear at the top level of str, it returns str, "", errNoDelim.
//
// TODO: There are corner cases this doesn't get right. Replace it
// with a standard library call if #44254 is implemented.
func regexpParseUntil(str, delim string) (expr, rest string, err error) {
	cs := 0
	cp := 0
	for i := 0; i < len(str); {
		if cs == 0 && cp == 0 && strings.HasPrefix(str[i:], delim) {
			return str[:i], str[i:], nil
		}
		switch str[i] {
		case '[':
			cs++
		case ']':
			if cs--; cs < 0 { // An unmatched ']' is legal.
				cs = 0
			}
		case '(':
			if cs == 0 {
				cp++
			}
		case ')':
			if cs == 0 {
				cp--
			}
		case '\\':
			i++
		}
		i++
	}
	return str, "", errNoDelim
}
