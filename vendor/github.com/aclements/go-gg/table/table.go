// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package table implements ordered, grouped two dimensional relations.
//
// There are two related abstractions: Table and Grouping.
//
// A Table is an ordered relation of rows and columns. Each column is
// a Go slice and hence must be homogeneously typed, but different
// columns may have different types. All columns in a Table have the
// same number of rows.
//
// A Grouping generalizes a Table by grouping the Table's rows into
// zero or more groups. A Table is itself a Grouping with zero or one
// groups. Most operations take a Grouping and operate on each group
// independently, though some operations sub-divide or combine groups.
//
// The structures of both Tables and Groupings are immutable. They are
// constructed using a Builder or a GroupingBuilder, respectively, and
// then "frozen" into their respective immutable data structures.
package table

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/generic/slice"
)

// TODO
//
// Rename Table to T?
//
// Make Table an interface? Then columns could be constructed lazily.
//
// Do all transformation functions as func(g Grouping) Grouping? That
// could be a "Transform" type that has easy methods for chaining. In
// a lot of cases, transformation functions could just return the
// Transform returned by another function (like MapTables).
//
// Make an error type for "unknown column".

// A Table is an immutable, ordered two dimensional relation. It
// consists of a set of named columns. Each column is a sequence of
// values of a consistent type or a constant value. All (non-constant)
// columns have the same length.
//
// The zero value of Table is the "empty table": it has no rows and no
// columns. Note that a Table may have one or more columns, but no
// rows; such a Table is *not* considered empty.
//
// A Table is also a trivial Grouping. If a Table is empty, it has no
// groups and hence the zero value of Table is also the "empty group".
// Otherwise, it consists only of the root group, RootGroupID.
type Table struct {
	cols     map[string]Slice
	consts   map[string]interface{}
	colNames []string
	len      int
}

// A Builder constructs a Table one column at a time.
//
// The zero value of a Builder represents an empty Table.
type Builder struct {
	t Table
}

// A Grouping is an immutable set of tables with identical sets of
// columns, each identified by a distinct GroupID.
//
// Visually, a Grouping can be thought of as follows:
//
//	   Col A  Col B  Col C
//	------ group /a ------
//	0   5.4    "x"     90
//	1   -.2    "y"     30
//	------ group /b ------
//	0   9.3    "a"     10
//
// Like a Table, a Grouping's structure is immutable. To construct a
// Grouping, use a GroupingBuilder.
//
// Despite the fact that GroupIDs form a hierarchy, a Grouping ignores
// this hierarchy and simply operates on a flat map of distinct
// GroupIDs to Tables.
type Grouping interface {
	// Columns returns the names of the columns in this Grouping,
	// or nil if there are no Tables or the group consists solely
	// of empty Tables. All Tables in this Grouping have the same
	// set of columns.
	Columns() []string

	// Tables returns the group IDs of the tables in this
	// Grouping.
	Tables() []GroupID

	// Table returns the Table in group gid, or nil if there is no
	// such Table.
	Table(gid GroupID) *Table
}

// A GroupingBuilder constructs a Grouping one table a time.
//
// The zero value of a GroupingBuilder represents an empty Grouping
// with no tables and no columns.
type GroupingBuilder struct {
	g        groupedTable
	colTypes []reflect.Type
}

type groupedTable struct {
	tables   map[GroupID]*Table
	groups   []GroupID
	colNames []string
}

// A Slice is a Go slice value.
//
// This is primarily for documentation. There is no way to statically
// enforce this in Go; however, functions that expect a Slice will
// panic with a *generic.TypeError if passed a non-slice value.
type Slice interface{}

func reflectSlice(s Slice) reflect.Value {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Slice {
		panic(&generic.TypeError{rv.Type(), nil, "is not a slice"})
	}
	return rv
}

// NewBuilder returns a new Builder. If t is non-nil, it populates the
// new Builder with the columns from t.
func NewBuilder(t *Table) *Builder {
	if t == nil {
		return new(Builder)
	}
	b := Builder{Table{
		cols:     make(map[string]Slice),
		consts:   make(map[string]interface{}),
		colNames: append([]string(nil), t.Columns()...),
		len:      t.len,
	}}
	for k, v := range t.cols {
		b.t.cols[k] = v
	}
	for k, v := range t.consts {
		b.t.consts[k] = v
	}
	return &b
}

