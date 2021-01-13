// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"fmt"
	"strings"
	"testing"
)

func checkKeyHeader(t *testing.T, tree *KeyHeader, want string) {
	t.Helper()
	got := renderKeyHeader(tree)
	if got != want {
		t.Errorf("want %s\ngot %s", want, got)
	}

	// Check the structure of the tree.
	prevEnd := make([]int, len(tree.Levels))
	var walk func(int, *KeyHeaderNode)
	walk = func(level int, n *KeyHeaderNode) {
		if n.Field != level {
			t.Errorf("want level %d, got %d", level, n.Field)
		}
		if n.Start != prevEnd[level] {
			t.Errorf("want start %d, got %d", prevEnd[level], n.Start)
		}
		prevEnd[level] = n.Start + n.Len
		for _, sub := range n.Children {
			walk(level+1, sub)
		}
	}
	for _, n := range tree.Top {
		walk(0, n)
	}
	// Check that we walked the full span of keys on each level.
	for level, width := range prevEnd {
		if width != len(tree.Keys) {
			t.Errorf("want width %d, got %d at level %d", len(tree.Keys), width, level)
		}
	}
}

func renderKeyHeader(t *KeyHeader) string {
	buf := new(strings.Builder)
	var walk func([]*KeyHeaderNode)
	walk = func(ns []*KeyHeaderNode) {
		for _, n := range ns {
			fmt.Fprintf(buf, "\n%s%s:%s", strings.Repeat("\t", n.Field), t.Levels[n.Field], n.Value)
			walk(n.Children)
		}
	}
	walk(t.Top)
	return buf.String()
}

func TestKeyHeader(t *testing.T) {
	// Test basic merging.
	t.Run("basic", func(t *testing.T) {
		s, _ := mustParse(t, ".config")
		c1 := p(t, s, "", "a", "a1", "b", "b1")
		c2 := p(t, s, "", "a", "a1", "b", "b2")
		tree := NewKeyHeader([]Key{c1, c2})
		checkKeyHeader(t, tree, `
a:a1
	b:b1
	b:b2`)
	})

	// Test that higher level differences prevent lower levels
	// from being merged, even if the lower levels match.
	t.Run("noMerge", func(t *testing.T) {
		s, _ := mustParse(t, ".config")
		c1 := p(t, s, "", "a", "a1", "b", "b1")
		c2 := p(t, s, "", "a", "a2", "b", "b1")
		tree := NewKeyHeader([]Key{c1, c2})
		checkKeyHeader(t, tree, `
a:a1
	b:b1
a:a2
	b:b1`)
	})

	// Test mismatched tuple lengths.
	t.Run("missingValues", func(t *testing.T) {
		s, _ := mustParse(t, ".config")
		c1 := p(t, s, "", "a", "a1")
		c2 := p(t, s, "", "a", "a1", "b", "b1")
		c3 := p(t, s, "", "a", "a1", "b", "b1", "c", "c1")
		tree := NewKeyHeader([]Key{c1, c2, c3})
		checkKeyHeader(t, tree, `
a:a1
	b:
		c:
	b:b1
		c:
		c:c1`)
	})

	// Test no Keys.
	t.Run("none", func(t *testing.T) {
		tree := NewKeyHeader([]Key{})
		if len(tree.Top) != 0 {
			t.Fatalf("wanted empty tree, got %v", tree)
		}
	})

	// Test empty Keys.
	t.Run("empty", func(t *testing.T) {
		s, _ := mustParse(t, ".config")
		c1 := p(t, s, "")
		c2 := p(t, s, "")
		tree := NewKeyHeader([]Key{c1, c2})
		checkKeyHeader(t, tree, "")
	})
}
