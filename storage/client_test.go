// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/perf/storage/benchfmt"
)

func TestQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.RequestURI(), "/search?q=key1%3Avalue+key2%3Avalue"; have != want {
			t.Errorf("RequestURI = %q, want %q", have, want)
		}
		fmt.Fprintf(w, "key: value\nBenchmarkOne 5 ns/op\nkey: value2\nBenchmarkTwo 10 ns/op\n")
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}

	q := c.Query("key1:value key2:value")
	defer q.Close()

	var buf bytes.Buffer
	bp := benchfmt.NewPrinter(&buf)

	for q.Next() {
		if err := bp.Print(q.Result()); err != nil {
			t.Fatalf("Print: %v", err)
		}
	}
	if err := q.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := "key: value\nBenchmarkOne 5 ns/op\nkey: value2\nBenchmarkTwo 10 ns/op\n"
	if diff := diff(buf.String(), want); diff != "" {
		t.Errorf("wrong results: (- have/+ want)\n%s", diff)
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
