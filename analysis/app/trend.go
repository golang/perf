// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Loosely based on github.com/aclements/go-misc/benchplot

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/ggstat"
	"github.com/aclements/go-gg/table"
	"golang.org/x/net/context"
	"golang.org/x/perf/storage"
)

// trend handles /trend.
// With no query, it prints the list of recent uploads containg a "trend" key.
// With a query, it shows a graph of the matching benchmark results.
func (a *App) trend(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")

	tmpl, err := ioutil.ReadFile("template/trend.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	t, err := template.New("main").Parse(string(tmpl))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	data := a.trendQuery(ctx, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

// trendData is the struct passed to the trend.html template.
type trendData struct {
	Q            string
	Error        string
	TrendUploads []storage.UploadInfo
	PlotData     template.JS
}

// trendData computes the values for the template and returns a trendData for display.
func (a *App) trendQuery(ctx context.Context, q string) *trendData {
	d := &trendData{Q: q}
	if q == "" {
		ul := a.StorageClient.ListUploads(`trend>`, []string{"by", "upload-time", "trend"}, 16)
		defer ul.Close()
		for ul.Next() {
			d.TrendUploads = append(d.TrendUploads, ul.Info())
		}
		if err := ul.Err(); err != nil {
			errorf(ctx, "failed to fetch recent trend uploads: %v", err)
		}
		return d
	}

	// TODO(quentin): Chunk query based on matching upload IDs.
	res := a.StorageClient.Query(q)
	defer res.Close()
	t, resultCols := queryToTable(res)
	for _, col := range []string{"commit", "commit-time", "branch"} {
		if t.Column(col) == nil {
			d.Error = fmt.Sprintf("results missing %q label", col)
			return d
		}
	}
	data := plot(t, resultCols)

	// TODO(quentin): Give the user control over across vs. plotting in separate graphs, instead of only showing one graph with ns/op for each benchmark.

	// Pivot all of the benchmarks into columns of a single table.
	ar := &aggResults{
		Across: "name",
		Values: []string{"filtered normalized mean result", "normalized mean result", "normalized min result", "normalized max result"},
	}
	data = ggstat.Agg("commit", "branch", "commit-index")(ar.agg).F(data)

	tables := data.Tables()
	infof(ctx, "tables: %v", tables)
	columns := []column{{Name: "commit-index"}}
	for _, prefix := range ar.Prefixes {
		if len(ar.Prefixes) == 1 {
			columns = append(columns,
				column{Name: prefix + "/normalized mean result"},
				column{Name: prefix + "/normalized min result", Role: "interval"},
				column{Name: prefix + "/normalized max result", Role: "interval"},
			)
		}
		columns = append(columns,
			column{Name: prefix + "/filtered normalized mean result"},
		)
	}
	d.PlotData = tableToJS(data.Table(tables[0]), columns)
	return d
}

// queryToTable converts the result of a Query into a Table for later processing.
// Each label is placed in a column named after the key.
// Each metric is placed in a separate result column named after the unit.
func queryToTable(q *storage.Query) (t *table.Table, resultCols []string) {
	var names []string
	labels := make(map[string][]string)
	results := make(map[string][]float64)
	i := 0
	for q.Next() {
		res := q.Result()
		// TODO(quentin): Handle multiple results with the same name but different NameLabels.
		names = append(names, res.NameLabels["name"])
		for k := range res.Labels {
			if labels[k] == nil {
				labels[k] = make([]string, i)
			}
		}
		for k := range labels {
			labels[k] = append(labels[k], res.Labels[k])
		}
		f := strings.Fields(res.Content)
		metrics := make(map[string]float64)
		for j := 2; j+2 <= len(f); j += 2 {
			val, err := strconv.ParseFloat(f[j], 64)
			if err != nil {
				continue
			}
			unit := f[j+1]
			if results[unit] == nil {
				results[unit] = make([]float64, i)
			}
			metrics[unit] = val
		}
		for k := range results {
			results[k] = append(results[k], metrics[k])
		}
		i++
	}

	tab := new(table.Builder).Add("name", names)

	for k, v := range labels {
		tab.Add(k, v)
	}
	for k, v := range results {
		tab.Add(k, v)
		resultCols = append(resultCols, k)
	}

	sort.Strings(resultCols)

	return tab.Done(), resultCols
}

// plot takes raw benchmark data in t and produces a Grouping object containing filtered, normalized metric results for a graph.
// t must contain the string columns "commit", "commit-time", "branch". resultCols specifies the names of float64 columns containing metric results.
// The returned grouping has columns "commit", "commit-time", "commit-index", "branch", "metric", "normalized min result", "normalized max result", "normalized mean result", "filtered normalized mean result".
// This is roughly the algorithm from github.com/aclements/go-misc/benchplot
func plot(t table.Grouping, resultCols []string) table.Grouping {
	nrows := len(table.GroupBy(t, "name").Tables())

	// Turn ordered commit-time into a "commit-index" column.
	t = table.SortBy(t, "commit-time", "commit")
	t = commitIndex{}.F(t)

	// Unpivot all of the metrics into one column.
	t = table.Unpivot(t, "metric", "result", resultCols...)

	// TODO(quentin): Let user choose which metric(s) to keep.
	t = table.FilterEq(t, "metric", "ns/op")

	// Average each result at each commit (but keep columns names
	// the same to keep things easier to read).
	t = ggstat.Agg("commit", "name", "metric", "branch", "commit-index")(ggstat.AggMean("result"), ggstat.AggMin("result"), ggstat.AggMax("result")).F(t)
	y := "mean result"

	// Normalize to earliest commit on master. It's important to
	// do this before the geomean if there are commits missing.
	// Unfortunately, that also means we have to *temporarily*
	// group by name and metric, since the geomean needs to be
	// done on a different grouping.
	t = table.GroupBy(t, "name", "metric")
	t = ggstat.Normalize{X: "branch", By: firstMasterIndex, Cols: []string{"mean result", "max result", "min result"}, DenomCols: []string{"mean result", "mean result", "mean result"}}.F(t)
	y = "normalized " + y
	for _, col := range []string{"mean result", "max result", "min result"} {
		t = table.Remove(t, col)
	}
	t = table.Ungroup(table.Ungroup(t))

	// Compute geomean for each metric at each commit if there's
	// more than one benchmark.
	if len(table.GroupBy(t, "name").Tables()) > 1 {
		gt := removeNaNs(t, y)
		gt = ggstat.Agg("commit", "metric", "branch", "commit-index")(ggstat.AggGeoMean(y), ggstat.AggMin("normalized min result"), ggstat.AggMax("normalized max result")).F(gt)
		gt = table.MapTables(gt, func(_ table.GroupID, t *table.Table) *table.Table {
			return table.NewBuilder(t).AddConst("name", " geomean").Done()
		})
		gt = table.Rename(gt, "geomean "+y, y)
		gt = table.Rename(gt, "min normalized min result", "normalized min result")
		gt = table.Rename(gt, "max normalized max result", "normalized max result")
		t = table.Concat(t, gt)
		nrows++
	}

	// Filter the data to reduce noise.
	t = table.GroupBy(t, "name", "metric")
	t = kza{y, 15, 3}.F(t)
	y = "filtered " + y
	t = table.Ungroup(table.Ungroup(t))

	return t
}

// aggResults pivots the table, taking the columns in Values and making a new column for each distinct value in Across.
// aggResults("in", []string{"value1", "value2"} will reshape a table like
//   in		value1	value2
//   one	1	2
//   two	3	4
// and will turn in into a table like
//   one/value1	one/value2	two/value1	two/value2
//   1		2		3		4
// across columns must be []string, and value columns must be []float64.
type aggResults struct {
	// Across is the name of the column whose values are the column prefix.
	Across string
	// Values is the name of the columns to split.
	Values []string
	// Prefixes is filled in after calling agg with the name of each prefix that was found.
	Prefixes []string
}

// agg implements ggstat.Aggregator and allows using a with ggstat.Agg.
func (a *aggResults) agg(input table.Grouping, output *table.Builder) {
	var prefixes []string
	rows := len(input.Tables())
	columns := make(map[string][]float64)
	for i, gid := range input.Tables() {
		var vs [][]float64
		for _, col := range a.Values {
			vs = append(vs, input.Table(gid).MustColumn(col).([]float64))
		}
		as := input.Table(gid).MustColumn(a.Across).([]string)
		for j, prefix := range as {
			for k, col := range a.Values {
				key := prefix + "/" + col
				if columns[key] == nil {
					if k == 0 {
						// First time we've seen this prefix, track it.
						prefixes = append(prefixes, prefix)
					}
					columns[key] = make([]float64, rows)
					for i := range columns[key] {
						columns[key][i] = math.NaN()
					}
				}
				columns[key][i] = vs[k][j]
			}
		}
	}
	sort.Strings(prefixes)
	a.Prefixes = prefixes
	for _, prefix := range prefixes {
		for _, col := range a.Values {
			key := prefix + "/" + col
			output.Add(key, columns[key])
		}
	}
}

// firstMasterIndex returns the index of the first commit on master.
// This is used to find the value to normalize against.
func firstMasterIndex(bs []string) int {
	return slice.Index(bs, "master")
}

// commitIndex is a gg.Stat that adds a column called "commit-index" sequentially counting unique values of the column "commit".
type commitIndex struct{}

func (commitIndex) F(g table.Grouping) table.Grouping {
	return table.MapTables(g, func(_ table.GroupID, t *table.Table) *table.Table {
		idxs := make([]int, t.Len())
		last, idx := "", -1
		for i, hash := range t.MustColumn("commit").([]string) {
			if hash != last {
				idx++
				last = hash
			}
			idxs[i] = idx
		}
		t = table.NewBuilder(t).Add("commit-index", idxs).Done()

		return t
	})
}

// removeNaNs returns a new Grouping with rows containg NaN in col removed.
func removeNaNs(g table.Grouping, col string) table.Grouping {
	return table.Filter(g, func(result float64) bool {
		return !math.IsNaN(result)
	}, col)
}

// kza implements adaptive Kolmogorov-Zurbenko filtering on the data in X.
type kza struct {
	X    string
	M, K int
}

func (k kza) F(g table.Grouping) table.Grouping {
	return table.MapTables(g, func(_ table.GroupID, t *table.Table) *table.Table {
		var xs []float64
		slice.Convert(&xs, t.MustColumn(k.X))
		nxs := AdaptiveKolmogorovZurbenko(xs, k.M, k.K)
		return table.NewBuilder(t).Add("filtered "+k.X, nxs).Done()
	})
}

// column represents a column in a google.visualization.DataTable
type column struct {
	Name string `json:"id"`
	Role string `json:"role,omitempty"`
	// These fields are filled in by tableToJS if unspecified.
	Type  string `json:"type"`
	Label string `json:"label"`
}

// tableToJS converts a Table to a javascript literal which can be passed to "new google.visualization.DataTable".
func tableToJS(t *table.Table, columns []column) template.JS {
	var out bytes.Buffer
	fmt.Fprint(&out, "{cols: [")
	var slices []table.Slice
	for i, c := range columns {
		if i > 0 {
			fmt.Fprint(&out, ",\n")
		}
		col := t.Column(c.Name)
		slices = append(slices, col)
		if c.Type == "" {
			switch col.(type) {
			case []string:
				c.Type = "string"
			case []int, []float64:
				c.Type = "number"
			}
		}
		if c.Label == "" {
			c.Label = c.Name
		}
		data, err := json.Marshal(c)
		if err != nil {
			panic(err)
		}
		out.Write(data)
	}
	fmt.Fprint(&out, "],\nrows: [")
	for i := 0; i < t.Len(); i++ {
		if i > 0 {
			fmt.Fprint(&out, ",\n")
		}
		fmt.Fprint(&out, "{c:[")
		for j := range columns {
			if j > 0 {
				fmt.Fprint(&out, ", ")
			}
			fmt.Fprint(&out, "{v: ")
			var value []byte
			var err error
			switch column := slices[j].(type) {
			case []string:
				value, err = json.Marshal(column[i])
			case []int:
				value, err = json.Marshal(column[i])
			case []float64:
				value, err = json.Marshal(column[i])
			}
			if err != nil {
				panic(err)
			}
			out.Write(value)
			fmt.Fprint(&out, "}")
		}
		fmt.Fprint(&out, "]}")
	}
	fmt.Fprint(&out, "]}")
	return template.JS(out.String())
}
