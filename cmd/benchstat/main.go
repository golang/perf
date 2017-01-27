// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Benchstat computes and compares statistics about benchmarks.
//
// Usage:
//
//	benchstat [-delta-test name] [-geomean] [-html] old.txt [new.txt] [more.txt ...]
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
//	$
//
// If run with two input files, benchstat summarizes and compares:
//
//	$ benchstat old.txt new.txt
//	name        old time/op  new time/op  delta
//	GobEncode   13.6ms ± 1%  11.8ms ± 1%  -13.31% (p=0.016 n=4+5)
//	JSONEncode  32.1ms ± 1%  31.8ms ± 1%     ~    (p=0.286 n=4+5)
//	$
//
// Note that the JSONEncode result is reported as
// statistically insignificant instead of a -0.93% delta.
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/perf/internal/stats"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: benchstat [options] old.txt [new.txt] [more.txt ...]\n")
	fmt.Fprintf(os.Stderr, "options:\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	flagDeltaTest = flag.String("delta-test", "utest", "significance `test` to apply to delta: utest, ttest, or none")
	flagAlpha     = flag.Float64("alpha", 0.05, "consider change significant if p < `α`")
	flagGeomean   = flag.Bool("geomean", false, "print the geometric mean of each file")
	flagHTML      = flag.Bool("html", false, "print results as an HTML table")
)

var deltaTestNames = map[string]func(old, new *Metrics) (float64, error){
	"none":   notest,
	"u":      utest,
	"u-test": utest,
	"utest":  utest,
	"t":      ttest,
	"t-test": ttest,
	"ttest":  ttest,
}

type row struct {
	cols []string
}

func newRow(cols ...string) *row {
	return &row{cols: cols}
}

func (r *row) add(col string) {
	r.cols = append(r.cols, col)
}

func (r *row) trim() {
	for len(r.cols) > 0 && r.cols[len(r.cols)-1] == "" {
		r.cols = r.cols[:len(r.cols)-1]
	}
}

