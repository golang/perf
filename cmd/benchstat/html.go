// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"html"
)

// FormatHTML appends an HTML formatting of the tables to buf.
func FormatHTML(buf *bytes.Buffer, tables []*Table) {
	var textTables [][]*textRow
	for _, t := range tables {
		textTables = append(textTables, toText(t))
	}

	for i, table := range textTables {
		if i > 0 {
			fmt.Fprintf(buf, "\n")
		}
		fmt.Fprintf(buf, "<style>.benchstat tbody td:nth-child(1n+2) { text-align: right; padding: 0em 1em; }</style>\n")
		fmt.Fprintf(buf, "<table class='benchstat'>\n")
		printRow := func(row *textRow, tag string) {
			fmt.Fprintf(buf, "<tr>")
			for _, cell := range row.cols {
				fmt.Fprintf(buf, "<%s>%s</%s>", tag, html.EscapeString(cell), tag)
			}
			fmt.Fprintf(buf, "\n")
		}
		printRow(table[0], "th")
		for _, row := range table[1:] {
			printRow(row, "td")
		}
		fmt.Fprintf(buf, "</table>\n")
	}
}
