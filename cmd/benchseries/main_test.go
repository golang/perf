// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"testing"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/internal/benchseries"
)

var bo *benchseries.BuilderOptions

var keepFiles bool

func init() {
	bo = benchseries.BentBuilderOptions()
	bo.Filter = ".unit:ns/op"
}

var keep = flag.Bool("keep", false, "write outputs to testdata for debugging")
var update = flag.Bool("update", false, "update reference files")

// The two tests here work with two subdirectories of testdata, one containing
// benchmarks that were started on an even second, the other containing benchmarks
// started on an odd second.  Each subdirectory contains a file "reference.json"
// that in theory contains the the reference values for computer summaries (for
// an artifcially small bootstrap, to save time).

var oddReference = filepath.Join("odd", "reference.json")
var evenReference = filepath.Join("even", "reference.json")

// TestReference ensures that each subdirectory generates its
// reference file.
func TestReference(t *testing.T) {
	evens, err := filepath.Glob("testdata/even/*-opt.*")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	odds, err := filepath.Glob("testdata/odd/*-opt.*")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	evenComparisons := combineFilesAndJson(t, evens, "")
	oddComparisons := combineFilesAndJson(t, odds, "")

	if *keep {
		writeJsonFile(t, "evens.json", evenComparisons)
		writeJsonFile(t, "odds.json", oddComparisons)
	}

	if *update {
		writeJsonFile(t, evenReference, evenComparisons)
		writeJsonFile(t, oddReference, oddComparisons)
		return
	}

	be := writeJsonBytes(evenComparisons)
	bo := writeJsonBytes(oddComparisons)

	eref := filepath.Join("testdata", evenReference)
	re, err := ioutil.ReadFile(eref)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	compareBytes(t, be, re, "calculated even bytes", eref)

	oref := filepath.Join("testdata", oddReference)
	ro, err := ioutil.ReadFile(oref)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	compareBytes(t, bo, ro, "calculated odd bytes", oref)

}

// TestReordered checks the ability of benchseries to combine existing summaries
// with new results, in either order; the newly computed answers should be identical.
func TestReordered(t *testing.T) {
	evens, err := filepath.Glob("testdata/even/*-opt.*")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	odds, err := filepath.Glob("testdata/odd/*-opt.*")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	oddEvens := combineFilesAndJson(t, evens, oddReference)
	evenOdds := combineFilesAndJson(t, odds, evenReference)

	if *keep {
		writeJsonFile(t, "evenOdds.json", evenOdds)
		writeJsonFile(t, "oddEvens.json", oddEvens)
	}

	beo := writeJsonBytes(evenOdds)
	boe := writeJsonBytes(oddEvens)

	compareBytes(t, beo, boe, "evenOdds", "oddEvens")
}

func combineFilesAndJson(t *testing.T, files []string, jsonFile string) []*benchseries.ComparisonSeries {
	benchFiles := benchfmt.Files{Paths: files, AllowStdin: false, AllowLabels: true}
	builder, err := benchseries.NewBuilder(bo)

	err = builder.AddFiles(benchFiles)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	var comparisons []*benchseries.ComparisonSeries

	if jsonFile != "" {
		f, err := os.Open(filepath.Join("testdata", jsonFile))
		if err != nil {
			t.Log(err)
			t.Fail()
		}
		decoder := json.NewDecoder(f)
		decoder.Decode(&comparisons)
		f.Close()
	}

	comparisons = builder.AllComparisonSeries(comparisons, benchseries.DUPE_REPLACE)

	for _, c := range comparisons {
		c.AddSummaries(os.Stdout, 0.95, 500) // 500 is faster than 1000.
	}

	return comparisons
}

func compareBytes(t *testing.T, a, b []byte, nameA, nameB string) {
	// Editors have Opinions about trailing whitespace. Get rid of those.
	// Editors that reformat json can also cause problems here.
	a = bytes.TrimSpace(a)
	b = bytes.TrimSpace(b)
	if la, lb := len(a), len(b); la != lb {
		t.Logf("JSON outputs have different lengths, %s=%d and %s=%d", nameA, la, nameB, lb)
		t.Fail()
	}

	// Keep going after a length mismatch, the first diffence can be useful.
	if bytes.Compare(a, b) != 0 {
		line := 1
		lbegin := 0
		l := len(a)
		if l > len(b) {
			l = len(b)
		}
		for i := 0; i < l; i++ {
			if a[i] != b[i] {
				// TODO ought to deal with runes.  Benchmark names could contain multibyte runes.
				t.Logf("JSON outputs differ at line %d, character %d, byte %d", line, i-lbegin, i)
				t.Fail()
				break
			}
			if a[i] == '\n' {
				line++
				lbegin = i
			}
		}
	}

}

func writeJsonBytes(cs []*benchseries.ComparisonSeries) []byte {
	buf := new(bytes.Buffer)
	writeJson(buf, cs)
	return buf.Bytes()
}

func writeJsonFile(t *testing.T, name string, cs []*benchseries.ComparisonSeries) {
	weo, err := os.Create(filepath.Join("testdata", name))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	writeJson(weo, cs)
}

func writeJson(w io.Writer, cs []*benchseries.ComparisonSeries) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	encoder.Encode(cs)
	if wc, ok := w.(io.Closer); ok {
		wc.Close()
	}
}
