// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
)

// TODO: Have a format struct with options for things like column
// separator, and header separator. Provide some defaults ones for,
// e.g., Markdown, CSV, TSV, and such. Make top-level Print and Fprint
// call methods in some default format.

// Print(...) is shorthand for Fprint(os.Stderr, ...).
func Print(g Grouping, formats ...string) error {
	return Fprint(os.Stdout, g, formats...)
}

// Fprint prints Grouping g to w. formats[i] specifies a fmt-style
// format string for column i. If there are more columns than formats,
// remaining columns are formatted with %v (in particular, formats may
// be omitted entirely to use %v for all columns). Numeric columns are
// right aligned; all other column types are left aligned.
func Fprint(w io.Writer, g Grouping, formats ...string) error {
	if g.Columns() == nil {
		return nil
	}

	// Convert each column to strings.
	ss := make([][]string, len(g.Columns()))
	rowFmts := make([]string, len(g.Columns()))
	for i, col := range g.Columns() {
		format := "%v"
		if i < len(formats) {
			format = formats[i]
		}

		// Format column.
		var valKind reflect.Kind
		ss[i] = []string{col}
		for _, gid := range g.Tables() {
			seq := reflect.ValueOf(g.Table(gid).Column(col))
			for row := 0; row < seq.Len(); row++ {
				str := fmt.Sprintf(format, seq.Index(row).Interface())
				ss[i] = append(ss[i], str)
			}

			if valKind == reflect.Invalid {
				valKind = seq.Type().Elem().Kind()
			}
		}

		// Find column width.
		width := 0
		for _, s := range ss[i] {
			if len(s) > width {
				width = len(s)
			}
		}

		// If it's a numeric column, right align.
		//
		// TODO: Even better would be to decimal align, though
		// that may require some understanding of the format;
		// or we could only do it for the default format.
		switch valKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64:
			width = -width
		}

		if i == len(g.Columns())-1 && width > 0 {
			// Don't pad the last column.
			rowFmts[i] = "%s"
		} else {
			rowFmts[i] = fmt.Sprintf("%%%ds", -width)
		}
	}

	// Compute group headers.
	groups := []GroupID{}
	groupPos := []int{}
	lastPos := 1
	for _, gid := range g.Tables() {
		groups = append(groups, gid)
		groupPos = append(groupPos, lastPos)
		lastPos += g.Table(gid).Len()
	}
	if len(groups) == 1 && groups[0] == RootGroupID {
		groups, groupPos = nil, nil
	}

	// Print rows.
	rowFmt := strings.Join(rowFmts, "  ") + "\n"
	rowBuf := make([]interface{}, len(rowFmts))
	for row := 0; row < len(ss[0]); row++ {
		if len(groupPos) > 0 && row == groupPos[0] {
			_, err := fmt.Fprintf(w, "-- %s\n", groups[0])
			if err != nil {
				return err
			}
			groups, groupPos = groups[1:], groupPos[1:]
		}

		for col := range rowBuf {
			rowBuf[col] = ss[col][row]
		}
		_, err := fmt.Fprintf(w, rowFmt, rowBuf...)
		if err != nil {
			return err
		}
	}
	return nil
}
