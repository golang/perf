// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Benchstat computes and compares statistics about benchmarks.
//
// Usage:
//
//	benchstat [-delta-test name] [-geomean] [-html] [-sort order] old.txt [new.txt] [more.txt ...]
//
// Each input file should contain the concatenated output of a number
// of runs of ``go test -bench.'' For each different benchmark listed in an input file,
// benchstat computes the mean, minimum, and maximum run time,
// after removing outliers using the interquartile range rule.
//
// If invoked on a single input file, benchstat prints the per-benchmark statistics
// for that file.
//
// If invoked on a pair of input files, benchstat adds to the output a column
// showing the statistics from the second file and a column showing the
// percent change in mean from the first to the second file.
// Next to the percent change, benchstat shows the p-value and sample
// sizes from a test of the two distributions of benchmark times.
// Small p-values indicate that the two distributions are significantly different.
// If the test indicates that there was no significant change between the two
// benchmarks (defined as p > 0.05), benchstat displays a single ~ instead of
// the percent change.
//
// The -delta-test option controls which significance test is applied:
// utest (Mann-Whitney U-test), ttest (two-sample Welch t-test), or none.
// The default is the U-test, sometimes also referred to as the Wilcoxon rank
// sum test.
//
// If invoked on more than two input files, benchstat prints the per-benchmark
// statistics for all the files, showing one column of statistics for each file,
// with no column for percent change or statistical significance.
//
// The -html option causes benchstat to print the results as an HTML table.
//
// The -sort option specifies an order in which to list the results:
// none (input order), delta (percent improvement), or name (benchmark name).
// A leading “-” prefix, as in “-delta”, reverses the order.
//
// Example
//
// Suppose we collect benchmark results from running ``go test -bench=Encode''
// five times before and after a particular change.
//
// The file old.txt contains:
//
//	BenchmarkGobEncode   	100	  13552735 ns/op	  56.63 MB/s
//	BenchmarkJSONEncode  	 50	  32395067 ns/op	  59.90 MB/s
//	BenchmarkGobEncode   	100	  13553943 ns/op	  56.63 MB/s
//	BenchmarkJSONEncode  	 50	  32334214 ns/op	  60.01 MB/s
//	BenchmarkGobEncode   	100	  13606356 ns/op	  56.41 MB/s
//	BenchmarkJSONEncode  	 50	  31992891 ns/op	  60.65 MB/s
//	BenchmarkGobEncode   	100	  13683198 ns/op	  56.09 MB/s
//	BenchmarkJSONEncode  	 50	  31735022 ns/op	  61.15 MB/s
//
// The file new.txt contains:
//
//	BenchmarkGobEncode   	 100	  11773189 ns/op	  65.19 MB/s
//	BenchmarkJSONEncode  	  50	  32036529 ns/op	  60.57 MB/s
//	BenchmarkGobEncode   	 100	  11942588 ns/op	  64.27 MB/s
//	BenchmarkJSONEncode  	  50	  32156552 ns/op	  60.34 MB/s
//	BenchmarkGobEncode   	 100	  11786159 ns/op	  65.12 MB/s
//	BenchmarkJSONEncode  	  50	  31288355 ns/op	  62.02 MB/s
//	BenchmarkGobEncode   	 100	  11628583 ns/op	  66.00 MB/s
//	BenchmarkJSONEncode  	  50	  31559706 ns/op	  61.49 MB/s
//	BenchmarkGobEncode   	 100	  11815924 ns/op	  64.96 MB/s
//	BenchmarkJSONEncode  	  50	  31765634 ns/op	  61.09 MB/s
//
// The order of the lines in the file does not matter, except that the
// output lists benchmarks in order of appearance.
//
// If run with just one input file, benchstat summarizes that file:
//
//	$ benchstat old.txt
//	name        time/op
//	GobEncode   13.6ms ± 1%
//	JSONEncode  32.1ms ± 1%
//
// If run with two input files, benchstat summarizes and compares:
//
//	$ benchstat old.txt new.txt
//	name        old time/op  new time/op  delta
//	GobEncode   13.6ms ± 1%  11.8ms ± 1%  -13.31% (p=0.016 n=4+5)
//	JSONEncode  32.1ms ± 1%  31.8ms ± 1%     ~    (p=0.286 n=4+5)
//
// Note that the JSONEncode result is reported as
// statistically insignificant instead of a -0.93% delta.
//
// An example benchmarking workflow in Unix shell language:
//
//	oldBin=/tmp/benchmarkBinaryOld
//	newBin=/tmp/benchmarkBinaryNew
//	old=/tmp/benchmarkReportOld
//	new=/tmp/benchmarkReportNew
//	result=/tmp/benchstatReport
//	
//	# Create first test executable.
//	go test -c -o "$oldBin" -bench .
//	
//	# Apply code patch now
//	git checkout fixes
//	
//	# Create the other test executable.
//	go test -c -o "$newBin" -bench .
//	
//	# Test and benchmark.
//	for i in 0 1 2 3 4 5 6 7 8 9 10 11 12 13; do
//		printf 'Tests %s starting.\n' "$i"
//		"$oldBin" -test.bench . >> "$old"
//		"$newBin" -test.bench . >> "$new"
//	done
//	
//	# Create final report with benchstat.
//	benchstat "$old" "$new" > "$result"
//
// Possible variations include disabling tests (done with the command
// line arguments "-run -"), running three instead of two benchmark
// executables in the loop or increasing niceness or, even better,
// running the binaries under a real time scheduling policy (see
// sched_setscheduler and SCHED_FIFO). If you are on Linux and have
// the chrt program, to run the test binary under a real time
// scheduling policy run it like so:
//
//	chrt -f 50 testBinary -test.bench regexp >> out
//
// Be aware, though, that since a real time scheduling policy gives a
// process or thread as much time as it "wants" to take, a thread of
// the running testBinary process or one of its children can take up
// all the time of a CPU core, and thus testBinary and its children
// could, if malicious or simply buggy, effectively make a denial of
// service attack on your computer.
//
// Other general benchmarking tips for reducing noise, Linux specific,
// include disabling address space randomization and disabling Intel
// turbo mode:
//
//	printf 0 > /proc/sys/kernel/randomize_va_space
//	printf 1 > /sys/devices/system/cpu/intel_pstate/no_turbo
//
// If your computer has sufficient cooling, set the Linux "performance"
// frequency scaling governor for all cores:
//
//	for f in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
//		printf performance > "$f"
//	done
//
// If your computer has insufficient cooling, lower the maximum
// frequency of all CPU cores:
//
//	# Get minimum frequency.
//	cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_min_freq
//	
//	for f in /sys/devices/system/cpu/cpu*/cpufreq/scaling_max_freq; do
//		printf minimumFrequency > "$f"
//	done
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/perf/benchstat"
)

