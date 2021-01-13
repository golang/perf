// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syntax documents the syntax used by benchmark filter and
// projection expressions.
//
// These expressions work with benchmark data in the Go benchmark
// format (https://golang.org/design/14313-benchmark-format). Each
// benchmark result (a line beginning with "Benchmark") consists of
// several field, including a name, name-based configuration, and
// file configuration pairs ("key: value" lines).
//
// Keys
//
// Filters and projections share a common set of keys for referring to
// these fields of a benchmark result:
//
// - ".name" refers to the benchmark name, excluding per-benchmark
// configuration. For example, the ".name" of the
// "BenchmarkCopy/size=4k-16" benchmark is "Copy".
//
// - ".fullname" refers to the full benchmark name, including
// per-benchmark configuration, but excluding the "Benchmark" prefix.
// For example, the ".fullname" of "BenchmarkCopy/size=4k-16" is
// "Copy/size=4k-16".
//
// - "/{key}" refers to value of {key} from the benchmark name
// configuration. For example, the "/size" of
// "BenchmarkCopy/size=4k-16", is "4k". As a special case,
// "/gomaxprocs" recognizes both a literal "/gomaxprocs=" in the name,
// and the "-N" convention. For the above example, "/gomaxprocs" is
// "16".
//
// - Any name NOT prefixed with "/" or "." refers to the value of a
// file configuration key. For example, the "testing" package
// automatically emits a few file configuration keys, including "pkg",
// "goos", and "goarch", so the projection "pkg" extracts the package
// path of a benchmark.
//
// - ".config" (only in projections) refers to the all file
// configuration keys of a benchmark. The value of .config isn't a
// string like the other fields, but rather a tuple. .config is useful
// for grouping results by all file configuration keys to avoid grouping
// together incomparable results. For example, benchstat separates
// results with different .config into different tables.
//
// - ".unit" (only in filters) refers to individual measurements in a
// result, such as the "ns/op" measurement. The filter ".unit:ns/op"
// extracts just the ns/op measurement of a result. This will match
// both original units (e.g., "ns/op") and tidied units (e.g.,
// "sec/op").
//
// - ".file" refers to the input file provided on the command line
// (for command-line tools that use benchfmt.Files).
//
// Filters
//
// Filters are boolean expressions that match or exclude benchmark
// results or individual measurements.
//
// Filters are built from key-value terms:
//
// 	key:value     - Match if key's value is exactly "value".
// 	key:"value"   - Same, but value is a double-quoted Go string that
// 	                may contain spaces or other special characters.
// 	"key":value   - Keys may also be double-quoted.
// 	key:/regexp/  - Match if key's value matches a regular expression.
// 	key:(val1 OR val2 OR ...)
//                    - Short-hand for key:val1 OR key:val2. Values may be
//                      double-quoted strings or regexps.
// 	*             - Match everything.
//
// These terms can be combined into larger expressions as follows:
//
// 	x y ...       - Match if x, y, etc. all match.
// 	x AND y       - Same as x y.
// 	x OR y        - Match if x or y match.
// 	-x            - Match if x does not match.
// 	(...)         - Subexpression.
//
// Precise syntax:
//
//   expr     = andExpr {"OR" andExpr}
//   andExpr  = match {"AND"? match}
//   match    = "(" expr ")"
//            | "-" match
//            | "*"
//            | key ":" value
//            | key ":" "(" value {"OR" value} ")"
//   key      = word
//   value    = word
//            | "/" regexp "/"
//
// Projections
//
// A projection expresses how to extract a tuple of data from a
// benchmark result, as well as a sort order for projected tuples.
//
// A projection is a comma- or space-separated list of fields.
// Each field specifies a key and optionally a sort order and a
// filter as follows:
//
// - "key" extracts the named field and orders it using the order
// values of this key are first observed in the data.
//
// - "key@order" specifies one of the built-in named sort orders. This
// can be "alpha" or "num" for alphabetic or numeric sorting. "num"
// understands basic use of metric and IEC prefixes like "2k" and
// "1Mi".
//
// - "key@(value value ...)" specifies a fixed value order for key.
// It also specifies a filter: if key has a value that isn't any of
// the specified values, the result is filtered out.
//
// Precise syntax:
//
//   expr     = part {","? part}
//   part     = key
//            | key "@" order
//            | key "@" "(" word {word} ")"
//   key      = word
//   order    = word
//
// Common syntax
//
// Filters and projections share the following common base syntax:
//
//   word     = bareWord
//            | double-quoted Go string
//   bareWord = [^-*"():@,][^ ():@,]*
package syntax
