// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchseries

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"time"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchproc"
)

// A Cell is the observations for part of a benchmark comparison.
type Cell struct {
	Values []float64 // Actual values observed for this cell (sorted).  Typically 1-100.

	// Residues is the set of residue Keys mapped to this cell.
	// It is used to check for non-unique keys.
	Residues map[benchproc.Key]struct{}
}

// A Comparison is a pair of numerator and denominator measurements,
// the date that they were collected (or the latest date if they were accumulated),
// an optional slice of medians of ratios of bootstrapped estimates
// and an optional summary node that contains the spreadsheet/json/database
// summary of this same information.
type Comparison struct {
	Numerator, Denominator *Cell
	Date                   string
	ratios                 []float64 // these are from bootstrapping. Typically 1000ish.
	Summary                *ComparisonSummary
}

// A ComparisonSummary is a summary of the comparison of a particular benchmark measurement
// for two different versions of the toolchain.  Low, Center, and High are lower, middle and
// upper estimates of the value, most likely 2.5%ile, 50%ile, and 97.5%ile from a bootstrap
// of the original measurement ratios.  Date is the (latest) date at which the measurements
// were taken.  Present indicates that Low/Center/High/Date are valid; if comparison is non-nil,
// then there is a bootstrap that can be used or was used to initialize the other fields.
// (otherwise the source was JSON or a database).
type ComparisonSummary struct {
	Low        float64     `json:"low"`
	Center     float64     `json:"center"`
	High       float64     `json:"high"`
	Date       string      `json:"date"`
	Present    bool        `json:"present"` // is this initialized?
	comparison *Comparison // backlink for K-S computation, also indicates initialization of L/C/H
}

func (s *ComparisonSummary) Defined() bool {
	return s != nil && s.Present
}

// ComparisonHashes contains the git hashes of the two tool chains being compared.
type ComparisonHashes struct {
	NumHash, DenHash string
}

type StringAndSlice struct {
	S     string   `json:"s"`
	Slice []string `json:"slice"`
}

// A ComparisonSeries describes a table/graph, indexed by paired elements of Benchmarks, Series.
// Summaries contains the points in the graph.
// HashPairs includes annotations for the Series axis.
type ComparisonSeries struct {
	Unit string `json:"unit"`

	Benchmarks []string               `json:"benchmarks"`
	Series     []string               `json:"series"`
	Summaries  [][]*ComparisonSummary `json:"summaries"`

	HashPairs map[string]ComparisonHashes `json:"hashpairs"` // maps a series point to the hashes compared at that point.

	Residues []StringAndSlice `json:"residues"`

	cells map[SeriesKey]*Comparison
}

// SeriesKey is a map key used to index a single cell in a ComparisonSeries.
// ordering is by benchmark, then "series" (== commit) order
type SeriesKey struct {
	Benchmark, Series string
}

// tableKey is a map key used to index a single cell in a lower-t table.
// ordering is by benchmark, then experiment order
type tableKey struct {
	Benchmark, Experiment benchproc.Key
}

type unitTableKey struct {
	unit, table benchproc.Key
}

type table struct {
	cells map[tableKey]*trial

	benchmarks map[benchproc.Key]struct{}
	exps       map[benchproc.Key]struct{}
}

type trial struct {
	baseline           *Cell
	baselineHash       benchproc.Key
	baselineHashString string
	tests              map[benchproc.Key]*Cell // map from test hash id to test information
}

// A Builder collects benchmark results into a set of tables, and transforms that into a slice of ComparisonSeries.
type Builder struct {
	// one table per unit; each table maps from (benchmark,experiment) to a single trial of baseline vs one or more tests
	tables map[unitTableKey]*table

	// numHashBy to numerator order.
	hashToOrder map[benchproc.Key]benchproc.Key

	filter *benchproc.Filter

	unitBy, tableBy, pkgBy, experimentBy, benchBy, seriesBy, compareBy, numHashBy, denHashBy *benchproc.Projection
	denCompareVal                                                                            string // the string value of compareBy that indicates the control/baseline in a comparison.
	numCompareVal                                                                            string // the string value of compareBy that indicates the test in a comparison.
	residue                                                                                  *benchproc.Projection

	unitField *benchproc.Field

	Residues map[benchproc.Key]struct{}

	warn func(format string, args ...interface{})
}

