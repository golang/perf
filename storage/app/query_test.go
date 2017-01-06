// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"

	"golang.org/x/perf/storage/benchfmt"
)

func TestQuery(t *testing.T) {
	app := createTestApp(t)
	defer app.Close()

	// Write 1024 test results to the database.  These results
	// have labels named label0, label1, etc. Each label's value
	// is an integer whose value is (record number) / (1 << label
	// number).  So 1 record has each value of label0, 2 records
	// have each value of label1, 4 records have each value of
	// label2, etc. This allows writing queries that match 2^n records.
	app.uploadFiles(t, func(mpw *multipart.Writer) {
		w, err := mpw.CreateFormFile("file", "1.txt")
		if err != nil {
			t.Errorf("CreateFormFile: %v", err)
		}
		bp := benchfmt.NewPrinter(w)
		for i := 0; i < 1024; i++ {
			r := &benchfmt.Result{Labels: make(map[string]string), NameLabels: make(map[string]string), Content: "BenchmarkName 1 ns/op"}
			for j := uint(0); j < 10; j++ {
				r.Labels[fmt.Sprintf("label%d", j)] = fmt.Sprintf("%d", i/(1<<j))
			}
			r.NameLabels["name"] = "Name"
			if err := bp.Print(r); err != nil {
				t.Fatalf("Print: %v", err)
			}
		}
	})

	tests := []struct {
		q    string
		want []int
	}{
		{"label0:0", []int{0}},
		{"label1:0", []int{0, 1}},
		{"label0:5 name:Name", []int{5}},
		{"label0:0 label0:5", nil},
	}
	for _, test := range tests {
		t.Run("query="+test.q, func(t *testing.T) {
			u := app.srv.URL + "/search?" + url.Values{"q": []string{test.q}}.Encode()
			resp, err := http.Get(u)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("get /search: %v", resp.Status)
			}
			br := benchfmt.NewReader(resp.Body)
			for i, num := range test.want {
				r, err := br.Next()
				if err != nil {
					t.Fatalf("#%d: Next() = %v, want nil", i, err)
				}
				if r.Labels["label0"] != fmt.Sprintf("%d", num) {
					t.Errorf("#%d: label0 = %q, want %d", i, r.Labels["label0"], num)
				}
				if r.NameLabels["name"] != "Name" {
					t.Errorf("#%d: name = %q, want %q", i, r.NameLabels["name"], "Name")
				}
			}
			_, err = br.Next()
			if err != io.EOF {
				t.Errorf("Next() = %v, want EOF", err)
			}
		})
	}
}
