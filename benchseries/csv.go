// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchseries

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"

	"golang.org/x/perf/benchmath"
)

type CsvOptions int

const (
	CSV_DISTRIBUTION_BITS CsvOptions = 3
	CSV_PLAIN             CsvOptions = 0
	CSV_DELTA             CsvOptions = 1
	CSV_LOHI              CsvOptions = 2

	CSV_VALUES CsvOptions = 4

	CSV_CHANGE_HEU CsvOptions = 8  // This is the interval-overlap heuristic
	CSV_CHANGE_KS  CsvOptions = 16 // This is a Kolmogorov-Smirnov statistic
)

func (cs *ComparisonSeries) ToCsvBootstrapped(out io.Writer, options CsvOptions, threshold float64) {
	tab, entries := cs.headerAndEntries(options)
	summaries := cs.Summaries
	for i, s := range cs.Series {
		row := []string{s}
		changesHeu := []float64{}
		changesKs := []float64{}
		for j, b := range cs.Benchmarks {
			clear(entries)
			// if _, ok := cs.SummaryAt(b, s); ok {
			_ = b
			if sum := summaries[i][j]; sum.Defined() {
				center, low, high := sum.Center, sum.Low, sum.High

				entries[0] = strof(center)
				entries = entries[:1]

				if i > 0 && summaries[i-1][j].Defined() {
					p := summaries[i-1][j]
					ch := p.HeurOverlap(sum, threshold)
					changesHeu = append(changesHeu, ch)
					cks := p.KSov(sum)
					changesKs = append(changesKs, cks)

					if options&CSV_CHANGE_HEU != 0 {
						if !(math.IsInf(ch, 0) || math.IsNaN(ch)) {
							entries = append(entries, strof(ch))
						} else {
							entries = append(entries, "")
						}
					}
					if options&CSV_CHANGE_KS != 0 {
						if !(math.IsInf(cks, 0) || math.IsNaN(cks)) {
							entries = append(entries, strof(cks))
						} else {
							entries = append(entries, "")
						}
					}
				} else {
					if options&CSV_CHANGE_HEU != 0 {
						entries = append(entries, "")
					}
					if options&CSV_CHANGE_KS != 0 {
						entries = append(entries, "")
					}
				}

				switch options & CSV_DISTRIBUTION_BITS {
				case CSV_PLAIN:
				case CSV_DELTA:
					entries = append(entries, pctof((high-low)/(2*center)))
				case CSV_LOHI:
					entries = append(entries, strof(low), strof(high))
				}
			}
			row = append(row, entries...)
		}
		changes := func(a []float64) []string {
			sort.Float64s(a)
			return []string{strof(percentile(a, .50)), strof(percentile(a, .75)), strof(percentile(a, .90)), strof(percentile(a, 1)), strof(norm(a, 1)), strof(norm(a, 2))}
		}
		if options&CSV_CHANGE_HEU != 0 {
			row = append(row, changes(changesHeu)...)
		}
		if options&CSV_CHANGE_KS != 0 {
			row = append(row, changes(changesKs)...)
		}
		tab = append(tab, row)
	}
	csvw := csv.NewWriter(out)
	csvw.WriteAll(tab)
	csvw.Flush()
}

func (cs *ComparisonSeries) headerAndEntries(options CsvOptions) (tab [][]string, entries []string) {
	entriesLen := 1
	switch options & CSV_DISTRIBUTION_BITS {
	case CSV_PLAIN:
	case CSV_DELTA:
		entriesLen += 1
	case CSV_LOHI:
		entriesLen += 2
	}
	if options&CSV_CHANGE_HEU != 0 {
		entriesLen += 6
	}
	if options&CSV_CHANGE_KS != 0 {
		entriesLen += 6
	}

	entries = make([]string, entriesLen, entriesLen) // ratio,  change, +/- or (lo, hi)

	hdr := []string{cs.Unit}
	for _, b := range cs.Benchmarks {
		hdr = append(hdr, b)
		if options&CSV_CHANGE_HEU != 0 {
			hdr = append(hdr, "change_heur")
		}
		if options&CSV_CHANGE_KS != 0 {
			hdr = append(hdr, "change_ks")
		}
		switch options & CSV_DISTRIBUTION_BITS {
		case CSV_PLAIN:
		case CSV_DELTA:
			hdr = append(hdr, "±")
		case CSV_LOHI:
			hdr = append(hdr, "lo", "hi")
		}
	}

	if options&CSV_CHANGE_HEU != 0 {
		hdr = append(hdr, "change_heur .5", "change_heur .75", "change_heur .9", "change_heur max", "change_heur avg", "change_heur rms")
	}

	if options&CSV_CHANGE_KS != 0 {
		hdr = append(hdr, "change_ks .5", "change_ks .75", "change_ks .9", "change_ks max", "change_ks avg", "change_ks rms")
	}

	tab = [][]string{hdr}
	return
}

func clear(entries []string) {
	for i := range entries {
		entries[i] = ""
	}
}

func percentPlusOrMinus(sum benchmath.Summary) float64 {
	return 100 * (sum.Hi - sum.Lo) / (2 * sum.Center)
}

func strof(x float64) string {
	return fmt.Sprintf("%f", x)
}

func pctof(x float64) string {
	return fmt.Sprintf("%f%%", x)
}