type BuilderOptions struct {
	Filter          string // how to filter benchmark results, as a benchproc option (e.g., ".unit:/.*/")
	Series          string // the name of the benchmark key that contains the time of the last commit to the experiment branch (e.g. "numerator_stamp", "tip-commit-time")
	Table           string // list of benchmark keys to group ComparisonSeries tables by, in addition to .unit (e.g., "goarch,goos", "" (none))
	Experiment      string // the name of the benchmark key that contains the time at which the comparative benchmarks were run (e.g., "upload-time", "runstamp")
	Compare         string // the name of the benchmark key that contains the id/role of the toolchain being compared (e.g., "toolchain", "role")
	Numerator       string // the value of the Compare key that indicates the numerator in the ratios (i.e., "test", "tip", "experiment")
	Denominator     string // the value of the Compare key that indicates the denominator in the ratios (i.e., "control", "base", "baseline")
	NumeratorHash   string // the name of the benchmark key that contains the git hash of the numerator (test) toolchain
	DenominatorHash string // the name of the benchmark key that contains the git hash of the denominator (control) toolchain
	Ignore          string // list of benchmark keys to ignore entirely (e.g. "tip,base,bentstamp,suite")
	Warn            func(format string, args ...interface{})
}

func BentBuilderOptions() *BuilderOptions {
	return &BuilderOptions{
		Filter:          ".unit:/.*/",
		Series:          "numerator_stamp",
		Table:           "goarch,goos,builder_id",
		Experiment:      "runstamp",
		Compare:         "toolchain",
		Numerator:       "Tip",
		Denominator:     "Base",
		NumeratorHash:   "numerator_hash",
		DenominatorHash: "denominator_hash",
		Ignore:          "go,tip,base,bentstamp,suite,cpu,denominator_branch,.fullname,shortname",
		Warn: func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format, args...)
		},
	}
}

func DefaultBuilderOptions() *BuilderOptions {
	return &BuilderOptions{
		Filter:          ".unit:/.*/",
		Series:          "experiment-commit-time",
		Table:           "", // .unit only
		Experiment:      "runstamp",
		Compare:         "toolchain",
		Numerator:       "experiment",
		Denominator:     "baseline",
		NumeratorHash:   "experiment-commit",
		DenominatorHash: "baseline-commit",
		Ignore:          "go,tip,base,bentstamp,shortname,suite",
		Warn: func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format, args...)
		},
	}
}

var noPuncDate = regexp.MustCompile("^[0-9]{8}T[0-9]{6}$")

// RFC3339NanoNoZ has the property that formatted date&time.000000000 < date&time.000000001,
// unlike RFC3339Nano where date&timeZ > date&timeZ.000000001Z
// i.e., "Z" > "."" but "+" < "." so if ".000000000" is elided must use "+00:00"
// to express the Z time zone to get the sort right.
const RFC3339NanoNoZ = "2006-01-02T15:04:05.999999999-07:00"

// NormalizeDateString converts dates in two formats used in bent/benchmarking
// into UTC, so that all sort properly into a single order with no confusion.
func NormalizeDateString(in string) (string, error) {
	if noPuncDate.MatchString(in) {
		//20211229T213212
		//2021-12-29T21:32:12
		in = in[0:4] + "-" + in[4:6] + "-" + in[6:11] + ":" + in[11:13] + ":" + in[13:15] + "+00:00"
	}
	t, err := time.Parse(time.RFC3339Nano, in)
	if err != nil {
		return "", err
	}
	return t.UTC().Format(RFC3339NanoNoZ), nil
}

