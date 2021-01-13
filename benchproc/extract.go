// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/perf/benchfmt"
)

// An extractor returns some field of a benchmark result. The
// result may be a view into a mutable []byte in *benchfmt.Result, so
// it may change if the Result is modified.
type extractor func(*benchfmt.Result) []byte

// newExtractor returns a function that extracts some field of a
// benchmark result.
//
// The key must be one of the following:
//
// - ".name" for the benchmark name (excluding per-benchmark
// configuration).
//
// - ".fullname" for the full benchmark name (including per-benchmark
// configuration).
//
// - "/{key}" for a benchmark sub-name key. This may be "/gomaxprocs"
// and the extractor will normalize the name as needed.
//
// - Any other string is a file configuration key.
func newExtractor(key string) (extractor, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("key must not be empty")
	}

	switch {
	case key == ".config", key == ".unit":
		// The caller should already have handled this more gracefully.
		panic(key + " is not an extractor")

	case key == ".name":
		return extractName, nil

	case key == ".fullname":
		return extractFull, nil

	case strings.HasPrefix(key, "/"):
		// Construct the byte prefix to search for.
		prefix := make([]byte, len(key)+1)
		copy(prefix, key)
		prefix[len(prefix)-1] = '='
		isGomaxprocs := key == "/gomaxprocs"
		return func(res *benchfmt.Result) []byte {
			return extractNamePart(res, prefix, isGomaxprocs)
		}, nil
	}

	return func(res *benchfmt.Result) []byte {
		return extractConfig(res, key)
	}, nil
}

// newExtractorFullName returns an extractor for the full name of a
// benchmark, but optionally with the base name or sub-name
// configuration keys excluded. Any excluded sub-name keys will be
// deleted from the name. If ".name" is excluded, the name will be
// normalized to "*". This will ignore anything in the exclude list that
// isn't in the form of a /-prefixed sub-name key or ".name".
func newExtractorFullName(exclude []string) extractor {
	// Extract the sub-name keys, turn them into substrings and
	// construct their normalized replacement.
	//
	// It's important that we fully delete excluded sub-name keys rather
	// than, say, normalizing them to "*". Simply normalizing them will
	// leak whether or not they are present into the result, resulting
	// in different strings. This is most noticeable when excluding
	// /gomaxprocs: since this is already omitted if it's 1, benchmarks
	// running at /gomaxprocs of 1 won't merge with benchmarks at higher
	// /gomaxprocs.
	var delete [][]byte
	excName := false
	excGomaxprocs := false
	for _, k := range exclude {
		if k == ".name" {
			excName = true
		}
		if !strings.HasPrefix(k, "/") {
			continue
		}
		delete = append(delete, append([]byte(k), '='))
		if k == "/gomaxprocs" {
			excGomaxprocs = true
		}
	}
	if len(delete) == 0 && !excName && !excGomaxprocs {
		return extractFull
	}
	return func(res *benchfmt.Result) []byte {
		return extractFullExcluded(res, delete, excName, excGomaxprocs)
	}
}

func extractName(res *benchfmt.Result) []byte {
	return res.Name.Base()
}

func extractFull(res *benchfmt.Result) []byte {
	return res.Name.Full()
}

func extractFullExcluded(res *benchfmt.Result, delete [][]byte, excName, excGomaxprocs bool) []byte {
	name := res.Name.Full()
	found := false
	if excName {
		found = true
	}
	if !found {
		for _, k := range delete {
			if bytes.Contains(name, k) {
				found = true
				break
			}
		}
	}
	if !found && excGomaxprocs && bytes.IndexByte(name, '-') >= 0 {
		found = true
	}
	if !found {
		// No need to transform name.
		return name
	}

	// Delete excluded keys from the name.
	base, parts := res.Name.Parts()
	var newName []byte
	if excName {
		newName = append(newName, '*')
	} else {
		newName = append(newName, base...)
	}
outer:
	for _, part := range parts {
		for _, k := range delete {
			if bytes.HasPrefix(part, k) {
				// Skip this part.
				continue outer
			}
		}
		if excGomaxprocs && part[0] == '-' {
			// Skip gomaxprocs.
			continue outer
		}
		newName = append(newName, part...)
	}
	return newName
}

func extractNamePart(res *benchfmt.Result, prefix []byte, isGomaxprocs bool) []byte {
	_, parts := res.Name.Parts()
	if isGomaxprocs && len(parts) > 0 {
		last := parts[len(parts)-1]
		if last[0] == '-' {
			// GOMAXPROCS specified as "-N" suffix.
			return last[1:]
		}
	}
	// Search for the prefix.
	for _, part := range parts {
		if bytes.HasPrefix(part, prefix) {
			return part[len(prefix):]
		}
	}
	// Not found.
	return nil
}

func extractConfig(res *benchfmt.Result, key string) []byte {
	pos, ok := res.ConfigIndex(key)
	if !ok {
		return nil
	}
	return res.Config[pos].Value
}
