// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Benchstat computes and compares statistics about benchmarks.
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
package benchstat

import (
	"flag"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aclements/go-moremath/stats"
	"golang.org/x/perf/storage/benchfmt"
)

var (
	flagHTML = flag.Bool("html", false, "print results as an HTML table")
)

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

type Options struct {
	// DeltaTest is the significance test to apply to delta.
	// This package provides Utest, Ttest, and Notest. If nil, Utest will be used.
	DeltaTest func(old, new *Benchstat) (pval float64, err error)
	// Alpha is the threshold to consider a change significant if p < α
	Alpha float64
	// Geomean will add a row for the geometric mean of each file
	Geomean bool
	// HTML will print the results as an HTML table
	HTML bool
}

func Run(w io.Writer, results [][]*benchfmt.Result, options *Options) {
	if options == nil {
		options = &Options{}
	}
	if options.DeltaTest == nil {
		options.DeltaTest = Utest
	}
	if options.Alpha == 0 {
		options.Alpha = .05
	}
	// Read in benchmark data.
	c := readResults(results)
	for _, stat := range c.Stats {
		stat.ComputeStats()
	}

	var tables [][]*row
	switch len(c.Configs) {
	case 2:
		before, after := c.Configs[0], c.Configs[1]
		key := BenchKey{}
		for _, key.Unit = range c.Units {
			var table []*row
			metric := metricOf(key.Unit)
			for _, key.Benchmark = range c.Benchmarks {
				key.Config = before
				old := c.Stats[key]
				key.Config = after
				new := c.Stats[key]
				if old == nil || new == nil {
					continue
				}
				if len(table) == 0 {
					table = append(table, newRow("name", "old "+metric, "new "+metric, "delta"))
				}

				pval, testerr := options.DeltaTest(old, new)

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
				} else if pval < options.Alpha {
					row.cols[3] = fmt.Sprintf("%+.2f%%", ((new.Mean/old.Mean)-1.0)*100.0)
				}
				if len(row.cols) == 4 && pval != -1 {
					row.add(fmt.Sprintf("(p=%0.3f n=%d+%d)", pval, len(old.RValues), len(new.RValues)))
				}
				table = append(table, row)
			}
			if len(table) > 0 {
				if options.Geomean {
					table = addGeomean(table, c, key.Unit, true)
				}
				tables = append(tables, table)
			}
		}

	default:
		key := BenchKey{}
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
					stat := c.Stats[key]
					if stat == nil {
						row.add("")
						continue
					}
					if scaler == nil {
						scaler = newScaler(stat.Mean, stat.Unit)
					}
					row.add(stat.Format(scaler))
				}
				row.trim()
				if len(row.cols) > 1 {
					table = append(table, row)
				}
			}
			if options.Geomean {
				table = addGeomean(table, c, key.Unit, false)
			}
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

	for i, table := range tables {
		if i > 0 {
			fmt.Fprintf(w, "\n")
		}

		if options.HTML {
			fmt.Fprintf(w, "<style>.benchstat tbody td:nth-child(1n+2) { text-align: right; padding: 0em 1em; }</style>\n")
			fmt.Fprintf(w, "<table class='benchstat'>\n")
			printRow := func(row *row, tag string) {
				fmt.Fprintf(w, "<tr>")
				for _, cell := range row.cols {
					fmt.Fprintf(w, "<%s>%s</%s>", tag, html.EscapeString(cell), tag)
				}
				fmt.Fprintf(w, "\n")
			}
			printRow(table[0], "th")
			for _, row := range table[1:] {
				printRow(row, "td")
			}
			fmt.Fprintf(w, "</table>\n")
			continue
		}

		// headings
		row := table[0]
		for i, s := range row.cols {
			switch i {
			case 0:
				fmt.Fprintf(w, "%-*s", max[i], s)
			default:
				fmt.Fprintf(w, "  %-*s", max[i], s)
			case len(row.cols) - 1:
				fmt.Fprintf(w, "  %s\n", s)
			}
		}

		// data
		for _, row := range table[1:] {
			for i, s := range row.cols {
				switch i {
				case 0:
					fmt.Fprintf(w, "%-*s", max[i], s)
				default:
					if i == len(row.cols)-1 && len(s) > 0 && s[0] == '(' {
						// Left-align p value.
						fmt.Fprintf(w, "  %s", s)
						break
					}
					fmt.Fprintf(w, "  %*s", max[i], s)
				}
			}
			fmt.Fprintf(w, "\n")
		}
	}
}