// Add adds a column to b, or removes the named column if data is nil.
// If b already has a column with the given name, Add replaces it. If
// data is non-nil, it must have the same length as any existing
// columns or Add will panic.
func (b *Builder) Add(name string, data Slice) *Builder {
	if data == nil {
		// Remove the column.
		if _, ok := b.t.cols[name]; !ok {
			if _, ok := b.t.consts[name]; !ok {
				// Nothing to remove.
				return b
			}
		}
		delete(b.t.cols, name)
		delete(b.t.consts, name)
		for i, n := range b.t.colNames {
			if n == name {
				copy(b.t.colNames[i:], b.t.colNames[i+1:])
				b.t.colNames = b.t.colNames[:len(b.t.colNames)-1]
				break
			}
		}
		return b
	}

	// Are we replacing an existing column?
	_, replace := b.t.cols[name]
	if !replace {
		_, replace = b.t.consts[name]
	}

	// Check the column and add it.
	rv := reflectSlice(data)
	dataLen := rv.Len()
	if len(b.t.cols) == 0 || (replace && len(b.t.cols) == 1) {
		if b.t.cols == nil {
			b.t.cols = make(map[string]Slice)
		}
		// First non-constant column (possibly replacing the
		// only non-constant column).
		b.t.cols[name] = data
		b.t.len = dataLen
	} else if b.t.len != dataLen {
		panic(fmt.Sprintf("cannot add column %q with %d elements to table with %d rows", name, dataLen, b.t.len))
	} else {
		b.t.cols[name] = data
	}

	if replace {
		// Make sure it's not in constants.
		delete(b.t.consts, name)
	} else {
		b.t.colNames = append(b.t.colNames, name)
	}

	return b
}

// AddConst adds a constant column to b whose value is val. If b
// already has a column with this name, AddConst replaces it.
//
// A constant column has the same value in every row of the Table. It
// does not itself have an inherent length.
func (b *Builder) AddConst(name string, val interface{}) *Builder {
	// Are we replacing an existing column?
	_, replace := b.t.cols[name]
	if !replace {
		_, replace = b.t.consts[name]
	}

	if b.t.consts == nil {
		b.t.consts = make(map[string]interface{})
	}
	b.t.consts[name] = val

	if replace {
		// Make sure it's not in cols.
		delete(b.t.cols, name)
	} else {
		b.t.colNames = append(b.t.colNames, name)
	}

	return b
}

// Has returns true if b has a column named "name".
func (b *Builder) Has(name string) bool {
	if _, ok := b.t.cols[name]; ok {
		return true
	}
	if _, ok := b.t.consts[name]; ok {
		return true
	}
	return false
}

// Done returns the constructed Table and resets b.
func (b *Builder) Done() *Table {
	if len(b.t.colNames) == 0 {
		return new(Table)
	}
	t := b.t
	b.t = Table{}
	return &t
}

// Len returns the number of rows in Table t.
func (t *Table) Len() int {
	return t.len
}

// Columns returns the names of the columns in Table t, or nil if this
// Table is empty.
func (t *Table) Columns() []string {
	return t.colNames
}

// Column returns the slice of data in column name of Table t, or nil
// if there is no such column. If name is a constant column, this
// returns a slice with the constant value repeated to the length of
// the Table.
func (t *Table) Column(name string) Slice {
	if c, ok := t.cols[name]; ok {
		// It's a regular column or a constant column with a
		// cached expansion.
		return c
	}

	if cv, ok := t.consts[name]; ok {
		// Expand the constant column and cache the result.
		expanded := slice.Repeat(cv, t.len)
		t.cols[name] = expanded
		return expanded
	}

	return nil
}

// MustColumn is like Column, but panics if there is no such column.
func (t *Table) MustColumn(name string) Slice {
	if c := t.Column(name); c != nil {
		return c
	}
	panic(fmt.Sprintf("unknown column %q", name))
}

// Const returns the value of constant column name. If this column
// does not exist or is not a constant column, Const returns nil,
// false.
func (t *Table) Const(name string) (val interface{}, ok bool) {
	cv, ok := t.consts[name]
	return cv, ok
}

// isEmpty returns true if t is an empty Table, meaning it has no rows
// or columns.
func (t *Table) isEmpty() bool {
	return t.colNames == nil
}

// Tables returns the groups IDs in this Table. If t is empty, there
// are no group IDs. Otherwise, there is only RootGroupID.
func (t *Table) Tables() []GroupID {
	if t.isEmpty() {
		return []GroupID{}
	}
	return []GroupID{RootGroupID}
}

// Table returns t if gid is RootGroupID and t is not empty; otherwise
// it returns nil.
func (t *Table) Table(gid GroupID) *Table {
	if gid == RootGroupID && !t.isEmpty() {
		return t
	}
	return nil
}

