// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// BenchmarkReader reads benchmark results from an io.Reader.
type BenchmarkReader struct {
	s       *bufio.Scanner
	labels  map[string]string
	lineNum int
}

// NewBenchmarkReader creates a BenchmarkReader that reads from r.
func NewBenchmarkReader(r io.Reader) *BenchmarkReader {
	return &BenchmarkReader{
		s:      bufio.NewScanner(r),
		labels: make(map[string]string),
	}
}

// AddLabels adds additional labels as if they had been read from the file.
// It must be called before the first call to r.Next.
func (r *BenchmarkReader) AddLabels(labels map[string]string) {
	for k, v := range labels {
		r.labels[k] = v
	}
}

// TODO: It would probably be helpful to add a named type for
// map[string]string with String(), Keys(), and Equal() methods.

// Result represents a single line from a benchmark file.
// All information about that line is self-contained in the Result.
type Result struct {
	// Labels is the set of persistent labels that apply to the result.
	// Labels must not be modified.
	Labels map[string]string
	// NameLabels is the set of ephemeral labels that were parsed
	// from the benchmark name/line.
	// NameLabels must not be modified.
	NameLabels map[string]string
	// LineNum is the line number on which the result was found
	LineNum int
	// Content is the verbatim input line of the benchmark file, beginning with the string "Benchmark".
	Content string
}

// A BenchmarkPrinter prints a sequence of benchmark results.
type BenchmarkPrinter struct {
	w      io.Writer
	labels map[string]string
}

// NewBenchmarkPrinter constructs a BenchmarkPrinter writing to w.
func NewBenchmarkPrinter(w io.Writer) *BenchmarkPrinter {
	return &BenchmarkPrinter{w: w}
}

// Print writes the lines necessary to recreate r.
func (bp *BenchmarkPrinter) Print(r *Result) error {
	var keys []string
	// Print removed keys first.
	for k := range bp.labels {
		if r.Labels[k] == "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(bp.w, "%s:\n", k); err != nil {
			return err
		}
	}
	// Then print new or changed keys.
	keys = keys[:0]
	for k, v := range r.Labels {
		if v != "" && bp.labels[k] != v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(bp.w, "%s: %s\n", k, r.Labels[k]); err != nil {
			return err
		}
	}
	// Finally print the actual line itself.
	if _, err := fmt.Fprintf(bp.w, "%s\n", r.Content); err != nil {
		return err
	}
	bp.labels = r.Labels
	return nil
}

// parseNameLabels extracts extra labels from a benchmark name and sets them in labels.
func parseNameLabels(name string, labels map[string]string) {
	dash := strings.LastIndex(name, "-")
	if dash >= 0 {
		// Accept -N as an alias for /GOMAXPROCS=N
		_, err := strconv.Atoi(name[dash+1:])
		if err == nil {
			labels["GOMAXPROCS"] = name[dash+1:]
			name = name[:dash]
		}
	}
	parts := strings.Split(name, "/")
	labels["name"] = parts[0]
	for i, sub := range parts[1:] {
		equals := strings.Index(sub, "=")
		var key string
		if equals >= 0 {
			key, sub = sub[:equals], sub[equals+1:]
		} else {
			key = fmt.Sprintf("sub%d", i+1)
		}
		labels[key] = sub
	}
}

// newResult parses a line and returns a Result object for the line.
func newResult(labels map[string]string, lineNum int, name, content string) *Result {
	r := &Result{
		Labels:     labels,
		NameLabels: make(map[string]string),
		LineNum:    lineNum,
		Content:    content,
	}
	parseNameLabels(name, r.NameLabels)
	return r
}

// copyLabels makes a new copy of the labels map, to protect against
// future modifications to labels.
func copyLabels(labels map[string]string) map[string]string {
	new := make(map[string]string)
	for k, v := range labels {
		new[k] = v
	}
	return new
}

// TODO(quentin): How to represent and efficiently group multiple lines?

// Next returns the next benchmark result from the file. If there are
// no further results, it returns nil, io.EOF.
func (r *BenchmarkReader) Next() (*Result, error) {
	copied := false
	for r.s.Scan() {
		r.lineNum++
		line := r.s.Text()
		if key, value, ok := parseKeyValueLine(line); ok {
			if !copied {
				copied = true
				r.labels = copyLabels(r.labels)
			}
			// TODO(quentin): Spec says empty value is valid, but
			// we need a way to cancel previous labels, so we'll
			// treat an empty value as a removal.
			if value == "" {
				delete(r.labels, key)
			} else {
				r.labels[key] = value
			}
			continue
		}
		if fullName, ok := parseBenchmarkLine(line); ok {
			return newResult(r.labels, r.lineNum, fullName, line), nil
		}
	}
	if err := r.s.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// parseKeyValueLine attempts to parse line as a key: value pair. ok
// indicates whether the line could be parsed.
func parseKeyValueLine(line string) (key, val string, ok bool) {
	for i, c := range line {
		if i == 0 && !unicode.IsLower(c) {
			return
		}
		if unicode.IsSpace(c) || unicode.IsUpper(c) {
			return
		}
		if i > 0 && c == ':' {
			key = line[:i]
			val = line[i+1:]
			break
		}
	}
	if val == "" {
		ok = true
		return
	}
	for len(val) > 0 && (val[0] == ' ' || val[0] == '\t') {
		val = val[1:]
		ok = true
	}
	return
}

// parseBenchmarkLine attempts to parse line as a benchmark result. If
// successful, fullName is the name of the benchmark with the
// "Benchmark" prefix stripped, and ok is true.
func parseBenchmarkLine(line string) (fullName string, ok bool) {
	space := strings.IndexFunc(line, unicode.IsSpace)
	if space < 0 {
		return
	}
	name := line[:space]
	if !strings.HasPrefix(name, "Benchmark") {
		return
	}
	return name[len("Benchmark"):], true
}