func addGeomean(table []*row, c *Collection, unit string, delta bool) []*row {
	row := newRow("[Geo mean]")
	key := BenchKey{Unit: unit}
	geomeans := []float64{}
	for _, key.Config = range c.Configs {
		var means []float64
		for _, key.Benchmark = range c.Benchmarks {
			stat := c.Stats[key]
			if stat != nil {
				means = append(means, stat.Mean)
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

func (b *Benchstat) Format(scaler func(float64) string) string {
	diff := 1 - b.Min/b.Mean
	if d := b.Max/b.Mean - 1; d > diff {
		diff = d
	}
	s := scaler(b.Mean)
	if b.Mean == 0 {
		s += "     "
	} else {
		s = fmt.Sprintf("%s ±%3s", s, fmt.Sprintf("%.0f%%", diff*100.0))
	}
	return s
}

// ComputeStats updates the derived statistics in s from the raw
// samples in s.Values.
func (stat *Benchstat) ComputeStats() {
	// Discard outliers.
	values := stats.Sample{Xs: stat.Values}
	q1, q3 := values.Percentile(0.25), values.Percentile(0.75)
	lo, hi := q1-1.5*(q3-q1), q3+1.5*(q3-q1)
	for _, value := range stat.Values {
		if lo <= value && value <= hi {
			stat.RValues = append(stat.RValues, value)
		}
	}

	// Compute statistics of remaining data.
	stat.Min, stat.Max = stats.Bounds(stat.RValues)
	stat.Mean = stats.Mean(stat.RValues)
}

// A Benchstat is the metrics along one axis (e.g., ns/op or MB/s)
// for all runs of a specific benchmark.
type Benchstat struct {
	Unit    string
	Values  []float64 // metrics
	RValues []float64 // metrics with outliers removed
	Min     float64   // min of RValues
	Mean    float64   // mean of RValues
	Max     float64   // max of RValues
}

// A BenchKey identifies one metric (e.g., "ns/op", "B/op") from one
// benchmark (function name sans "Benchmark" prefix) in one
// configuration (input file name).
type BenchKey struct {
	Config, Benchmark, Unit string
}

type Collection struct {
	Stats map[BenchKey]*Benchstat

	// Configs, Benchmarks, and Units give the set of configs,
	// benchmarks, and units from the keys in Stats in an order
	// meant to match the order the benchmarks were read in.
	Configs, Benchmarks, Units []string
}

func (c *Collection) AddStat(key BenchKey) *Benchstat {
	if stat, ok := c.Stats[key]; ok {
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
	stat := &Benchstat{Unit: key.Unit}
	c.Stats[key] = stat
	return stat
}

// readResults parses all sets of results
func readResults(results [][]*benchfmt.Result) *Collection {
	c := Collection{Stats: make(map[BenchKey]*Benchstat)}
	for _, group := range results {
		readGroup(group, &c)
	}
	return &c
}

// readGroup parses a single set of benchmarks in to a Collection.
func readGroup(group []*benchfmt.Result, c *Collection) {
	// TODO(quentin): Pick a better name for each group
	name := fmt.Sprintf("#%d", len(c.Configs))
	c.Configs = append(c.Configs, name)
	key := BenchKey{Config: name}

	for _, result := range group {
		line := result.Content
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

func Notest(old, new *Benchstat) (pval float64, err error) {
	return -1, nil
}

func Ttest(old, new *Benchstat) (pval float64, err error) {
	t, err := stats.TwoSampleWelchTTest(stats.Sample{Xs: old.RValues}, stats.Sample{Xs: new.RValues}, stats.LocationDiffers)
	if err != nil {
		return -1, err
	}
	return t.P, nil
}

func Utest(old, new *Benchstat) (pval float64, err error) {
	u, err := stats.MannWhitneyUTest(old.RValues, new.RValues, stats.LocationDiffers)
	if err != nil {
		return -1, err
	}
	return u.P, nil
}
