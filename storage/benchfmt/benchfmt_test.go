// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func readAllResults(t *testing.T, r *Reader) []*Result {
	var out []*Result
	for {
		result, err := r.Next()
		switch err {
		case io.EOF:
			return out
		case nil:
			out = append(out, result)
		default:
			t.Fatal(err)
		}
	}
}

func TestBenchmarkReader(t *testing.T) {
	tests := []struct {
		name, input string
		want        []*Result
	}{
		{
			"basic",
			`key: value
BenchmarkOne 1 ns/sec
`,
			[]*Result{{
				Labels{"key": "value"},
				Labels{"name": "One"},
				2,
				"BenchmarkOne 1 ns/sec",
			}},
		},
		{
			"two results with indexed and named subnames",
			`key: value
BenchmarkOne/foo/bar=1-2 1 ns/sec
BenchmarkTwo 2 ns/sec
`,
			[]*Result{
				{
					Labels{"key": "value"},
					Labels{"name": "One", "sub1": "foo", "bar": "1", "gomaxprocs": "2"},
					2,
					"BenchmarkOne/foo/bar=1-2 1 ns/sec",
				},
				{
					Labels{"key": "value"},
					Labels{"name": "Two"},
					3,
					"BenchmarkTwo 2 ns/sec",
				},
			},
		},
		{
			"remove existing label",
			`key: value
key:
BenchmarkOne 1 ns/sec
`,
			[]*Result{
				{
					Labels{},
					Labels{"name": "One"},
					3,
					"BenchmarkOne 1 ns/sec",
				},
			},
		},
		{
			"parse file headers",
			`key: fixed

key: haha
BenchmarkOne 1 ns/sec
`,
			[]*Result{
				{
					Labels{"key": "fixed"},
					Labels{"name": "One"},
					4,
					"BenchmarkOne 1 ns/sec",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(test.input))
			have := readAllResults(t, r)
			want := test.want
			diff := ""
			mismatch := false
			for i := 0; i < len(have) || i < len(want); i++ {
				if i < len(have) && i < len(want) && reflect.DeepEqual(have[i], want[i]) {
					diff += fmt.Sprintf(" %+v\n", have[i])
					continue
				}
				mismatch = true
				if i < len(have) {
					diff += fmt.Sprintf("-%+v\n", have[i])
				}
				if i < len(want) {
					diff += fmt.Sprintf("+%+v\n", want[i])
				}
			}
			if mismatch {
				t.Errorf("wrong results: (- have/+ want)\n%s", diff)
			}
		})
	}
}

func TestBenchmarkPrinter(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{
			"basic",
			`key: value
BenchmarkOne 1 ns/sec
`,
			`key: value
BenchmarkOne 1 ns/sec
`,
		},
		{
			"missing newline",
			`key: value
BenchmarkOne 1 ns/sec`,
			`key: value
BenchmarkOne 1 ns/sec
`,
		},
		{
			"duplicate and removed fields",
			`one: 1
two: 2
BenchmarkOne 1 ns/sec
one: 1
two: 3
BenchmarkOne 1 ns/sec
two:
BenchmarkOne 1 ns/sec
`,
			`one: 1
two: 2
BenchmarkOne 1 ns/sec
two: 3
BenchmarkOne 1 ns/sec
two:
BenchmarkOne 1 ns/sec
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(test.input))
			results := readAllResults(t, r)
			var have bytes.Buffer
			bp := NewPrinter(&have)
			for _, result := range results {
				if err := bp.Print(result); err != nil {
					t.Errorf("Print returned %v", err)
				}
			}
			if diff := diff(have.String(), test.want); diff != "" {
				t.Errorf("wrong output: (- got/+ want)\n%s", diff)
			}
		})
	}
}

// diff returns the output of unified diff on s1 and s2. If the result
// is non-empty, the strings differ or the diff command failed.
func diff(s1, s2 string) string {
	f1, err := ioutil.TempFile("", "benchfmt_test")
	if err != nil {
		return err.Error()
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "benchfmt_test")
	if err != nil {
		return err.Error()
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	f1.Write([]byte(s1))
	f2.Write([]byte(s2))

	data, err := exec.Command("diff", "-u", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	if err != nil {
		data = append(data, []byte(err.Error())...)
	}
	return string(data)

}