// ParseNormalizedDateString parses a time in the format returned by
// NormalizeDateString.
func ParseNormalizedDateString(in string) (time.Time, error) {
	return time.Parse(RFC3339NanoNoZ, in)
}

// NewBuilder creates a new Builder for collecting benchmark results
// into tables. Each result will be mapped to a Table by seriesBy.
// Within each table, the results are mapped to cells by benchBy and
// seriesBy. Any results within a single cell that vary by residue will
// be reported as warnings.
func NewBuilder(bo *BuilderOptions) (*Builder, error) {

	filter, err := benchproc.NewFilter(bo.Filter)
	if err != nil {
		return nil, fmt.Errorf("parsing -filter: %s", err)
	}

	var parserErr error
	var parser benchproc.ProjectionParser
	mustParse := func(name, val string) *benchproc.Projection {
		schema, err := parser.Parse(val, filter)
		if err != nil {
			parserErr = fmt.Errorf("parsing %s: %s", name, err)
		}
		return schema
	}

	unitBy, unitField, err := parser.ParseWithUnit("", nil)
	if err != nil {
		panic("Couldn't parse the unit schema")
	}

	tableBy, err := parser.Parse(bo.Table, nil)
	if err != nil {
		panic("Couldn't parse the table schema")
	}

	benchBy, err := parser.Parse(".fullname", nil)
	if err != nil {
		panic("Couldn't parse the .name schema")
	}

	pkgBy, err := parser.Parse("pkg", nil)
	if err != nil {
		panic("Couldn't parse 'pkg' schema")
	}

	seriesBy := mustParse("-series", bo.Series)
	experimentBy := mustParse("-experiment", bo.Experiment)
	compareBy := mustParse("-compare", bo.Compare)
	numHashBy := mustParse("-numerator-hash", bo.NumeratorHash)
	denHashBy := mustParse("-denominator-hash", bo.DenominatorHash)

	mustParse("-ignore", bo.Ignore)

	if parserErr != nil {
		return nil, parserErr
	}

	residue := parser.Residue()

	return &Builder{
		filter:        filter,
		unitBy:        unitBy,
		tableBy:       tableBy,
		pkgBy:         pkgBy,
		experimentBy:  experimentBy,
		benchBy:       benchBy,
		seriesBy:      seriesBy,
		compareBy:     compareBy,
		numHashBy:     numHashBy,
		denHashBy:     denHashBy,
		denCompareVal: bo.Denominator,
		numCompareVal: bo.Numerator,
		residue:       residue,
		unitField:     unitField,
		hashToOrder:   make(map[benchproc.Key]benchproc.Key),
		tables:        make(map[unitTableKey]*table),
		Residues:      make(map[benchproc.Key]struct{}),
		warn:          bo.Warn,
	}, nil
}

func (b *Builder) AddFiles(files benchfmt.Files) error {
	for files.Scan() {
		rec := files.Result()
		if err, ok := rec.(*benchfmt.SyntaxError); ok {
			// Non-fatal result parse error. Warn
			// but keep going.
			b.warn("%v\n", err)
			continue
		}
		res := rec.(*benchfmt.Result)

		b.Add(res)
	}
	if err := files.Err(); err != nil {
		return err
	}
	return nil
}

