// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriter(t *testing.T) {
	const input = `BenchmarkOne 1 1 ns/op

key: val
key1: val1

BenchmarkOne 1 1 ns/op

key:

BenchmarkOne 1 1 ns/op

key: a

BenchmarkOne 1 1 ns/op

key1: val2
key: b

BenchmarkOne 1 1 ns/op
BenchmarkTwo 1 1 no-tidy-B/op
`

	out := new(strings.Builder)
	w := NewWriter(out)
	r := NewReader(bytes.NewReader([]byte(input)), "test")
	for r.Scan() {
		if err := w.Write(r.Result()); err != nil {
			t.Fatal(err)
		}
	}

	if out.String() != input {
		t.Fatalf("want:\n%sgot:\n%s", input, out.String())
	}
}
