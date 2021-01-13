// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import "strings"

// A Key is an immutable tuple mapping from Fields to strings whose
// structure is given by a Projection. Two Keys are == if they come
// from the same Projection and have identical values.
type Key struct {
	k *keyNode
}

// IsZero reports whether k is a zeroed Key with no projection and no fields.
func (k Key) IsZero() bool {
	return k.k == nil
}

// Get returns the value of Field f in this Key.
//
// It panics if Field f does not come from the same Projection as the
// Key or if f is a tuple Field.
func (k Key) Get(f *Field) string {
	if k.IsZero() {
		panic("zero Key has no fields")
	}
	if k.k.proj != f.proj {
		panic("Key and Field have different Projections")
	}
	if f.IsTuple {
		panic(f.Name + " is a tuple field")
	}
	idx := f.idx
	if idx >= len(k.k.vals) {
		return ""
	}
	return k.k.vals[idx]
}

// Projection returns the Projection describing Key k.
func (k Key) Projection() *Projection {
	if k.IsZero() {
		return nil
	}
	return k.k.proj
}

// String returns Key as a space-separated sequence of key:value
// pairs in field order.
func (k Key) String() string {
	return k.string(true)
}

// StringValues returns Key as a space-separated sequences of
// values in field order.
func (k Key) StringValues() string {
	return k.string(false)
}

func (k Key) string(keys bool) string {
	if k.IsZero() {
		return "<zero>"
	}
	buf := new(strings.Builder)
	for _, field := range k.k.proj.FlattenedFields() {
		if field.idx >= len(k.k.vals) {
			continue
		}
		val := k.k.vals[field.idx]
		if val == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		if keys {
			buf.WriteString(field.Name)
			buf.WriteByte(':')
		}
		buf.WriteString(val)
	}
	return buf.String()
}

// commonProjection returns the Projection that all Keys have, or panics if any
// Key has a different Projection. It returns nil if len(keys) == 0.
func commonProjection(keys []Key) *Projection {
	if len(keys) == 0 {
		return nil
	}
	s := keys[0].Projection()
	for _, k := range keys[1:] {
		if k.Projection() != s {
			panic("Keys must all have the same Projection")
		}
	}
	return s
}

// keyNode is the internal heap-allocated object backing a Key.
// This allows Key itself to be a value type whose equality is
// determined by the pointer equality of the underlying keyNode.
type keyNode struct {
	proj *Projection
	// vals are the values in this Key, indexed by fieldInternal.idx. Trailing
	// ""s are always trimmed.
	//
	// Notably, this is *not* in the order of the flattened schema. This is
	// because fields can be added in the middle of a schema on-the-fly, and we
	// need to not invalidate existing Keys.
	vals []string
}

func (n *keyNode) equalRow(row []string) bool {
	if len(n.vals) != len(row) {
		return false
	}
	for i, v := range n.vals {
		if row[i] != v {
			return false
		}
	}
	return true
}