// Add adds all of the values in result to the tables in the Builder.
func (b *Builder) Add(result *benchfmt.Result) {
	if ok, _ := b.filter.Apply(result); !ok {
		return
	}

	// Project the result.
	unitCfgs := b.unitBy.ProjectValues(result)
	tableCfg := b.tableBy.Project(result)

	_ = b.pkgBy.Project(result) // for now we are dropping pkg on the floor

	expCfg := b.experimentBy.Project(result)
	benchCfg := b.benchBy.Project(result)
	serCfg := b.seriesBy.Project(result)
	cmpCfg := b.compareBy.Project(result)
	numHashCfg := b.numHashBy.Project(result)
	denHashCfg := b.denHashBy.Project(result)

	// tableBy, experimentBy, benchBy, seriesBy, compareBy, numHashBy, denHashBy

	residueCfg := b.residue.Project(result)
	cellCfg := tableKey{Benchmark: benchCfg, Experiment: expCfg}

	// Map to tables.
	for unitI, unitCfg := range unitCfgs {
		tuk := unitTableKey{unitCfg, tableCfg}
		table := b.tables[tuk]
		if table == nil {
			table = b.newTable()
			b.tables[tuk] = table
		}

		// Map to a trial.
		t := table.cells[cellCfg]
		if t == nil {
			t = new(trial)
			table.cells[cellCfg] = t
			t.tests = make(map[benchproc.Key]*Cell)

			table.exps[expCfg] = struct{}{}
			table.benchmarks[benchCfg] = struct{}{}

		}

		var c *Cell
		newCell := func() *Cell {
			return &Cell{Residues: make(map[benchproc.Key]struct{})}
		}
		if cmpCfg.StringValues() == b.denCompareVal {
			c = t.baseline
			if c == nil {
				c = newCell()
				t.baseline = c
				t.baselineHash = denHashCfg
				t.baselineHashString = denHashCfg.StringValues()
			}
		} else {
			c = t.tests[numHashCfg]
			if c == nil {
				c = newCell()
				t.tests[numHashCfg] = c
				b.hashToOrder[numHashCfg] = serCfg
			}
		}

		// Add to the cell.
		c.Values = append(c.Values, result.Values[unitI].Value)
		c.Residues[residueCfg] = struct{}{}
		b.Residues[residueCfg] = struct{}{}
	}
}

func (b *Builder) newTable() *table {
	return &table{
		benchmarks: make(map[benchproc.Key]struct{}),
		exps:       make(map[benchproc.Key]struct{}),
		cells:      make(map[tableKey]*trial),
	}
}

// union combines two sets of benchproc.Key into one.
func union(a, b map[benchproc.Key]struct{}) map[benchproc.Key]struct{} {
	if len(b) < len(a) {
		a, b = b, a
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			// a member of the not-larger set was not present in the larger set
			c := make(map[benchproc.Key]struct{})
			for k := range a {
				c[k] = struct{}{}
			}
			for k := range b {
				c[k] = struct{}{}
			}
			return c
		}
	}
	return b
}

func concat(a, b []float64) []float64 {
	return append(append([]float64{}, a...), b...)
}

const (
	DUPE_REPLACE = iota
	DUPE_COMBINE
	// TODO DUPE_REPEAT
)

