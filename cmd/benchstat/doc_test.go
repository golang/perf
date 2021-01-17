// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Test that the examples in the command documentation do what they
// say.
func TestDoc(t *testing.T) {
	// Read the package documentation.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.NewFromFiles(fset, []*ast.File{f}, "p")
	if err != nil {
		t.Fatal(err)
	}
	tests := parseDocTests(p.Doc)
	if len(tests) == 0 {
		t.Fatal("failed to parse doc tests: found 0 tests")
	}

	// Run the tests.
	if err := os.Chdir("testdata"); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir("..")
	for _, test := range tests {
		var got, gotErr bytes.Buffer
		t.Logf("benchstat %s", strings.Join(test.args, " "))
		if err := benchstat(&got, &gotErr, test.args); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// None of the doc tests should have error output.
		if gotErr.Len() != 0 {
			t.Errorf("unexpected stderr output:\n%s", gotErr.String())
			continue
		}

		// Compare the output
		diff(t, []byte(test.want), got.Bytes())
	}
}

type docTest struct {
	args []string
	want string
}

var docTestRe = regexp.MustCompile(`(?m)^[ \t]+\$ benchstat (.*)\n((?:\t.*\n|\n)+)`)

func parseDocTests(doc string) []*docTest {
	var tests []*docTest
	for _, m := range docTestRe.FindAllStringSubmatch(doc, -1) {
		want := m[2]
		// Strip extra trailing newlines
		want = strings.TrimRight(want, "\n") + "\n"
		// Strip \t at the beginning of each line
		want = strings.Replace(want[1:], "\n\t", "\n", -1)
		tests = append(tests, &docTest{
			args: parseArgs(m[1]),
			want: want,
		})
	}
	return tests
}

// parseArgs is a very basic parser for shell-like word lists.
func parseArgs(x string) []string {
	// TODO: Use strings.Cut
	var out []string
	x = strings.Trim(x, " ")
	for len(x) > 0 {
		if x[0] == '"' {
			x = x[1:]
			i := strings.Index(x, "\"")
			if i < 0 {
				panic("missing \"")
			}
			out = append(out, x[:i])
			x = strings.TrimLeft(x[i+1:], " ")
		} else if i := strings.Index(x, " "); i < 0 {
			out = append(out, x)
			x = ""
		} else {
			out = append(out, x[:i])
			x = strings.TrimLeft(x[i+1:], " ")
		}
	}
	return out
}
