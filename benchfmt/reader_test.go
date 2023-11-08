// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func parseAll(t *testing.T, data string, setup ...func(r *Reader, sr io.Reader)) ([]Record, *Reader) {
	sr := strings.NewReader(data)
	r := NewReader(sr, "test")
	for _, f := range setup {
		f(r, sr)
	}
	var out []Record
	for r.Scan() {
		switch rec := r.Result(); rec := rec.(type) {
		case *Result:
			res := rec.Clone()
			// Wipe position information for comparisons.
			res.fileName = ""
			res.line = 0
			out = append(out, res)
		case *SyntaxError, *UnitMetadata:
			out = append(out, rec)
		default:
			t.Fatalf("unexpected result type %T", rec)
		}
	}
	if err := r.Err(); err != nil {
		t.Fatal("parsing failed: ", err)
	}
	return out, r
}

func printRecord(w io.Writer, r Record) {
	switch r := r.(type) {
	case *Result:
		for _, fc := range r.Config {
			fmt.Fprintf(w, "{%s: %s} ", fc.Key, fc.Value)
		}
		fmt.Fprintf(w, "%s %d", r.Name.Full(), r.Iters)
		for _, val := range r.Values {
			fmt.Fprintf(w, " %v %s", val.Value, val.Unit)
		}
		fmt.Fprintf(w, "\n")
	case *UnitMetadata:
		fmt.Fprintf(w, "Unit: %+v\n", r)
	case *SyntaxError:
		fmt.Fprintf(w, "SyntaxError: %s\n", r)
	default:
		panic(fmt.Sprintf("unknown record type %T", r))
	}
}

type resultBuilder struct {
	res *Result
}

func r(fullName string, iters int) *resultBuilder {
	return &resultBuilder{
		&Result{
			Config: []Config{},
			Name:   Name(fullName),
			Iters:  iters,
		},
	}
}

func (b *resultBuilder) config(keyVals ...string) *resultBuilder {
	for i := 0; i < len(keyVals); i += 2 {
		key, val := keyVals[i], keyVals[i+1]
		file := true
		if val[0] == '*' {
			file = false
			val = val[1:]
		}
		b.res.Config = append(b.res.Config, Config{key, []byte(val), file})
	}
	return b
}

func (b *resultBuilder) v(value float64, unit string) *resultBuilder {
	var v Value
	if unit == "ns/op" {
		v = Value{Value: value * 1e-9, Unit: "sec/op", OrigValue: value, OrigUnit: unit}
	} else {
		v = Value{Value: value, Unit: unit}
	}
	b.res.Values = append(b.res.Values, v)
	return b
}

func compareRecords(t *testing.T, got, want []Record) {
	t.Helper()
	var diff bytes.Buffer
	for i := 0; i < len(got) || i < len(want); i++ {
		if i >= len(got) {
			fmt.Fprintf(&diff, "[%d] got: none, want:\n", i)
			printRecord(&diff, want[i])
		} else if i >= len(want) {
			fmt.Fprintf(&diff, "[%d] want: none, got:\n", i)
			printRecord(&diff, got[i])
		} else if !reflect.DeepEqual(got[i], want[i]) {
			fmt.Fprintf(&diff, "[%d] got:\n", i)
			printRecord(&diff, got[i])
			fmt.Fprintf(&diff, "[%d] want:\n", i)
			printRecord(&diff, want[i])
		}
	}
	if diff.Len() != 0 {
		t.Error(diff.String())
	}
}