// AllComparisonSeries converts the accumulated "experiments" into a slice of series of comparisons,
// with one slice element per goos-goarch-unit.  The experiments need not have occurred in any
// sensible order; this deals with that, including overlaps (depend on flag, either replaces old with
// younger or combines, REPLACE IS PREFERRED and works properly with combining old summary data with
// fresh benchmarking data) and possibly also with previously processed summaries.
func (b *Builder) AllComparisonSeries(existing []*ComparisonSeries, dupeHow int) ([]*ComparisonSeries, error) {
	old := make(map[string]*ComparisonSeries)
	for _, cs := range existing {
		old[cs.Unit] = cs
	}
	var css []*ComparisonSeries

	// Iterate over units.
	for _, u := range sortTableKeys(b.tables) {
		t := b.tables[u]
		uString := u.unit.StringValues()
		if ts := u.table.StringValues(); ts != "" {
			uString += " " + u.table.StringValues()
		}
		var cs *ComparisonSeries

		sers := make(map[string]struct{})
		benches := make(map[string]struct{})

		if o := old[uString]; o != nil {
			cs = o
			delete(old, uString)

			cs.cells = make(map[SeriesKey]*Comparison)
			for i, s := range cs.Series {
				for j, b := range cs.Benchmarks {
					if cs.Summaries[i][j].Defined() {
						sk := SeriesKey{
							Benchmark: b,
							Series:    s,
						}
						benches[b] = struct{}{}
						sers[s] = struct{}{}
						sum := cs.Summaries[i][j]
						cc := &Comparison{Summary: sum, Date: sum.Date}
						sum.comparison = cc
						cs.cells[sk] = cc
					}
				}
			}

		} else {
			cs = &ComparisonSeries{Unit: uString,
				HashPairs: make(map[string]ComparisonHashes),
				cells:     make(map[SeriesKey]*Comparison),
			}
		}

		// TODO not handling overlapping samples between "existing" and "newly read" yet.

		// Rearrange into paired comparisons, gathering repeats of same comparison from multiple experiments.
		for tk, tr := range t.cells {
			// tk == bench, experiment, tr == baseline, tests, tests == map hash -> cell.
			bench := tk.Benchmark
			dateString, err := NormalizeDateString(tk.Experiment.StringValues())
			if err != nil {
				return nil, fmt.Errorf("error parsing experiment date %q: %w", tk.Experiment.StringValues(), err)
			}
			benchString := bench.StringValues()
			benches[benchString] = struct{}{}
			for hash, cell := range tr.tests {
				hashString := hash.StringValues()
				ser := b.hashToOrder[hash]
				serString, err := NormalizeDateString(ser.StringValues())
				if err != nil {
					return nil, fmt.Errorf("error parsing series date %q: %w", ser.StringValues(), err)
				}
				sers[serString] = struct{}{}
				sk := SeriesKey{
					Benchmark: benchString,
					Series:    serString,
				}
				cc := cs.cells[sk]
				if cc == nil || dupeHow == DUPE_REPLACE {
					if cc == nil || cc.Date < dateString {
						cc = &Comparison{
							Numerator:   cell,
							Denominator: tr.baseline,
							Date:        dateString,
						}
						cs.cells[sk] = cc
					}

					hp, ok := cs.HashPairs[serString]
					if !ok {
						cs.HashPairs[serString] = ComparisonHashes{NumHash: hashString, DenHash: tr.baselineHashString}
					} else {
						if hp.NumHash != hashString || hp.DenHash != tr.baselineHashString {
							fmt.Fprintf(os.Stderr, "numerator/denominator mismatch, expected %s/%s got %s/%s\n",
								hp.NumHash, hp.DenHash, hashString, tr.baselineHashString)
						}
					}

				} else { // Current augments, but this will do the wrong thing if one is an old summary; also need to think about "repeat"
					// augment an existing measurement (i.e., a second experiment on this same datapoint)
					// fmt.Printf("Augment u:%s,b:%s,ch:%s,cd:%s; cc=%v[n(%d+%d)d(%d+%d)]\n",
					// 	u.StringValues(), bench.StringValues(), hash.StringValues(), ser.StringValues(),
					// 	cc, len(cc.Numerator.Values), len(cell.Values), len(cc.Denominator.Values), len(tr.baseline.Values))
					cc.Numerator = &Cell{
						Values:   concat(cc.Numerator.Values, cell.Values),
						Residues: union(cc.Numerator.Residues, cell.Residues),
					}
					cc.Denominator = &Cell{
						Values:   concat(cc.Denominator.Values, tr.baseline.Values),
						Residues: union(cc.Denominator.Residues, tr.baseline.Residues),
					}
					if cc.Date < dateString {
						cc.Date = dateString
					}
				}
			}
		}

		cs.Benchmarks = sortStringSet(benches)
		cs.Series = sortStringSet(sers)
		for _, b := range cs.Benchmarks {
			for _, s := range cs.Series {
				cc := cs.cells[SeriesKey{Benchmark: b, Series: s}]
				if cc != nil && cc.Numerator != nil && cc.Denominator != nil {
					sort.Float64s(cc.Numerator.Values)
					sort.Float64s(cc.Denominator.Values)
				}
			}
		}

		// Accumulate residues for this unit's table
		type seenKey struct {
			f *benchproc.Field
			s string
		}

		seen := make(map[seenKey]bool)
		rmap := make(map[string][]string)

		for _, c := range cs.cells {
			for _, f := range b.residue.FlattenedFields() {
				if c.Numerator == nil {
					continue
				}
				for k, _ := range c.Numerator.Residues {
					s := k.Get(f)
					if !seen[seenKey{f, s}] {
						seen[seenKey{f, s}] = true
						rmap[f.Name] = append(rmap[f.Name], s)
					}
				}
				for k, _ := range c.Denominator.Residues {
					s := k.Get(f)
					if !seen[seenKey{f, s}] {
						seen[seenKey{f, s}] = true
						rmap[f.Name] = append(rmap[f.Name], s)
					}
				}
			}
		}

		sas := []StringAndSlice{}
		for k, v := range rmap {
			sort.Strings(v)
			sas = append(sas, StringAndSlice{k, v})
		}
		sort.Slice(sas, func(i, j int) bool { return sas[i].S < sas[j].S })

		if len(cs.Residues) > 0 {
			// Need to merge old and new
			osas, nsas := cs.Residues, []StringAndSlice{}
			for i, j := 0, 0; i < len(sas) || j < len(osas); {
				if i == len(sas) || j < len(osas) && osas[j].S < sas[i].S {
					nsas = append(nsas, osas[j])
					j++
					continue
				}
				if j == len(osas) || osas[j].S > sas[i].S {
					nsas = append(nsas, sas[i])
					i++
					continue
				}

				// S (keys) are equal, merge value slices
				sl, osl, nsl := sas[i].Slice, osas[j].Slice, []string{}
				for ii, jj := 0, 0; ii < len(sl) || jj < len(osl); {
					if ii == len(sl) || jj < len(osl) && osl[jj] < sl[ii] {
						nsl = append(nsl, osl[jj])
						jj++
						continue
					}
					if jj == len(osl) || osl[jj] > sl[ii] {
						nsl = append(nsl, sl[ii])
						ii++
						continue
					}
					nsl = append(nsl, sl[ii])
					ii++
					jj++
				}
				nsas = append(nsas, StringAndSlice{sas[i].S, nsl})
				i++
				j++
			}
			sas = nsas
		}

		cs.Residues = sas

		css = append(css, cs)
	}

	for _, cs := range existing {
		if o := old[cs.Unit]; o != nil {
			css = append(css, cs)
		}
	}

	return css, nil
}

