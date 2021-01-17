// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/gob"
	"encoding/json"
	"io/ioutil"
	"testing"
)

// Example benchmark used in package documentation.
func BenchmarkEncode(b *testing.B) {
	data := makeTree(4)

	b.Run("format=json", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e := json.NewEncoder(ioutil.Discard)
			err := e.Encode(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("format=gob", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e := gob.NewEncoder(ioutil.Discard)
			err := e.Encode(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

type tree struct {
	L, R *tree
}

func makeTree(depth int) *tree {
	if depth <= 0 {
		return nil
	}
	return &tree{makeTree(depth - 1), makeTree(depth - 1)}
}