var exit = os.Exit // replaced during testing

func usage() {
	fmt.Fprintf(os.Stderr, "usage: benchstat [options] old.txt [new.txt] [more.txt ...]\n")
	fmt.Fprintf(os.Stderr, "options:\n")
	flag.PrintDefaults()
	exit(2)
}

var (
	flagDeltaTest = flag.String("delta-test", "utest", "significance `test` to apply to delta: utest, ttest, or none")
	flagAlpha     = flag.Float64("alpha", 0.05, "consider change significant if p < `α`")
	flagGeomean   = flag.Bool("geomean", false, "print the geometric mean of each file")
	flagHTML      = flag.Bool("html", false, "print results as an HTML table")
	flagCSV       = flag.Bool("csv", false, "print results in CSV form")
	flagNoRange   = flag.Bool("norange", false, "suppress range columns (CSV only)")
	flagSplit     = flag.String("split", "pkg,goos,goarch", "split benchmarks by `labels`")
	flagSort      = flag.String("sort", "none", "sort by `order`: [-]delta, [-]name, none")
)

var deltaTestNames = map[string]benchstat.DeltaTest{
	"none":   benchstat.NoDeltaTest,
	"u":      benchstat.UTest,
	"u-test": benchstat.UTest,
	"utest":  benchstat.UTest,
	"t":      benchstat.TTest,
	"t-test": benchstat.TTest,
	"ttest":  benchstat.TTest,
}

var sortNames = map[string]benchstat.Order{
	"none":  nil,
	"name":  benchstat.ByName,
	"delta": benchstat.ByDelta,
}

func main() {
	log.SetPrefix("benchstat: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	deltaTest := deltaTestNames[strings.ToLower(*flagDeltaTest)]
	sortName := *flagSort
	reverse := false
	if strings.HasPrefix(sortName, "-") {
		reverse = true
		sortName = sortName[1:]
	}
	order, ok := sortNames[sortName]
	if flag.NArg() < 1 || deltaTest == nil || !ok {
		flag.Usage()
	}

	c := &benchstat.Collection{
		Alpha:      *flagAlpha,
		AddGeoMean: *flagGeomean,
		DeltaTest:  deltaTest,
	}
	if *flagSplit != "" {
		c.SplitBy = strings.Split(*flagSplit, ",")
	}
	if order != nil {
		if reverse {
			order = benchstat.Reverse(order)
		}
		c.Order = order
	}
	for _, file := range flag.Args() {
		f, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		if err := c.AddFile(file, f); err != nil {
			log.Fatal(err)
		}
		f.Close()
	}

	tables := c.Tables()
	var buf bytes.Buffer
	if *flagHTML {
		buf.WriteString(htmlHeader)
		benchstat.FormatHTML(&buf, tables)
		buf.WriteString(htmlFooter)
	} else if *flagCSV {
		benchstat.FormatCSV(&buf, tables, *flagNoRange)
	} else {
		benchstat.FormatText(&buf, tables)
	}
	os.Stdout.Write(buf.Bytes())
}

var htmlHeader = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Performance Result Comparison</title>
<style>
.benchstat { border-collapse: collapse; }
.benchstat th:nth-child(1) { text-align: left; }
.benchstat tbody td:nth-child(1n+2):not(.note) { text-align: right; padding: 0em 1em; }
.benchstat tr:not(.configs) th { border-top: 1px solid #666; border-bottom: 1px solid #ccc; }
.benchstat .nodelta { text-align: center !important; }
.benchstat .better td.delta { font-weight: bold; }
.benchstat .worse td.delta { font-weight: bold; color: #c00; }
</style>
</head>
<body>
`
var htmlFooter = `</body>
</html>
`
