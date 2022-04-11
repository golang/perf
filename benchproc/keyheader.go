// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

// A KeyHeader represents a slice of Keys, combined into a forest of
// trees by common Field values. This is intended to visually present a
// sequence of Keys in a compact form; primarily, as a header on a
// table.
//
// All Keys must have the same Projection. The levels of the tree
// correspond to the Fields of the Projection, and the depth of the tree
// is exactly the number of Fields. A node at level i represents a
// subslice of the Keys that all have the same values as each other for
// fields 0 through i.
//
// For example, given four keys:
//
//	K[0] = a:1 b:1 c:1
//	K[1] = a:1 b:1 c:2
//	K[2] = a:2 b:2 c:2
//	K[3] = a:2 b:3 c:3
//
// the KeyHeader is as follows:
//
//	Level 0      a:1         a:2
//	              |         /   \
//	Level 1      b:1      b:2   b:3
//	            /   \      |     |
//	Level 2   c:1   c:2   c:2   c:3
//	          K[0]  K[1]  K[2]  K[3]
type KeyHeader struct {
	// Keys is the sequence of keys at the leaf level of this tree.
	// Each level subdivides this sequence.
	Keys []Key

	// Levels are the labels for each level of the tree. Level i of the
	// tree corresponds to the i'th field of all Keys.
	Levels []*Field

	// Top is the list of tree roots. These nodes are all at level 0.
	Top []*KeyHeaderNode
}

// KeyHeaderNode is a node in a KeyHeader.
type KeyHeaderNode struct {
	// Field is the index into KeyHeader.Levels of the Field represented
	// by this node. This is also the level of this node in the tree,
	// starting with 0 at the top.
	Field int

	// Value is the value that all Keys have in common for Field.
	Value string

	// Start is the index of the first Key covered by this node.
	Start int
	// Len is the number of Keys in the sequence represented by this node.
	// This is also the number of leaf nodes in the subtree at this node.
	Len int

	// Children are the children of this node. These are all at level Field+1.
	// Child i+1 differs from child i in the value of field Field+1.
	Children []*KeyHeaderNode
}

// NewKeyHeader computes the KeyHeader of a slice of Keys by combining
// common prefixes of fields.
func NewKeyHeader(keys []Key) *KeyHeader {
	if len(keys) == 0 {
		return &KeyHeader{}
	}

	fields := commonProjection(keys).FlattenedFields()

	var walk func(parent *KeyHeaderNode)
	walk = func(parent *KeyHeaderNode) {
		level := parent.Field + 1
		if level == len(fields) {
			return
		}
		field := fields[level]
		var node *KeyHeaderNode
		for j, key := range keys[parent.Start : parent.Start+parent.Len] {
			val := key.Get(field)
			if node != nil && val == node.Value {
				// Add this key to the current node.
				node.Len++
			} else {
				// Start a new node.
				node = &KeyHeaderNode{level, val, parent.Start + j, 1, nil}
				parent.Children = append(parent.Children, node)
			}
		}
		for _, child := range parent.Children {
			walk(child)
		}
	}
	root := &KeyHeaderNode{-1, "", 0, len(keys), nil}
	walk(root)
	return &KeyHeader{keys, fields, root.Children}
}
