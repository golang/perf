// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package benchproc provides tools for filtering, grouping, and
// sorting benchmark results.
//
// This package supports a pipeline processing model based around
// domain-specific languages for filtering and projecting benchmark
// results. These languages are described in "go doc
// golang.org/x/perf/benchproc/syntax".
//
// The typical steps for processing a stream of benchmark
// results are:
//
// 1. Read a stream of benchfmt.Results from one or more input sources.
// Command-line tools will often do this using benchfmt.Files.
//
// 2. For each benchfmt.Result:
//
// 2a. Optionally transform the benchfmt.Result to add or modify keys
// according to a particular tool's needs. Custom keys often start with
// "." to distinguish them from file or sub-name keys. For example,
// benchfmt.Files adds a ".file" key.
//
// 2b. Optionally filter the benchfmt.Result according to a user-provided
// predicate parsed by NewFilter. Filters can keep or discard entire
// Results, or just particular measurements from a Result.
//
// 2c. Project the benchfmt.Result using one or more Projections.
// Projecting a Result extracts a subset of the information from a
// Result into a Key. Projections are produced by a ProjectionParser,
// typically from user-provided projection expressions. An application
// should have a Projection for each "dimension" of its output. For
// example, an application that emits a table may have two Projections:
// one for rows and one for columns. An application that produces a
// scatterplot could have Projections for X and Y as well as other
// visual properties like point color and size.
//
// 2d. Group the benchfmt.Result according to the projected Keys.
// Usually this is done by storing the measurements from the Result in a
// Go map indexed by Key. Because identical Keys compare ==, they can be
// used as map keys. Applications that use two or more Projections may
// use a map of maps, or a map keyed by a struct of two Keys, or some
// combination.
//
// 3. At the end of the Results stream, once all Results have been
// grouped by their Keys, sort the Keys of each dimension using SortKeys
// and present the data in the resulting order.
package benchproc