func sortStringSet(m map[string]struct{}) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

func sortTableKeys(m map[unitTableKey]*table) []unitTableKey {
	var s []unitTableKey
	for k := range m {
		s = append(s, k)
	}
	sort.Slice(s, func(i, j int) bool {
		if s[i].unit != s[j].unit {
			return s[i].unit.StringValues() < s[j].unit.StringValues()
		}
		if s[i].table == s[j].table {
			return false
		}
		return s[i].table.StringValues() < s[j].table.StringValues()

	})
	return s
}

func absSortedPermFor(a []float64) []int {
	p := make([]int, len(a), len(a))
	for i := range p {
		p[i] = i
	}
	sort.Slice(p, func(i, j int) bool {
		return math.Abs(a[p[i]]) < math.Abs(a[p[j]])
	})
	return p
}

func permute(a []float64, p []int) []float64 {
	b := make([]float64, len(a), len(a))
	for i, j := range p {
		b[i] = a[j]
	}
	return b
}

// TODO Does this need to export the individual cells? What's the expected/intended use?

func (cs *ComparisonSeries) ComparisonAt(benchmark, series string) (*Comparison, bool) {
	if cc := cs.cells[SeriesKey{Benchmark: benchmark, Series: series}]; cc != nil {
		return cc, true
	}
	return nil, false
}

