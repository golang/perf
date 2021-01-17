// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Note: Blocks that begin with "$ benchstat" below will be tested by
// doc_test.go.

// Benchstat computes statistical summaries and A/B comparisons of Go
// benchmarks.
//
// Usage:
//
//	benchstat [flags] inputs...
//
// Each input file should be in the Go benchmark format
// (https://golang.org/design/14313-benchmark-format), such as the
// output of “go test -bench .”. Typically, there should be two (or
// more) inputs files for before and after some change (or series of
// changes) to be measured. Each benchmark should be run at least 10
// times to gather a statistically significant sample of results. For
// each benchmark, benchstat computes the median and the confidence
// interval for the median. By default, if there are two or more
// inputs files, it compares each benchmark in the first file to the
// same benchmark in each subsequent file and reports whether there
// was a statistically significant difference, though it can be
// configured to compare on other dimensions.
//
// # Example
//
// Suppose we collect results from running a set of benchmarks 10 times
// before a particular change:
//
//	go test -run='^$' -bench=. -count=10 > old.txt
//
// And the same benchmarks 10 times after:
//
//	go test -run='^$' -bench=. -count=10 > new.txt
//
// The file old.txt contains:
//
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	BenchmarkEncode/format=json-48         	  690848	      1726 ns/op
//	BenchmarkEncode/format=json-48         	  684861	      1723 ns/op
//	BenchmarkEncode/format=json-48         	  693285	      1707 ns/op
//	BenchmarkEncode/format=json-48         	  677692	      1707 ns/op
//	BenchmarkEncode/format=json-48         	  692130	      1713 ns/op
//	BenchmarkEncode/format=json-48         	  684164	      1729 ns/op
//	BenchmarkEncode/format=json-48         	  682500	      1736 ns/op
//	BenchmarkEncode/format=json-48         	  677509	      1707 ns/op
//	BenchmarkEncode/format=json-48         	  687295	      1705 ns/op
//	BenchmarkEncode/format=json-48         	  695533	      1774 ns/op
//	BenchmarkEncode/format=gob-48          	  372699	      3069 ns/op
//	BenchmarkEncode/format=gob-48          	  394740	      3075 ns/op
//	BenchmarkEncode/format=gob-48          	  391335	      3069 ns/op
//	BenchmarkEncode/format=gob-48          	  383588	      3067 ns/op
//	BenchmarkEncode/format=gob-48          	  385885	      3207 ns/op
//	BenchmarkEncode/format=gob-48          	  389970	      3064 ns/op
//	BenchmarkEncode/format=gob-48          	  393361	      3064 ns/op
//	BenchmarkEncode/format=gob-48          	  393882	      3058 ns/op
//	BenchmarkEncode/format=gob-48          	  396171	      3059 ns/op
//	BenchmarkEncode/format=gob-48          	  397812	      3062 ns/op
//
// The file new.txt contains:
//
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	BenchmarkEncode/format=json-48         	  714387	      1423 ns/op
//	BenchmarkEncode/format=json-48         	  845445	      1416 ns/op
//	BenchmarkEncode/format=json-48         	  815714	      1411 ns/op
//	BenchmarkEncode/format=json-48         	  828824	      1413 ns/op
//	BenchmarkEncode/format=json-48         	  834070	      1412 ns/op
//	BenchmarkEncode/format=json-48         	  828123	      1426 ns/op
//	BenchmarkEncode/format=json-48         	  834493	      1422 ns/op
//	BenchmarkEncode/format=json-48         	  838406	      1424 ns/op
//	BenchmarkEncode/format=json-48         	  836227	      1447 ns/op
//	BenchmarkEncode/format=json-48         	  830835	      1425 ns/op
//	BenchmarkEncode/format=gob-48          	  394441	      3075 ns/op
//	BenchmarkEncode/format=gob-48          	  393207	      3065 ns/op
//	BenchmarkEncode/format=gob-48          	  392374	      3059 ns/op
//	BenchmarkEncode/format=gob-48          	  396037	      3065 ns/op
//	BenchmarkEncode/format=gob-48          	  393255	      3060 ns/op
//	BenchmarkEncode/format=gob-48          	  382629	      3081 ns/op
//	BenchmarkEncode/format=gob-48          	  389558	      3186 ns/op
//	BenchmarkEncode/format=gob-48          	  392668	      3135 ns/op
//	BenchmarkEncode/format=gob-48          	  392313	      3087 ns/op
//	BenchmarkEncode/format=gob-48          	  394274	      3062 ns/op
//
// The order of the lines in the file does not matter, except that the
// output lists benchmarks in order of appearance.
//
// If we run “benchstat old.txt new.txt”, it will summarize the
// benchmarks and compare the before and after results:
//
//	$ benchstat old.txt new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	                      │   old.txt   │               new.txt               │
//	                      │   sec/op    │   sec/op     vs base                │
//	Encode/format=json-48   1.718µ ± 1%   1.423µ ± 1%  -17.20% (p=0.000 n=10)
//	Encode/format=gob-48    3.066µ ± 0%   3.070µ ± 2%        ~ (p=0.446 n=10)
//	geomean                 2.295µ        2.090µ        -8.94%
//
// Before the comparison table, we see common file-level
// configuration. If there are benchmarks with different configuration
// (for example, from different packages), benchstat will print
// separate tables for each configuration.
//
// The table then compares the two input files for each benchmark. It
// shows the median and 95% confidence interval summaries for each
// benchmark before and after the change, and an A/B comparison under
// "vs base". The comparison shows that Encode/format=json got 17.20%
// faster with a p-value of 0.000 and 10 samples from each input file.
// The p-value measures how likely it is that any differences were due
// to random chance (i.e., noise). In this case, it's extremely
// unlikely the difference between the medians was due to chance. For
// Encode/format=gob, the "~" means benchstat did not detect a
// statistically significant difference between the two inputs. In
// this case, we see a p-value of 0.446, meaning it's very likely the
// differences for this benchmark are simply due to random chance.
//
// Note that "statistically significant" is not the same as "large":
// with enough low-noise data, even very small changes can be
// distinguished from noise and considered statistically significant.
// It is, of course, generally easier to distinguish large changes
// from noise.
//
// Finally, the last row of the table shows the geometric mean of each
// column, giving an overall picture of how the benchmarks changed.
// Proportional changes in the geomean reflect proportional changes in
// the benchmarks. For example, given n benchmarks, if sec/op for one
// of them increases by a factor of 2, then the sec/op geomean will
// increase by a factor of ⁿ√2.
//
// # Filtering
//
// benchstat has a very flexible system of configuring exactly which
// benchmarks are summarized and compared. First, all inputs are
// filtered according to an expression provided as the -filter flag.
//
// Filters are built from key-value terms:
//
//	key:value     - Match if key equals value.
//	key:"value"   - Same, but value is a double-quoted Go string that
//	                may contain spaces or other special characters.
//	"key":value   - Keys may also be double-quoted.
//	key:/regexp/  - Match if key matches a regular expression.
//	key:(val1 OR val2 OR ...)
//	              - Short-hand for key:val1 OR key:val2. Values may be
//	                double-quoted strings or regexps.
//	*             - Match everything.
//
// These terms can be combined into larger expressions as follows:
//
//	x y ...       - Match if x, y, etc. all match.
//	x AND y       - Same as x y.
//	x OR y        - Match if x or y match.
//	-x            - Match if x does not match.
//	(...)         - Subexpression.
//
// Each key is one of the following:
//
//	.name         - The base name of a benchmark
//	.fullname     - The full name of a benchmark (including configuration)
//	.file         - The name of the input file or user-provided file label
//	/{name-key}   - Per-benchmark sub-name configuration key
//	{file-key}    - File-level configuration key
//	.unit         - The name of a unit for a particular metric
//
// For example, the following matches benchmarks with "/format=json"
// in the sub-name keys with file-level configuration "goos" equal to
// "linux" and extracts the "ns/op" and "B/op" measurements:
//
//	$ benchstat -filter "/format:json goos:linux .unit:(ns/op OR B/op)" old.txt new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	                      │   old.txt   │               new.txt               │
//	                      │   sec/op    │   sec/op     vs base                │
//	Encode/format=json-48   1.718µ ± 1%   1.423µ ± 1%  -17.20% (p=0.000 n=10)
//
// # Configuring comparisons
//
// The way benchstat groups and compares results is configurable using
// a similar set of keys as used for filtering. By default, benchstat
// groups results into tables using all file-level configuration keys,
// then within each table, it groups results into rows by .fullname
// (the benchmark's full name) and compares across columns by .file
// (the name of each input file). This can be changed via the
// following flags:
//
//	-table KEYS   - Group results into tables by KEYS
//	-row KEYS     - Group results into table rows by KEYS
//	-col KEYS     - Compare across results with different values of KEYS
//
// Using these flags, benchstat "projects" each result into a
// particular table cell. Each KEYS argument is a comma- or
// space-separated list of keys, each of which can optionally also
// specify a sort order (described below).
//
// Each key is one of the following:
//
//	.name         - The base name of a benchmark
//	.fullname     - The full name of a benchmark (including configuration)
//	.file         - The name of the input file or user-provided file label
//	/{name-key}   - Per-benchmark sub-name configuration key
//	{file-key}    - File-level configuration key
//	.config       - All file-level configuration keys
//
// Some of these keys can overlap. For example, ".config" includes the
// file-level key "goos", and ".fullname" includes the sub-name key
// "/format". When keys overlap like this, benchstat omits the more
// specific key from the general key. For example, if -table is the
// full file-level configuration ".config", and -col is the specific
// file key "goos", benchstat will omit "goos" from ".config".
//
// Finally, the -ignore flag can list keys that benchstat should
// ignore when grouping results. Continuing the previous example, if
// -table is ".config" and -ignore is "goos", benchstat will omit
// "goos" from ".config", but also not use it for any grouping.
//
// For precise details of the filter syntax and supported keys, see
// https://pkg.go.dev/golang.org/x/perf/benchproc/syntax.
//
// # Projection examples
//
// Returning to our first example, we can now see how the default
// projection flags produce this output:
//
//	$ benchstat -table .config -row .fullname -col .file old.txt new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	                      │   old.txt   │               new.txt               │
//	                      │   sec/op    │   sec/op     vs base                │
//	Encode/format=json-48   1.718µ ± 1%   1.423µ ± 1%  -17.20% (p=0.000 n=10)
//	Encode/format=gob-48    3.066µ ± 0%   3.070µ ± 2%        ~ (p=0.446 n=10)
//	geomean                 2.295µ        2.090µ        -8.94%
//
// In this example, all benchmarks have the same file-level
// configuration, consisting of "goos", "goarch", and "pkg", so
// ".config" groups them into just one table. Within this table,
// results are grouped into rows by their full name, including
// configuration, and grouped into columns by the name of each input
// file.
//
// Suppose we instead want to compare json encoding to gob encoding
// from new.txt.
//
//	$ benchstat -col /format new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	          │    json     │                 gob                  │
//	          │   sec/op    │   sec/op     vs base                 │
//	Encode-48   1.423µ ± 1%   3.070µ ± 2%  +115.82% (p=0.000 n=10)
//
// The columns are now labeled by the "/format" configuration from the
// benchmark name. benchstat still compares columns even though we've
// only provided a single input file. We also see that /format has
// been removed from the benchmark name to make a single row.
//
// We can simplify the output by grouping rows by just the benchmark name,
// rather than the full name:
//
//	$ benchstat -col /format -row .name new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	       │    json     │                 gob                  │
//	       │   sec/op    │   sec/op     vs base                 │
//	Encode   1.423µ ± 1%   3.070µ ± 2%  +115.82% (p=0.000 n=10)
//
// benchstat will attempt to detect and warn if projections strip away
// too much information. For example, here we group together json and
// gob results into a single row:
//
//	$ benchstat  -row .name new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	       │    new.txt     │
//	       │     sec/op     │
//	Encode   2.253µ ± 37% ¹
//	¹ benchmarks vary in .fullname
//
// Since this is probably not a meaningful comparison, benchstat warns
// that the benchmarks it grouped together vary in a hidden dimension.
// If this really were our intent, we could -ignore .fullname.
//
// # Sorting
//
// By default, benchstat sorts each dimension according to the order
// in which it first observes each value of that dimension. This can
// be overridden in each projection using the following syntax:
//
// {key}@{order} - specifies one of the built-in named sort orders.
// This can be "alpha" or "num" for alphabetic or numeric sorting.
// "num" understands basic use of metric and IEC prefixes like "2k"
// and "1Mi".
//
// {key}@({value} {value} ...) - specifies a fixed value order for
// key. It also specifies a filter: if key has a value that isn't any
// of the specified values, the result is filtered out.
//
// For example, we can use a fixed order to compare the improvement of
// json over gob rather than the other way around:
//
//	$ benchstat -col "/format@(gob json)" -row .name -ignore .file new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	       │     gob     │                json                 │
//	       │   sec/op    │   sec/op     vs base                │
//	Encode   3.070µ ± 2%   1.423µ ± 1%  -53.66% (p=0.000 n=10)
//
// # Overriding .file
//
// Often, you want to compare results from different files, but want
// to provide more meaningful (or perhaps shorter) column labels than
// raw file names. File name labels can be overridden by specifying an
// input argument of the form "label=path" instead of just "path".
// This provides a custom value for the .file key.
//
// For example, the following will perform the default comparison, but
// label the columns O and N instead of old.txt and new.txt:
//
//	$ benchstat O=old.txt N=new.txt
//	goos: linux
//	goarch: amd64
//	pkg: golang.org/x/perf/cmd/benchstat/testdata
//	                      │      O      │                  N                  │
//	                      │   sec/op    │   sec/op     vs base                │
//	Encode/format=json-48   1.718µ ± 1%   1.423µ ± 1%  -17.20% (p=0.000 n=10)
//	Encode/format=gob-48    3.066µ ± 0%   3.070µ ± 2%        ~ (p=0.446 n=10)
//	geomean                 2.295µ        2.090µ        -8.94%
//
// # Units
//
// benchstat normalizes the units "ns" to "sec" and "MB" to "B" to
// avoid creating nonsense units like "µns/op". These appear in the
// testing package's default metrics and are also common in custom
// metrics.
//
// benchstat supports custom unit metadata (see
// https://golang.org/design/14313-benchmark-format). In particular,
// "assume" metadata is useful for controlling the statistics used by
// benchstat. By default, units use "assume=nothing", so benchstat
// uses non-parametric statistics: median for summaries, and the
// Mann-Whitney U-test for A/B comparisons.
//
// Some benchmarks measure things that have no noise, such as the size
// of a binary produced by a compiler. These do not benefit from
// repeated measurements or non-parametric statistics. For these
// units, it's useful to set "assume=exact". This will cause benchstat
// to warn if there's any variation in the measured values, and to
// show A/B comparisons even if there's only one before and after
// measurement.
//
// # Tips
//
// Reducing noise and/or increasing the number of benchmark runs will
// enable benchstat to discern smaller changes as "statistically
// significant". To reduce noise, make sure you run benchmarks on an
// otherwise idle machine, ideally one that isn't running on battery
// and isn't likely to be affected by thermal throttling.
// https://llvm.org/docs/Benchmarking.html has many good tips on
// reducing noise in benchmarks.
//
// It's also important that noise is evenly distributed across
// benchmark runs. The best way to do this is to interleave before and
// after runs, rather than running, say, 10 iterations of the before
// benchmark, and then 10 iterations of the after benchmark. For Go
// benchmarks, you can often speed up this process by using "go test
// -c" to pre-compile the benchmark binary.
//
// Pick a number of benchmark runs (at least 10, ideally 20) and stick
// to it. If benchstat reports no statistically significant change,
// avoid simply rerunning your benchmarks until it reports a
// significant change. This is known as "multiple testing" and is a
// common statistical error. By default, benchstat uses an ɑ threshold
// of 0.05, which means it is *expected* to show a difference 5% of
// the time even if there is no difference. Hence, if you rerun
// benchmarks looking for a change, benchstat will probably eventually
// say there is a change, even if there isn't, which creates a
// statistical bias.
//
// As an extension of this, if you compare a large number of
// benchmarks, you should expect that about 5% of them will report a
// statistically significant change even if there is no difference
// between the before and after.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
	"golang.org/x/perf/benchproc"
	"golang.org/x/perf/cmd/benchstat/internal/benchtab"
)

