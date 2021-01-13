// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parse implements parsers for golang.org/x/perf/benchproc/syntax.
//
// Currently this package is internal to benchproc, but if we ever
// migrate perf.golang.org to this expression syntax, it will be
// valuable to construct database queries from the same grammar.
package parse