func (cs *ComparisonSeries) SummaryAt(benchmark, series string) (*ComparisonSummary, bool) {
	if cc := cs.cells[SeriesKey{Benchmark: benchmark, Series: series}]; cc != nil {
		return cc.Summary, true
	}
	return nil, false
}

func (c *Cell) resampleInto(r *rand.Rand, x []float64) {
	l := len(x)
	for i := range x {
		x[i] = c.Values[r.Intn(l)]
	}
	sort.Float64s(x)
}

const rot = 23

func (c *Cell) hash() int64 {
	var x int64
	for _, v := range c.Values {
		xlow := (x >> (64 - rot)) & (1<<rot - 1)
		x = (x << rot) ^ xlow ^ int64(math.Float64bits(v))
	}
	return x
}

// ratio computes a bootstrapped estimate of the confidence interval for
// the ratio of measurements in nu divided by measurements in de.
func ratio(nu, de *Cell, confidence float64, r *rand.Rand, ratios []float64) (center, low, high float64) {
	N := len(ratios)
	rnu := make([]float64, len(nu.Values), len(nu.Values))
	rde := make([]float64, len(de.Values), len(de.Values))
	for i := 0; i < N; i++ {
		nu.resampleInto(r, rnu)
		de.resampleInto(r, rde)
		den := median(rde)
		if den == 0 {
			num := median(rnu)
			if num >= 0 {
				ratios[i] = (num + 1)
			} else {
				ratios[i] = (num - 1)
			}
		} else {
			ratios[i] = median(rnu) / den
		}
	}
	sort.Float64s(ratios)
	p := (1 - confidence) / 2
	low = percentile(ratios, p)
	high = percentile(ratios, 1-p)
	center = median(ratios)
	return
}

func percentile(a []float64, p float64) float64 {
	if len(a) == 0 {
		return math.NaN()
	}
	if p == 0 {
		return a[0]
	}
	n := len(a)
	if p == 1 {
		return a[n-1]
	}
	f := float64(float64(n) * p) // Suppress fused-multiply-add
	i := int(f)
	x := f - float64(i)
	r := a[i]
	if x > 0 && i+1 < len(a) {
		r = float64(r*(1-x)) + float64(a[i+1]*x) // Suppress fused-multiply-add
	}
	return r
}

func median(a []float64) float64 {
	l := len(a)
	if l&1 == 1 {
		return a[l/2]
	}
	return (a[l/2] + a[l/2-1]) / 2
}

func norm(a []float64, l float64) float64 {
	if len(a) == 0 {
		return math.NaN()
	}
	n := 0.0
	sum := 0.0
	for _, x := range a {
		if math.IsInf(x, 0) || math.IsNaN(x) {
			continue
		}
		sum += math.Pow(math.Abs(x), l)
		n++
	}
	return math.Pow(sum/n, 1/l)
}

// ChangeScore returns an indicator of the change and direction.
// This is a heuristic measure of the lack of overlap between
// two confidence intervals; minimum lack of overlap (i.e., same
// confidence intervals) is zero.  Exact non-overlap, meaning
// the high end of one interval is equal to the low end of the
// other, is one.  A gap of size G between the two intervals
// yields a score of 1 + G/M where M is the size of the smaller
// interval (this penalizes a ChangeScore in noise, which is also a
// ChangeScore). A partial overlap of size G yields a score of
// 1 - G/M.
//
// Empty confidence intervals are problematic and produces infinities
// or NaNs.
func ChangeScore(l1, c1, h1, l2, c2, h2 float64) float64 {
	sign := 1.0
	if c1 > c2 {
		l1, c1, h1, l2, c2, h2 = l2, c2, h2, l1, c1, h1
		sign = -sign
	}
	r := math.Min(h1-l1, h2-l2)
	// we know l1 < c1 < h1, c1 < c2, l2 < c2 < h2
	// therefore l1 < c1 < c2 < h2
	if h1 > l2 { // overlap
		if h1 > h2 {
			h1 = h2
		}
		if l2 < l1 {
			l2 = l1
		}
		return sign * (1 - (h1-l2)/r) // perfect overlap == 0
	} else { // no overlap
		return sign * (1 + (l2-h1)/r) //
	}
}

