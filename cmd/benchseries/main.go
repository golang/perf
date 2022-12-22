// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	// "github.com/dr2chase/debug-server/debug_client"
	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchseries"
	// "runtime"
)

func main() {
	// runtime.OnPanic(debug_client.TryDebug)

	bo := benchseries.BentBuilderOptions()

	var delta bool = false
	var change bool = false
	var values bool = true
	var csv bool = true
	var logScale bool = true
	var boring bool = false

	var pngDir = ""
	var svgDir = ""
	var pdfDir = ""

	var jsonOut = ""
	var jsonIn = ""

	confidence := 0.95
	threshold := 0.02

	flag.StringVar(&bo.Series, "series", bo.Series, "Specify the benchmarking key for the series x-axis")
	flag.StringVar(&bo.Experiment, "experiment", bo.Experiment, "Specify the experient-time key common to trials in an experiment")

	flag.StringVar(&bo.Compare, "compare", bo.Compare, "Specify the benchmark label/key used to select numerator and denominator in the comparison")
	flag.StringVar(&bo.Numerator, "numerator", bo.Numerator, "Numerator value of -compare key")
	flag.StringVar(&bo.Denominator, "denominator", bo.Denominator, "Denominator value of -compare key")

	flag.StringVar(&bo.NumeratorHash, "numerator-hash", bo.NumeratorHash, "Key for hash id of numerators (can be same as denominator-hash)")
	flag.StringVar(&bo.DenominatorHash, "denominator-hash", bo.DenominatorHash, "Key for hash id of denominators (can be same as numerator-hash)")

	flag.StringVar(&bo.Filter, "filter", bo.Filter, "Apply this filter to incoming benchmarks")

	flag.BoolVar(&csv, "csv", csv, "Write the series in CSV form")

	flag.BoolVar(&delta, "delta", delta, "Include the plus-or-minus range in the spreadsheet view")
	flag.BoolVar(&change, "change", change, "Include a change-detected column")
	flag.BoolVar(&values, "values", values, "Include values columns")
	flag.BoolVar(&logScale, "log", logScale, "Use a log scale in the chart")

	flag.StringVar(&pngDir, "png", pngDir, "Directory to write png chart(s) into")
	flag.StringVar(&pdfDir, "pdf", pdfDir, "Directory to write pdf chart(s) into")
	flag.StringVar(&svgDir, "svg", svgDir, "Directory to write svg chart(s) into")

	flag.StringVar(&jsonOut, "jo", jsonOut, "Save benchmarking summary in this json file")
	flag.StringVar(&jsonIn, "ji", jsonIn, "Read benchmarking summary from this json file")

	flag.Float64Var(&confidence, "confidence", confidence, "width of confidence interval")
	flag.Float64Var(&threshold, "threshold", threshold, "threshold for 'it changed' for exact metrics")
	flag.BoolVar(&boring, "boring", boring, "include the boring parts of the history")

	flag.Parse()

	bo.Warn = warn
	seriesBuilder, err := benchseries.NewBuilder(bo)

	if err != nil {
		fail("%v\n", err)
	}

	// Read supplied files
	files := benchfmt.Files{Paths: flag.Args(), AllowStdin: true, AllowLabels: true}
	err = seriesBuilder.AddFiles(files)
	if err != nil {
		fail("%v\n", err)
	}

	// Optionally read JSON of pre-existing comparisons.
	var comparisons []*benchseries.ComparisonSeries

	if jsonIn != "" {
		f, err := os.Open(jsonIn)
		if err != nil {
			fail("Could not read JSON input file (flag -ji), %v", err)
		}
		decoder := json.NewDecoder(f)
		decoder.Decode(&comparisons)
		f.Close()
	}

	// Rearrange into comparisons (not yet doing the statistical work)
	comparisons, err = seriesBuilder.AllComparisonSeries(comparisons, benchseries.DUPE_REPLACE)
	if err != nil {
		fail("Error building comparison series: %v", err)
	}

	// Chat about residues, per-table
	for _, t := range comparisons {
		fmt.Fprintf(os.Stderr, "%s residues=", t.Unit)
		first := true
		for _, r := range t.Residues {
			if len(r.Slice) == 1 {
				sep := ","
				if first {
					sep = "["
					first = false
				}
				fmt.Fprintf(os.Stderr, "%s%s=%s", sep, r.S, r.Slice[0])
			}
		}
		for _, r := range t.Residues {
			if len(r.Slice) == 2 {
				sep := ","
				if first {
					sep = "["
					first = false
				}
				fmt.Fprintf(os.Stderr, "%s%s={%s,%s}", sep, r.S, r.Slice[0], r.Slice[1])
			} else if len(r.Slice) > 2 {
				sep := ","
				if first {
					sep = "["
					first = false
				}
				fmt.Fprintf(os.Stderr, "%slen(%s)=%d", sep, r.S, len(r.Slice))
			}

		}
		fmt.Fprintf(os.Stderr, "]\n")
	}

	// Bootstrap and add (missing, if some already supplied by JSON) summaries.
	for _, c := range comparisons {
		c.AddSummaries(confidence, 1000)
	}

	// Generate some output.  Options include CSV, JSON, PNG, perhaps also PDF and SVG.

	var options benchseries.CsvOptions

	if delta {
		options |= benchseries.CSV_DELTA
	}
	if change {
		options |= benchseries.CSV_CHANGE_HEU | benchseries.CSV_CHANGE_KS
	}
	if values {
		options |= benchseries.CSV_VALUES
	}

	if jsonOut != "" {
		w, err := os.Create(jsonOut)
		if err != nil {
			fail("Could not create JSON output file (flag -jo), %v", err)
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "\t")
		encoder.Encode(comparisons)
		w.Close()
	}

	if csv {
		for _, comparison := range comparisons {
			comparison.ToCsvBootstrapped(os.Stdout, options, threshold)
		}
	}
	if pngDir != "" || pdfDir != "" || svgDir != "" {
		benchseries.Chart(comparisons, pngDir, pdfDir, svgDir, logScale, threshold, boring)
	}
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
func warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}