func main() {
	log.SetPrefix("benchstat: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	deltaTest := deltaTestNames[strings.ToLower(*flagDeltaTest)]
	if flag.NArg() < 1 || deltaTest == nil {
		flag.Usage()
	}

	// Read in benchmark data.
	c := readFiles(flag.Args())
	for _, m := range c.Metrics {
		m.ComputeStats()
	}

	var tables [][]*row
	switch len(c.Configs) {
	case 2:
		before, after := c.Configs[0], c.Configs[1]
		key := Key{}
		for _, key.Unit = range c.Units {
			var table []*row
			metric := metricOf(key.Unit)
			for _, key.Benchmark = range c.Benchmarks {
				key.Config = before
				old := c.Metrics[key]
				key.Config = after
				new := c.Metrics[key]
				if old == nil || new == nil {
					continue
				}
				if len(table) == 0 {
					table = append(table, newRow("name", "old "+metric, "new "+metric, "delta"))
				}

				pval, testerr := deltaTest(old, new)

				scaler := newScaler(old.Mean, old.Unit)
				row := newRow(key.Benchmark, old.Format(scaler), new.Format(scaler), "~   ")
				if testerr == stats.ErrZeroVariance {
					row.add("(zero variance)")
				} else if testerr == stats.ErrSampleSize {
					row.add("(too few samples)")
				} else if testerr == stats.ErrSamplesEqual {
					row.add("(all equal)")
				} else if testerr != nil {
					row.add(fmt.Sprintf("(%s)", testerr))
				} else if pval < *flagAlpha {
					row.cols[3] = fmt.Sprintf("%+.2f%%", ((new.Mean/old.Mean)-1.0)*100.0)
				}
				if len(row.cols) == 4 && pval != -1 {
					row.add(fmt.Sprintf("(p=%0.3f n=%d+%d)", pval, len(old.RValues), len(new.RValues)))
				}
				table = append(table, row)
			}
			if len(table) > 0 {
				table = addGeomean(table, c, key.Unit, true)
				tables = append(tables, table)
			}
		}

	default:
		key := Key{}
		for _, key.Unit = range c.Units {
			var table []*row
			metric := metricOf(key.Unit)

			if len(c.Configs) > 1 {
				hdr := newRow("name \\ " + metric)
				for _, config := range c.Configs {
					hdr.add(config)
				}
				table = append(table, hdr)
			} else {
				table = append(table, newRow("name", metric))
			}

			for _, key.Benchmark = range c.Benchmarks {
				row := newRow(key.Benchmark)
				var scaler func(float64) string
				for _, key.Config = range c.Configs {
					m := c.Metrics[key]
					if m == nil {
						row.add("")
						continue
					}
					if scaler == nil {
						scaler = newScaler(m.Mean, m.Unit)
					}
					row.add(m.Format(scaler))
				}
				row.trim()
				if len(row.cols) > 1 {
					table = append(table, row)
				}
			}
			table = addGeomean(table, c, key.Unit, false)
			tables = append(tables, table)
		}
	}

	numColumn := 0
	for _, table := range tables {
		for _, row := range table {
			if numColumn < len(row.cols) {
				numColumn = len(row.cols)
			}
		}
	}

	max := make([]int, numColumn)
	for _, table := range tables {
		for _, row := range table {
			for i, s := range row.cols {
				n := utf8.RuneCountInString(s)
				if max[i] < n {
					max[i] = n
				}
			}
		}
	}

	var buf bytes.Buffer
	for i, table := range tables {
		if i > 0 {
			fmt.Fprintf(&buf, "\n")
		}

		if *flagHTML {
			fmt.Fprintf(&buf, "<style>.benchstat tbody td:nth-child(1n+2) { text-align: right; padding: 0em 1em; }</style>\n")
			fmt.Fprintf(&buf, "<table class='benchstat'>\n")
			printRow := func(row *row, tag string) {
				fmt.Fprintf(&buf, "<tr>")
				for _, cell := range row.cols {
					fmt.Fprintf(&buf, "<%s>%s</%s>", tag, html.EscapeString(cell), tag)
				}
				fmt.Fprintf(&buf, "\n")
			}
			printRow(table[0], "th")
			for _, row := range table[1:] {
				printRow(row, "td")
			}
			fmt.Fprintf(&buf, "</table>\n")
			continue
		}

		// headings
		row := table[0]
		for i, s := range row.cols {
			switch i {
			case 0:
				fmt.Fprintf(&buf, "%-*s", max[i], s)
			default:
				fmt.Fprintf(&buf, "  %-*s", max[i], s)
			case len(row.cols) - 1:
				fmt.Fprintf(&buf, "  %s\n", s)
			}
		}

		// data
		for _, row := range table[1:] {
			for i, s := range row.cols {
				switch i {
				case 0:
					fmt.Fprintf(&buf, "%-*s", max[i], s)
				default:
					if i == len(row.cols)-1 && len(s) > 0 && s[0] == '(' {
						// Left-align p value.
						fmt.Fprintf(&buf, "  %s", s)
						break
					}
					fmt.Fprintf(&buf, "  %*s", max[i], s)
				}
			}
			fmt.Fprintf(&buf, "\n")
		}
	}

	os.Stdout.Write(buf.Bytes())
}

func addGeomean(table []*row, c *Collection, unit string, delta bool) []*row {
	if !*flagGeomean {
		return table
	}

	row := newRow("[Geo mean]")
	key := Key{Unit: unit}
	geomeans := []float64{}
	for _, key.Config = range c.Configs {
		var means []float64
		for _, key.Benchmark = range c.Benchmarks {
			m := c.Metrics[key]
			if m != nil {
				means = append(means, m.Mean)
			}
		}
		if len(means) == 0 {
			row.add("")
			delta = false
		} else {
			geomean := stats.GeoMean(means)
			geomeans = append(geomeans, geomean)
			row.add(newScaler(geomean, unit)(geomean) + "     ")
		}
	}
	if delta {
		row.add(fmt.Sprintf("%+.2f%%", ((geomeans[1]/geomeans[0])-1.0)*100.0))
	}
	return append(table, row)
}

func timeScaler(ns float64) func(float64) string {
	var format string
	var scale float64
	switch x := ns / 1e9; {
	case x >= 99.5:
		format, scale = "%.0fs", 1
	case x >= 9.95:
		format, scale = "%.1fs", 1
	case x >= 0.995:
		format, scale = "%.2fs", 1
	case x >= 0.0995:
		format, scale = "%.0fms", 1000
	case x >= 0.00995:
		format, scale = "%.1fms", 1000
	case x >= 0.000995:
		format, scale = "%.2fms", 1000
	case x >= 0.0000995:
		format, scale = "%.0fµs", 1000*1000
	case x >= 0.00000995:
		format, scale = "%.1fµs", 1000*1000
	case x >= 0.000000995:
		format, scale = "%.2fµs", 1000*1000
	case x >= 0.0000000995:
		format, scale = "%.0fns", 1000*1000*1000
	case x >= 0.00000000995:
		format, scale = "%.1fns", 1000*1000*1000
	default:
		format, scale = "%.2fns", 1000*1000*1000
	}
	return func(ns float64) string {
		return fmt.Sprintf(format, ns/1e9*scale)
	}
}

func newScaler(val float64, unit string) func(float64) string {
	if unit == "ns/op" {
		return timeScaler(val)
	}

	var format string
	var scale float64
	var suffix string

	prescale := 1.0
	if unit == "MB/s" {
		prescale = 1e6
	}

	switch x := val * prescale; {
	case x >= 99500000000000:
		format, scale, suffix = "%.0f", 1e12, "T"
	case x >= 9950000000000:
		format, scale, suffix = "%.1f", 1e12, "T"
	case x >= 995000000000:
		format, scale, suffix = "%.2f", 1e12, "T"
	case x >= 99500000000:
		format, scale, suffix = "%.0f", 1e9, "G"
	case x >= 9950000000:
		format, scale, suffix = "%.1f", 1e9, "G"
	case x >= 995000000:
		format, scale, suffix = "%.2f", 1e9, "G"
	case x >= 99500000:
		format, scale, suffix = "%.0f", 1e6, "M"
	case x >= 9950000:
		format, scale, suffix = "%.1f", 1e6, "M"
	case x >= 995000:
		format, scale, suffix = "%.2f", 1e6, "M"
	case x >= 99500:
		format, scale, suffix = "%.0f", 1e3, "k"
	case x >= 9950:
		format, scale, suffix = "%.1f", 1e3, "k"
	case x >= 995:
		format, scale, suffix = "%.2f", 1e3, "k"
	case x >= 99.5:
		format, scale, suffix = "%.0f", 1, ""
	case x >= 9.95:
		format, scale, suffix = "%.1f", 1, ""
	default:
		format, scale, suffix = "%.2f", 1, ""
	}

	if unit == "B/op" {
		suffix += "B"
	}
	if unit == "MB/s" {
		suffix += "B/s"
	}
	scale /= prescale

	return func(val float64) string {
		return fmt.Sprintf(format+suffix, val/scale)
	}
}

func (m *Metrics) Format(scaler func(float64) string) string {
	diff := 1 - m.Min/m.Mean
	if d := m.Max/m.Mean - 1; d > diff {
		diff = d
	}
	s := scaler(m.Mean)
	if m.Mean == 0 {
		s += "     "
	} else {
		s = fmt.Sprintf("%s ±%3s", s, fmt.Sprintf("%.0f%%", diff*100.0))
	}
	return s
}

// ComputeStats updates the derived statistics in s from the raw
// samples in s.Values.
func (m *Metrics) ComputeStats() {
	// Discard outliers.
	values := stats.Sample{Xs: m.Values}
	q1, q3 := values.Percentile(0.25), values.Percentile(0.75)
	lo, hi := q1-1.5*(q3-q1), q3+1.5*(q3-q1)
	for _, value := range m.Values {
		if lo <= value && value <= hi {
			m.RValues = append(m.RValues, value)
		}
	}

	// Compute statistics of remaining data.
	m.Min, m.Max = stats.Bounds(m.RValues)
	m.Mean = stats.Mean(m.RValues)
}

// A Metrics holds the measurements of a single metric (for example, ns/op or MB/s)
// for all runs of a particular benchmark.
type Metrics struct {
	Unit    string    // unit being measured
	Values  []float64 // measured values
	RValues []float64 // Values with outliers removed
	Min     float64   // min of RValues
	Mean    float64   // mean of RValues
	Max     float64   // max of RValues
}

// A Key identifies one metric (e.g., "ns/op", "B/op") from one
// benchmark (function name sans "Benchmark" prefix) in one
// configuration (input file name).
type Key struct {
	Config, Benchmark, Unit string
}

type Collection struct {
	// Configs, Benchmarks, and Units give the set of configs,
	// benchmarks, and units from the keys in Stats in an order
	// meant to match the order the benchmarks were read in.
	Configs, Benchmarks, Units []string

	// Metrics holds the accumulated metrics for each key.
	Metrics map[Key]*Metrics
}

func (c *Collection) AddStat(key Key) *Metrics {
	if stat, ok := c.Metrics[key]; ok {
		return stat
	}

	addString := func(strings *[]string, add string) {
		for _, s := range *strings {
			if s == add {
				return
			}
		}
		*strings = append(*strings, add)
	}
	addString(&c.Configs, key.Config)
	addString(&c.Benchmarks, key.Benchmark)
	addString(&c.Units, key.Unit)
	m := &Metrics{Unit: key.Unit}
	c.Metrics[key] = m
	return m
}

// readFiles reads a set of benchmark files.
func readFiles(files []string) *Collection {
	c := Collection{Metrics: make(map[Key]*Metrics)}
	for _, file := range files {
		readFile(file, &c)
	}
	return &c
}

// readFile reads a set of benchmarks from a file in to a Collection.
func readFile(file string, c *Collection) {
	c.Configs = append(c.Configs, file)
	key := Key{Config: file}

	text, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	for _, line := range strings.Split(string(text), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		name := f[0]
		if !strings.HasPrefix(name, "Benchmark") {
			continue
		}
		name = strings.TrimPrefix(name, "Benchmark")
		n, _ := strconv.Atoi(f[1])
		if n == 0 {
			continue
		}

		key.Benchmark = name
		for i := 2; i+2 <= len(f); i += 2 {
			val, err := strconv.ParseFloat(f[i], 64)
			if err != nil {
				continue
			}
			key.Unit = f[i+1]
			stat := c.AddStat(key)
			stat.Values = append(stat.Values, val)
		}
	}
}

func metricOf(unit string) string {
	switch unit {
	case "ns/op":
		return "time/op"
	case "B/op":
		return "alloc/op"
	case "MB/s":
		return "speed"
	default:
		return unit
	}
}

// Significance tests.

func notest(old, new *Metrics) (pval float64, err error) {
	return -1, nil
}

func ttest(old, new *Metrics) (pval float64, err error) {
	t, err := stats.TwoSampleWelchTTest(stats.Sample{Xs: old.RValues}, stats.Sample{Xs: new.RValues}, stats.LocationDiffers)
	if err != nil {
		return -1, err
	}
	return t.P, nil
}

func utest(old, new *Metrics) (pval float64, err error) {
	u, err := stats.MannWhitneyUTest(old.RValues, new.RValues, stats.LocationDiffers)
	if err != nil {
		return -1, err
	}
	return u.P, nil
}