type compareFn func(c *Comparison) (center, low, high float64)

func withBootstrap(confidence float64, N int) compareFn {
	return func(c *Comparison) (center, low, high float64) {
		c.ratios = make([]float64, N, N)
		r := rand.New(rand.NewSource(c.Numerator.hash() * c.Denominator.hash()))
		center, low, high = ratio(c.Numerator, c.Denominator, confidence, r, c.ratios)
		return
	}
}

// KSov returns the size-adjusted Kolmogorov-Smirnov statistic,
// equal to D_{n,m} / sqrt((n+m)/n*m).  The result can be compared
// to c(α) where α is the level at which the null hypothesis is rejected.
//
//	   α:  0.2   0.15  0.10  0.05  0.025 0.01  0.005 0.001
//	c(α):  1.073 1.138 1.224 1.358 1.48  1.628 1.731 1.949
//
// see
// https://en.wikipedia.org/wiki/Kolmogorov%E2%80%93Smirnov_test#Two-sample_Kolmogorov%E2%80%93Smirnov_test
func (a *ComparisonSummary) KSov(b *ComparisonSummary) float64 {
	// TODO Kolmogorov-Smirnov hasn't worked that well
	ra, rb := a.comparison.ratios, b.comparison.ratios
	ia, ib := 0, 0
	la, lb := len(ra), len(rb)
	fla, flb := float64(la), float64(lb)

	gap := 0.0

	for ia < la && ib < lb {
		if ra[ia] < rb[ib] {
			ia++
		} else if ra[ia] > rb[ib] {
			ib++
		} else {
			ia++
			ib++
		}
		g := math.Abs(float64(ia)/fla - float64(ib)/flb)
		if g > gap {
			gap = g
		}
	}
	return gap * math.Sqrt(fla*flb/(fla+flb))
}

// HeurOverlap computes a heuristic overlap between two confidence intervals
func (a *ComparisonSummary) HeurOverlap(b *ComparisonSummary, threshold float64) float64 {
	if a.Low == a.High && b.Low == b.High {
		ca, cb, sign := a.Center, b.Center, 100.0
		if cb < ca {
			ca, cb, sign = cb, ca, -100.0
		}
		if ca == 0 {
			if cb > threshold {
				return sign
			}
		} else if (cb-ca)/ca > threshold {
			return sign
		}
		return 0
	}
	return ChangeScore(a.Low, a.Center, a.High, b.Low, b.Center, b.High)
}

// AddSumaries computes the summary data (bootstrapped estimated of the specified
// confidence interval) for the comparison series cs.  The 3rd parameter N specifies
// the number of sampled bootstraps to use; 1000 is recommended, but 500 is good enough
// for testing.
func (cs *ComparisonSeries) AddSummaries(confidence float64, N int) {
	fn := withBootstrap(confidence, N)
	var tab [][]*ComparisonSummary
	for _, s := range cs.Series {
		row := []*ComparisonSummary{}
		for _, b := range cs.Benchmarks {
			if c, ok := cs.ComparisonAt(b, s); ok {
				sum := c.Summary
				if sum == nil || (!sum.Present && sum.comparison == nil) {
					sum = &ComparisonSummary{comparison: c, Date: c.Date}
					sum.Center, sum.Low, sum.High = fn(c)
					sum.Present = true
					c.Summary = sum
				}
				row = append(row, sum)
			} else {
				row = append(row, &ComparisonSummary{})
			}
		}
		tab = append(tab, row)
	}
	cs.Summaries = tab
}