// TODO: Add a flag to perform Holm–Bonferroni correction for
// family-wise error rates. This can be done after-the-fact on a
// collection of benchstat.Comparison values.

// TODO: -unit flag.

// TODO: Support sorting by commit order.

// TODO: Add some quick usage examples to the -h output?

// TODO: If the projection results in a very sparse table, that's
// usually the result of correlated keys. Can we detect that and
// suggest fixes?

func main() {
	if err := benchstat(os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "benchstat: %s\n", err)
	}
}

func benchstat(w, wErr io.Writer, args []string) error {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), `Usage: benchstat [flags] inputs...

benchstat computes statistical summaries and A/B comparisons of Go
benchmarks. It shows benchmark medians in a table with a row for each
benchmark and a column for each input file. If there is more than one
input file, it also shows A/B comparisons between the files. If a
difference is likely to be noise, it shows "~".

For details, see https://pkg.go.dev/golang.org/x/perf/cmd/benchstat.
`)
		flags.PrintDefaults()
	}

	thresholds := benchmath.DefaultThresholds
	flagTable := flags.String("table", ".config", "split results into tables by distinct values of `projection`")
	flagRow := flags.String("row", ".fullname", "split results into rows by distinct values of `projection`")
	flagCol := flags.String("col", ".file", "split results into columns by distinct values of `projection`")
	flagIgnore := flags.String("ignore", "", "ignore variations in `keys`")
	flagFilter := flags.String("filter", "*", "use only benchmarks matching benchfilter `query`")
	flags.Float64Var(&thresholds.CompareAlpha, "alpha", thresholds.CompareAlpha, "consider change significant if p < `α`")
	// TODO: Support -confidence none to disable CI column? This
	// would be equivalent to benchstat v1's -norange for CSV.
	flagConfidence := flags.Float64("confidence", 0.95, "confidence `level` for ranges")
	flagFormat := flags.String("format", "text", "print results in `format`:\n  text - plain text\n  csv  - comma-separated values (warnings will be written to stderr)\n")
	flags.Parse(args)

	if flags.NArg() == 0 {
		flags.Usage()
		os.Exit(2)
	}

	filter, err := benchproc.NewFilter(*flagFilter)
	if err != nil {
		return fmt.Errorf("parsing -filter: %s", err)
	}

	var parser benchproc.ProjectionParser
	var parseErr error
	mustParse := func(name, val string, unit bool) *benchproc.Projection {
		var proj *benchproc.Projection
		var err error
		if unit {
			proj, _, err = parser.ParseWithUnit(val, filter)
		} else {
			proj, err = parser.Parse(val, filter)
		}
		if err != nil && parseErr == nil {
			parseErr = fmt.Errorf("parsing %s: %s", name, err)
		}
		return proj
	}
	tableBy := mustParse("-table", *flagTable, true)
	rowBy := mustParse("-row", *flagRow, false)
	colBy := mustParse("-col", *flagCol, false)
	mustParse("-ignore", *flagIgnore, false)
	residue := parser.Residue()
	if parseErr != nil {
		return parseErr
	}

	if thresholds.CompareAlpha < 0 || thresholds.CompareAlpha > 1 {
		return fmt.Errorf("-alpha must be in range [0, 1]")
	}
	if *flagConfidence < 0 || *flagConfidence > 1 {
		return fmt.Errorf("-confidence must be in range [0, 1]")
	}
	var format func(t *benchtab.Tables) error
	switch *flagFormat {
	default:
		return fmt.Errorf("-format must be text or csv")
	case "text":
		format = func(t *benchtab.Tables) error { return t.ToText(w, false) }
	case "csv":
		format = func(t *benchtab.Tables) error { return t.ToCSV(w, wErr) }
	}

	stat := benchtab.NewBuilder(tableBy, rowBy, colBy, residue)
	files := benchfmt.Files{Paths: flags.Args(), AllowStdin: true, AllowLabels: true}
	for files.Scan() {
		switch rec := files.Result(); rec := rec.(type) {
		case *benchfmt.SyntaxError:
			// Non-fatal result parse error. Warn
			// but keep going.
			fmt.Fprintln(wErr, rec)
		case *benchfmt.Result:
			if ok, err := filter.Apply(rec); !ok {
				if err != nil {
					// Print the reason we rejected this result.
					fmt.Fprintln(wErr, err)
				}
				continue
			}

			stat.Add(rec)
		}
	}
	if err := files.Err(); err != nil {
		return err
	}

	tables := stat.ToTables(benchtab.TableOpts{
		Confidence: *flagConfidence,
		Thresholds: &thresholds,
		Units:      files.Units(),
	})
	return format(tables)
}
