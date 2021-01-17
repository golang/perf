// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package texttab

import (
	"strings"
	"testing"
)

func TestAlign(t *testing.T) {
	check := func(s string, a align, w int, want string) {
		t.Helper()
		got := a.lpad(s, w)
		if got != want {
			t.Errorf("want %q, got %q", want, got)
		}
	}

	check("abc", alignLeft, 10, "abc")
	check("abc", alignCenter, 10, "   abc")
	check("abc", alignCenter, 11, "    abc")
	check("abc", alignRight, 10, "       abc")
	check("☃", alignRight, 4, "   ☃")
}

func TestTable(t *testing.T) {
	var tab Table
	check := func(want string) {
		t.Helper()
		var gotBuf strings.Builder
		tab.Format(&gotBuf)
		got := gotBuf.String()
		if want != got {
			t.Errorf("want:\n%sgot:\n%s", want, got)
		}
		// Reset tab.
		tab = Table{}
	}

	// Basic test.
	tab.Row().Cell("a").Cell("b").Cell("c")
	tab.Row().Cell("d").Cell("e").Cell("f")
	check("a b c\nd e f\n")

	// Basic cell padding. Also checks that we don't print
	// unnecessary spaces at the ends of lines.
	tab.Row().Cell("a").Cell("b").Cell("c")
	tab.Row().Cell("long").Cell("e").Cell("long")
	check("a    b c\nlong e long\n")

	// Cell alignment.
	tab.Row().Cell("a", Left).Cell("b", Center).Cell("c", Right)
	tab.Row().Cell("xxx").Cell("xxx").Cell("xxx")
	check("a    b    c\nxxx xxx xxx\n")

	// Margins.
	tab.Row().Cell("a").Cell("b", LeftMargin("  "))
	tab.Row().Cell("c").Cell("d")
	tab.Row().Cell("e").Cell("f", LeftMargin("|"))
	check("a  b\nc  d\ne |f\n")

	// Missing cell in the middle.
	tab.Row().Cell("a").Col(2).Cell("c")
	tab.Row().Cell("d").Cell("e").Cell("f")
	check("a   c\nd e f\n")

	// Missing cells at the end.
	tab.Row().Cell("a")
	tab.Row().Cell("d").Cell("e").Cell("f")
	check("a\nd e f\n")

	// Blank rows.
	tab.Row().Cell("a")
	tab.Row()
	tab.Row()
	tab.Row().Cell("b")
	check("a\n\n\nb\n")

	// Basic spans.
	tab.Row().Cell("a").Cell("b")
	tab.Row().Span(2, "abc")
	check("a b\nabc\n")

	// Spans expanding other cells.
	tab.Row().Cell("a").Cell("b")
	tab.Row().Span(2, "abcdefg")
	check("a   b\nabcdefg\n")

	// Other cells expanding spans.
	tab.Row().Cell("abc").Cell("def")
	tab.Row().Span(2, "a", Right)
	check("abc def\n      a\n")

	// Some cells are already large enough to complete the span.
	tab.Row().Cell("a").Cell("def")
	tab.Row().Span(2, "abdef", Right)
	check("a def\nabdef\n")

	// Larger cells are sufficient, but smaller cells need to be
	// expanded.
	tab.Row().Cell("a").Cell("def")
	tab.Row().Span(2, "abcdef", Right)
	check("a  def\nabcdef\n")

	// Spans with margins.
	tab.Row().Cell("a").Cell("b", LeftMargin("  ")).Cell("x")
	tab.Row().Span(2, "a__b").Cell("x")
	check("a  b x\na__b x\n")

	// Shrink columns.
	tab.Row().Span(2, "abcdef")
	tab.Row().Cell("a").Cell("bc")
	tab.Row().Cell("x").Cell("y")
	tab.SetShrink(1, true)
	check("abcdef\na   bc\nx   y\n")
}
