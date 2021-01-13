// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import "testing"

func TestParseFilter(t *testing.T) {
	check := func(query string, want string) {
		t.Helper()
		q, err := ParseFilter(query)
		if err != nil {
			t.Errorf("%s: unexpected error %s", query, err)
		} else if got := q.String(); got != want {
			t.Errorf("%s: got %s, want %s", query, got, want)
		}
	}
	checkErr := func(query, error string, pos int) {
		t.Helper()
		_, err := ParseFilter(query)
		if se, _ := err.(*SyntaxError); se == nil || se.Msg != error || se.Off != pos {
			t.Errorf("%s: want error %s at %d; got %s", query, error, pos, err)
		}
	}
	check(`*`, `*`)
	check(`a:b`, `a:b`)
	checkErr(`a`, "expected key:value", 0)
	checkErr(`a :`, "expected key:value", 0)
	check(`a :b`, `a:b`)
	check(`a : b`, `a:b`)
	checkErr(`a:`, "expected key:value", 0)
	checkErr(``, "expected key:value or subexpression", 0)
	checkErr(`()`, "expected key:value or subexpression", 1)
	checkErr(`AND`, "expected key:value or subexpression", 0)
	check(`"a":"b c"`, `a:"b c"`)
	check(`"a\"":"b c"`, `"a\"":"b c"`)
	check(`"a\u2603":"b c"`, `a☃:"b c"`)
	checkErr(`"a\z":"b c"`, "bad escape sequence", 0)
	checkErr(`a "b`, "missing end quote", 2)
	check("a-b:c", `a-b:c`) // "-" inside bare word
	check("a*b:c", `a*b:c`) // "*" inside bare word

	// Parens
	check(`(a:b)`, `a:b`)
	checkErr(`(a:b`, "missing \")\"", 4)
	checkErr(`(a:b))`, "unexpected \")\"", 5)

	// Operators
	check(`a:b c:d e:f`, `(a:b AND c:d AND e:f)`)
	check(`-a:b`, `-a:b`)
	check(`-*`, `-*`)
	check(`a:b AND c:d`, `(a:b AND c:d)`)
	check(`-a:b AND c:d`, `(-a:b AND c:d)`)
	check(`-(a:b AND c:d)`, `-(a:b AND c:d)`)
	check(`a:b AND * AND c:d`, `(a:b AND * AND c:d)`)
	check(`a:b OR c:d`, `(a:b OR c:d)`)
	check(`a:b AND c:d OR e:f AND g:h`, `((a:b AND c:d) OR (e:f AND g:h))`)
	check(`a:b AND (c:d OR e:f) AND g:h`, `(a:b AND (c:d OR e:f) AND g:h)`)

	// Regexp match
	checkErr("a:/b", "missing close \"/\"", 2)
	checkErr("a:/[[:foo:]]/", "error parsing regexp: invalid character class range: `[:foo:]`", 2)
	checkErr("a:/b/c", "regexp must be followed by space or an operator (unescaped \"/\"?)", 5)
	check("a:/b[/](/)\\/c/", "a:/b[/](/)\\/c/")

	// Multi-match
	check(`a:(b OR c OR d)`, `(a:b OR a:c OR a:d)`)
	check(`a:(b OR "c " OR /d/)`, `(a:b OR a:"c " OR a:/d/)`)
	checkErr(`a:(b c)`, "value list must be separated by OR", 5)
	checkErr(`a:(b AND c)`, "value list must be separated by OR", 5)
	checkErr(`a:(b OR AND)`, "expected value", 8)
	checkErr(`a:()`, "expected value", 3)
}
