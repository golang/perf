// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package texttab

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// Table does layout of text-based tables.
//
// Many of its methods return the textTable so callers can easily
// chain them to build up many cells at once.
type Table struct {
	cells []textCell
	cols  int

	shrink []bool

	curRow, curCol int
}

type textCell struct {
	row, col, span int
	value          string
	leftMargin     string
	alignment      align
}

type CellOption func(c *textCell)

func LeftMargin(x string) CellOption {
	return func(c *textCell) {
		c.leftMargin = x
	}
}

var (
	Left   CellOption = func(c *textCell) { c.alignment = alignLeft }
	Center            = func(c *textCell) { c.alignment = alignCenter }
	Right             = func(c *textCell) { c.alignment = alignRight }
)

type align int

const (
	alignLeft align = iota
	alignCenter
	alignRight
)

func (a align) lpad(s string, w int) string {
	switch a {
	default:
		return s
	case alignCenter:
		l := (w - utf8.RuneCountInString(s)) / 2
		return fmt.Sprintf("%*s%s", l, "", s)
	case alignRight:
		return fmt.Sprintf("%*s", w, s)
	}
}

// Row starts a new row in table t.
func (t *Table) Row() *Table {
	if len(t.cells) > 0 {
		t.curRow++
	}
	t.curCol = 0
	return t
}

// Col skips to column "col" in table t. Columns are numbered starting
// at 0.
func (t *Table) Col(col int) *Table {
	if col < t.curCol {
		panic(fmt.Sprintf("cannot move from column %d to earlier column %d", t.curCol, col))
	}
	t.curCol = col
	return t
}

// CurCol returns the current column index.
func (t *Table) CurCol() int {
	return t.curCol
}

// Cell adds a single-column cell at the current row and column.
func (t *Table) Cell(value string, opts ...CellOption) *Table {
	return t.Span(1, value, opts...)
}

// Span adds a multi-column cell at the current row and column.
func (t *Table) Span(cols int, value string, opts ...CellOption) *Table {
	lMargin := " "
	if t.curCol == 0 || len(value) == 0 {
		// For the left-most column or empty cells, we default
		// to no left margin.
		lMargin = ""
	}
	t.cells = append(t.cells, textCell{t.curRow, t.curCol, cols, value, lMargin, alignLeft})
	for _, o := range opts {
		o(&t.cells[len(t.cells)-1])
	}

	t.curCol += cols
	if t.curCol > t.cols {
		t.cols = t.curCol
	}

	return t
}

// SetShrink marks a column as a "shrink" column, which will have
// minimum width.
func (t *Table) SetShrink(col int, shrink bool) {
	for len(t.shrink) < col+1 {
		t.shrink = append(t.shrink, false)
	}
	t.shrink[col] = shrink
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Format lays out table t and writes it to w.
func (t *Table) Format(w io.Writer) error {
	shrink := func(col int) bool {
		if col < len(t.shrink) {
			return t.shrink[col]
		}
		return false
	}

	// Collect max length margin for each column.
	lmargin := make([]int, t.cols)
	for _, cell := range t.cells {
		lmargin[cell.col] = max(utf8.RuneCountInString(cell.leftMargin), lmargin[cell.col])
	}

	// Compute column widths, including their left margins.
	ws := make([]int, t.cols)
	// Consider cells in increasing span width.
	sort.Slice(t.cells, func(i, j int) bool {
		return t.cells[i].span < t.cells[j].span
	})
	var spanCols []int
	for _, cell := range t.cells {
		w := utf8.RuneCountInString(cell.value) + lmargin[cell.col]

		if cell.span == 1 {
			// Easy case.
			ws[cell.col] = max(ws[cell.col], w)
			continue
		}

		// This cell spans multiple columns. Is the total
		// width of those columns already sufficient?
		tw := 0
		for col := cell.col; col < cell.col+cell.span; col++ {
			tw += ws[col]
		}
		if tw >= w {
			continue
		}

		// We need to expand columns. The goal is to expand
		// them all to the necessary average, but some may
		// already be wider than this average. Hence, we
		// process columns from widest to narrowest,
		// subtracting out the columns that are already wider
		// than the target average (which in turn changes the
		// target), and then distributing the remaining space
		// among the narrower ones.
		spanCols = spanCols[:0]
		for col := cell.col; col < cell.col+cell.span; col++ {
			if shrink(col) {
				// We can't grow a shrink column, so
				// account for its space, but don't
				// add it to the columns to adjust.
				w -= ws[col]
			} else {
				spanCols = append(spanCols, col)
			}
		}
		// Process the wider columns first.
		sort.Slice(spanCols, func(i, j int) bool {
			return ws[spanCols[i]] > ws[spanCols[j]]
		})
		span := len(spanCols)
		for _, col := range spanCols {
			// What's the target average width at this
			// point? Round up w/span.
			avg := (w + span - 1) / span
			// Expand column if it isn't wide enough.
			ws[col] = max(ws[col], avg)
			// Subtract this column from the space needed.
			// If the column was already wide enough, this
			// will redistribute its excess across the
			// smaller columns. We also do this if we
			// expanded the column as a convenient way to
			// spread out the integer rounding of avg.
			w -= ws[col]
			span--
		}
	}

	// Convert column widths into starting offsets. The offset of
	// column i is where i's left margin begins. The slice
	// includes a final offset for the width of the table.
	offs := make([]int, t.cols+1)
	off := 0
	for i, w := range ws {
		offs[i] = off
		off += w
	}
	offs[len(ws)] = off

	const debugPrintColumns = false
	if debugPrintColumns {
		fmt.Println(ws)
		pos := 0
		for col, off := range offs {
			fmt.Fprintf(w, "%*s", off-pos, "")
			pos = off
			for i := 0; i < lmargin[col]; i++ {
				fmt.Fprintf(w, "|")
				pos++
			}
		}
		fmt.Fprint(w, "\n")
	}

	// Format the table. Put the cells back into top-to-bottom
	// left-to-right order.
	sort.Slice(t.cells, func(i, j int) bool {
		if t.cells[i].row != t.cells[j].row {
			return t.cells[i].row < t.cells[j].row
		}
		return t.cells[i].col < t.cells[j].col
	})
	row, off := 0, 0
	for _, cell := range t.cells {
		if strings.TrimSpace(cell.value) == "" && strings.TrimSpace(cell.leftMargin) == "" {
			// Skip empty cells. This avoids printing
			// unnecessary trailing spaces if cells appear
			// at the end of a row.
			continue
		}

		// Get to cell's row.
		for cell.row > row {
			if _, err := fmt.Fprintf(w, "\n"); err != nil {
				return err
			}
			row++
			off = 0
		}

		// Space to the cell's starting offset and print its
		// left margin.
		spaces := offs[cell.col] - off
		if _, err := fmt.Fprintf(w, "%*s%*s", spaces, "", lmargin[cell.col], cell.leftMargin); err != nil {
			return err
		}
		off += spaces + lmargin[cell.col]

		// Compute total cell width, excluding the margin we
		// just printed.
		tw := offs[cell.col+cell.span] - offs[cell.col] - lmargin[cell.col]

		// Print cell contents.
		s := cell.alignment.lpad(cell.value, tw)
		if _, err := fmt.Fprintf(w, "%s", s); err != nil {
			return err
		}
		off += utf8.RuneCountInString(s)
	}
	if len(t.cells) > 0 {
		if _, err := fmt.Fprintf(w, "\n"); err != nil {
			return err
		}
	}

	return nil
}