func TestReader(t *testing.T) {
	type testCase struct {
		name, input string
		want        []Record
	}
	for _, test := range []testCase{
		{
			"basic",
			`key: value
BenchmarkOne 100 1 ns/op 2 B/op
BenchmarkTwo 300 4.5 ns/op
`,
			[]Record{
				r("One", 100).
					config("key", "value").
					v(1, "ns/op").v(2, "B/op").res,
				r("Two", 300).
					config("key", "value").
					v(4.5, "ns/op").res,
			},
		},
		{
			"weird",
			`
BenchmarkSpaces    1   1   ns/op
BenchmarkHugeVal 1 9999999999999999999999999999999 ns/op
BenchmarkEmSpace  1  1  ns/op
`,
			[]Record{
				r("Spaces", 1).
					v(1, "ns/op").res,
				r("HugeVal", 1).
					v(9999999999999999999999999999999, "ns/op").res,
				r("EmSpace", 1).
					v(1, "ns/op").res,
			},
		},
		{
			"basic file keys",
			`key1:    	 value
: not a key
ab:not a key
a b: also not a key
key2: value

BenchmarkOne 100 1 ns/op
`,
			[]Record{
				r("One", 100).
					config("key1", "value", "key2", "value").
					v(1, "ns/op").res,
			},
		},
		{
			"bad lines",
			`not a benchmark
BenchmarkTailingSpaceNoIter 
BenchmarkBadIter abc
BenchmarkHugeIter 9999999999999999999999999999999
BenchmarkMissingVal 100
BenchmarkBadVal 100 abc
BenchmarkMissingUnit 100 1
BenchmarkMissingUnit2 100 1 ns/op 2
also not a benchmark
Unit
Unit ns/op blah
Unit ns/op a=1
Unit ns/op a=2
`,
			[]Record{
				&SyntaxError{"test", 2, "missing iteration count"},
				&SyntaxError{"test", 3, "parsing iteration count: invalid syntax"},
				&SyntaxError{"test", 4, "parsing iteration count: value out of range"},
				&SyntaxError{"test", 5, "missing measurements"},
				&SyntaxError{"test", 6, "parsing measurement: invalid syntax"},
				&SyntaxError{"test", 7, "missing units"},
				&SyntaxError{"test", 8, "missing units"},
				&SyntaxError{"test", 10, "missing unit"},
				&SyntaxError{"test", 11, "expected key=value"},
				&UnitMetadata{UnitMetadataKey{"sec/op", "a"}, "ns/op", "1", "test", 12},
				&SyntaxError{"test", 13, "metadata a of unit ns/op already set to 1"},
			},
		},
		{
			"remove existing label",
			`key: value
key:
BenchmarkOne 100 1 ns/op
`,
			[]Record{
				r("One", 100).
					v(1, "ns/op").res,
			},
		},
		{
			"overwrite exiting label",
			`key1: first
key2: second
key1: third
BenchmarkOne 100 1 ns/op
`,
			[]Record{
				r("One", 100).
					config("key1", "third", "key2", "second").
					v(1, "ns/op").res,
			},
		},
		{
			"unit metadata",
			`Unit ns/op a=1 b=2
Unit ns/op c=3 error d=4
# Repeated unit should report nothing
Unit ns/op d=4
# Starts like a unit line but actually isn't
Unitx
BenchmarkOne 100 1 ns/op
`,
			[]Record{
				&UnitMetadata{UnitMetadataKey{"sec/op", "a"}, "ns/op", "1", "test", 1},
				&UnitMetadata{UnitMetadataKey{"sec/op", "b"}, "ns/op", "2", "test", 1},
				&UnitMetadata{UnitMetadataKey{"sec/op", "c"}, "ns/op", "3", "test", 2},
				&SyntaxError{"test", 2, "expected key=value"},
				&UnitMetadata{UnitMetadataKey{"sec/op", "d"}, "ns/op", "4", "test", 2},
				r("One", 100).
					v(1, "ns/op").res,
			},
		},
		{
			// go test -v prints just the benchmark name
			// on a line when starting each benchmark.
			// Make sure we ignore it.
			"verbose",
			`BenchmarkOne
BenchmarkOne 100 1 ns/op
`,
			[]Record{
				r("One", 100).
					v(1, "ns/op").res,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, _ := parseAll(t, test.input)
			compareRecords(t, got, test.want)
		})
	}
}

func TestReaderInternalConfig(t *testing.T) {
	got, _ := parseAll(t, `
# Test initial internal config
Benchmark1 100 1 ns/op
# Overwrite internal config with file config
key1: file1
key3: file3
Benchmark2 100 1 ns/op
# Delete internal config, check that file config is right
key2:
Benchmark3 100 1 ns/op
	`, func(r *Reader, sr io.Reader) {
		r.Reset(sr, "test", "key1", "internal1", "key2", "internal2")
	})
	want := []Record{
		r("1", 100).v(1, "ns/op").config("key1", "*internal1", "key2", "*internal2").res,
		r("2", 100).v(1, "ns/op").config("key1", "file1", "key2", "*internal2", "key3", "file3").res,
		r("3", 100).v(1, "ns/op").config("key1", "file1", "key3", "file3").res,
	}
	compareRecords(t, got, want)
}

func BenchmarkReader(b *testing.B) {
	path := "testdata/bent"
	fileInfos, err := os.ReadDir(path)
	if err != nil {
		b.Fatal("reading test data directory: ", err)
	}

	var files []*os.File
	for _, info := range fileInfos {
		f, err := os.Open(filepath.Join(path, info.Name()))
		if err != nil {
			b.Fatal(err)
		}
		defer f.Close()
		files = append(files, f)
	}

	b.ResetTimer()

	start := time.Now()
	var n int
	for i := 0; i < b.N; i++ {
		r := new(Reader)
		for _, f := range files {
			if _, err := f.Seek(0, 0); err != nil {
				b.Fatal("seeking to 0: ", err)
			}
			r.Reset(f, f.Name())
			for r.Scan() {
				n++
				if err, ok := r.Result().(error); ok {
					b.Fatal("malformed record: ", err)
				}
			}
			if err := r.Err(); err != nil {
				b.Fatal(err)
			}
		}
	}
	dur := time.Since(start)
	b.Logf("read %d records", n)

	b.StopTimer()
	b.ReportMetric(float64(n/b.N), "records/op")
	b.ReportMetric(float64(n)*float64(time.Second)/float64(dur), "records/sec")
}