// NewGroupingBuilder returns a new GroupingBuilder. If g is non-nil,
// it populates the new GroupingBuilder with the tables from g.
func NewGroupingBuilder(g Grouping) *GroupingBuilder {
	if g == nil {
		return new(GroupingBuilder)
	}
	b := GroupingBuilder{groupedTable{
		tables:   make(map[GroupID]*Table),
		groups:   append([]GroupID(nil), g.Tables()...),
		colNames: append([]string(nil), g.Columns()...),
	}, nil}
	for _, gid := range g.Tables() {
		t := g.Table(gid)
		b.g.tables[gid] = t
		if b.colTypes == nil {
			b.colTypes = colTypes(t)
		}
	}
	return &b
}

func colTypes(t *Table) []reflect.Type {
	colTypes := make([]reflect.Type, len(t.colNames))
	for i, col := range t.colNames {
		if c, ok := t.cols[col]; ok {
			colTypes[i] = reflect.TypeOf(c).Elem()
		} else {
			colTypes[i] = reflect.TypeOf(t.consts[col])
		}
	}
	return colTypes
}

// Add adds a Table to b, or removes a table if t is nil. If t is the
// empty Table, this is a no-op because the empty Table contains no
// groups. If gid already exists, Add replaces it. Table t must have
// the same columns as any existing Tables in this Grouping and they
// must have identical types; otherwise, Add will panic.
//
// TODO This doesn't make it easy to combine two Groupings. It could
// instead take a Grouping and reparent it.
func (b *GroupingBuilder) Add(gid GroupID, t *Table) *GroupingBuilder {
	if t == nil {
		if _, ok := b.g.tables[gid]; !ok {
			// Nothing to remove.
			return b
		}
		delete(b.g.tables, gid)
		for i, g2 := range b.g.groups {
			if g2 == gid {
				copy(b.g.groups[i:], b.g.groups[i+1:])
				b.g.groups = b.g.groups[:len(b.g.groups)-1]
				break
			}
		}
		return b
	}

	if t != nil && t.isEmpty() {
		// Adding an empty table has no effect.
		return b
	}

	if len(b.g.groups) == 1 && b.g.groups[0] == gid {
		// We're replacing the only group. This is allowed to
		// change the shape of the Grouping.
		b.g.tables[gid] = t
		b.g.colNames = t.Columns()
		b.colTypes = colTypes(t)
		return b
	} else if len(b.g.groups) == 0 {
		b.g.tables = map[GroupID]*Table{gid: t}
		b.g.groups = []GroupID{gid}
		b.g.colNames = t.Columns()
		b.colTypes = colTypes(t)
		return b
	}

	// Check that t's column names match.
	matches := true
	if len(t.colNames) != len(b.g.colNames) {
		matches = false
	} else {
		for i, n := range t.colNames {
			if b.g.colNames[i] != n {
				matches = false
				break
			}
		}
	}
	if !matches {
		panic(fmt.Sprintf("table columns %q do not match group columns %q", t.colNames, b.g.colNames))
	}

	// Check that t's column types match.
	for i, col := range b.g.colNames {
		t0 := b.colTypes[i]
		var t1 reflect.Type
		if c, ok := t.cols[col]; ok {
			t1 = reflect.TypeOf(c).Elem()
		} else if cv, ok := t.consts[col]; ok {
			t1 = reflect.TypeOf(cv)
		}
		if t0 != t1 {
			panic(&generic.TypeError{t0, t1, fmt.Sprintf("for column %q are not the same", col)})
		}
	}

	// Add t.
	if _, ok := b.g.tables[gid]; !ok {
		b.g.groups = append(b.g.groups, gid)
	}
	b.g.tables[gid] = t

	return b
}

// Done returns the constructed Grouping and resets b.
func (b *GroupingBuilder) Done() Grouping {
	if len(b.g.groups) == 0 {
		return new(groupedTable)
	}
	g := b.g
	b.g = groupedTable{}
	return &g
}

func (g *groupedTable) Columns() []string {
	return g.colNames
}

func (g *groupedTable) Tables() []GroupID {
	return g.groups
}

func (g *groupedTable) Table(gid GroupID) *Table {
	return g.tables[gid]
}

// ColType returns the type of column col in g. This will always be a
// slice type, even if col is a constant column. ColType panics if col
// is unknown.
//
// TODO: If I introduce a first-class representation for a grouped
// column, this should probably be in that.
func ColType(g Grouping, col string) reflect.Type {
	tables := g.Tables()
	if len(tables) == 0 {
		panic(fmt.Sprintf("unknown column %q", col))
	}
	t0 := g.Table(tables[0])
	if cv, ok := t0.Const(col); ok {
		return reflect.SliceOf(reflect.TypeOf(cv))
	}
	return reflect.TypeOf(t0.MustColumn(col))
}
