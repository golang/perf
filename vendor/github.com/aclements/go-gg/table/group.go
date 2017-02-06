// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/aclements/go-gg/generic/slice"
)

// GroupID identifies a group. GroupIDs form a tree, rooted at
// RootGroupID (which is also the zero GroupID).
type GroupID struct {
	*groupNode
}

// RootGroupID is the root of the GroupID tree.
var RootGroupID = GroupID{}

type groupNode struct {
	parent GroupID
	label  interface{}
}

// String returns the path to GroupID g in the form "/l1/l2/l3". If g
// is RootGroupID, it returns "/". Each level in the group is formed
// by formatting the label using fmt's "%v" verb. Note that this is
// purely diagnostic; this string may not uniquely identify g.
func (g GroupID) String() string {
	if g == RootGroupID {
		return "/"
	}
	parts := []string{}
	for p := g; p != RootGroupID; p = p.parent {
		part := fmt.Sprintf("/%v", p.label)
		parts = append(parts, part)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "")
}

// Extend returns a new GroupID that is a child of GroupID g. The
// returned GroupID will not be equal to any existing GroupID (even if
// label is not unique among g's children). The label is primarily
// diagnostic; the table package uses it only when printing tables,
// but callers may store semantic information in group labels.
func (g GroupID) Extend(label interface{}) GroupID {
	return GroupID{&groupNode{g, label}}
}

// Parent returns the parent of g. The parent of RootGroupID is
// RootGroupID.
func (g GroupID) Parent() GroupID {
	if g == RootGroupID {
		return RootGroupID
	}
	return g.parent
}

// Label returns the label of g.
func (g GroupID) Label() interface{} {
	return g.label
}

// GroupBy sub-divides all groups such that all of the rows in each
// group have equal values for all of the named columns. The relative
// order of rows with equal values for the named columns is
// maintained. Grouped-by columns become constant columns within each
// group.
func GroupBy(g Grouping, cols ...string) Grouping {
	// TODO: This would generate much less garbage if we grouped
	// all of cols in one pass.
	//
	// TODO: This constructs one slice per column per input group,
	// but it would be even better if it constructed just one
	// slice per column.

	if len(cols) == 0 {
		return g
	}

	var out GroupingBuilder
	for _, gid := range g.Tables() {
		t := g.Table(gid)

		if cv, ok := t.Const(cols[0]); ok {
			// Grouping by a constant is trivial.
			subgid := gid.Extend(cv)
			out.Add(subgid, t)
			continue
		}

		c := t.MustColumn(cols[0])

		// Create an index on c.
		type subgroupInfo struct {
			key  interface{}
			rows []int
		}
		subgroups := []subgroupInfo{}
		keys := make(map[interface{}]int)
		seq := reflect.ValueOf(c)
		for i := 0; i < seq.Len(); i++ {
			x := seq.Index(i).Interface()
			sg, ok := keys[x]
			if !ok {
				sg = len(subgroups)
				subgroups = append(subgroups, subgroupInfo{x, []int{}})
				keys[x] = sg
			}
			subgroup := &subgroups[sg]
			subgroup.rows = append(subgroup.rows, i)
		}

		// Count rows in each subgroup.
		offsets := make([]int, 1+len(subgroups))
		for i := range subgroups {
			offsets[i+1] = offsets[i] + len(subgroups[i].rows)
		}

		// Split each column.
		builders := make([]Builder, len(subgroups))
		for _, name := range t.Columns() {
			if name == cols[0] {
				// Promote the group-by column to a
				// constant.
				for i := range subgroups {
					builders[i].AddConst(name, subgroups[i].key)
				}
				continue
			}

			if cv, ok := t.Const(name); ok {
				// Keep constants constant.
				for i := range builders {
					builders[i].AddConst(name, cv)
				}
				continue
			}

			// Create a slice for all of the values.
			col := t.Column(name)
			ncol := reflect.MakeSlice(reflect.TypeOf(col), t.Len(), t.Len())

			// Shuffle each subgroup into ncol.
			for i := range subgroups {
				subcol := ncol.Slice(offsets[i], offsets[i+1]).Interface()
				slice.SelectInto(subcol, col, subgroups[i].rows)
				builders[i].Add(name, subcol)
			}
		}

		// Add tables to output Grouping.
		for i := range builders {
			subgid := gid.Extend(subgroups[i].key)
			out.Add(subgid, builders[i].Done())
		}
	}

	return GroupBy(out.Done(), cols[1:]...)
}

// Ungroup concatenates adjacent Tables in g that share a group parent
// into a Table identified by the parent, undoing the effects of the
// most recent GroupBy operation.
func Ungroup(g Grouping) Grouping {
	groups := g.Tables()
	if len(groups) == 0 || len(groups) == 1 && groups[0] == RootGroupID {
		return g
	}

	var out GroupingBuilder
	runGid := groups[0].Parent()
	runTabs := []*Table{}
	for _, gid := range groups {
		if gid.Parent() != runGid {
			// Flush the run.
			out.Add(runGid, concatRows(runTabs...))

			runGid = gid.Parent()
			runTabs = runTabs[:0]
		}
		runTabs = append(runTabs, g.Table(gid))
	}
	// Flush the last run.
	out.Add(runGid, concatRows(runTabs...))

	return out.Done()
}

// Flatten concatenates all of the groups in g into a single Table.
// This is equivalent to repeatedly Ungrouping g.
func Flatten(g Grouping) *Table {
	groups := g.Tables()
	switch len(groups) {
	case 0:
		return new(Table)

	case 1:
		return g.Table(groups[0])
	}

	tabs := make([]*Table, len(groups))
	for i, gid := range groups {
		tabs[i] = g.Table(gid)
	}

	return concatRows(tabs...)
}

// concatRows concatenates the rows of tabs into a single Table. All
// Tables in tabs must all have the same column set.
func concatRows(tabs ...*Table) *Table {
	// TODO: Consider making this public. It would have to check
	// the columns, and we would probably also want a concatCols.

	switch len(tabs) {
	case 0:
		return new(Table)

	case 1:
		return tabs[0]
	}

	// Construct each column.
	var out Builder
	seqs := make([]slice.T, len(tabs))
	for _, col := range tabs[0].Columns() {
		seqs = seqs[:0]
		for _, tab := range tabs {
			seqs = append(seqs, tab.Column(col))
		}
		out.Add(col, slice.Concat(seqs...))
	}

	return out.Done()
}
