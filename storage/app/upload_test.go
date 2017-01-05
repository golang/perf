// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/perf/storage/db"
	_ "golang.org/x/perf/storage/db/sqlite3"
	"golang.org/x/perf/storage/fs"
)

func TestUpload(t *testing.T) {
	db, err := db.OpenSQL("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	fs := fs.NewMemFS()

	app := &App{DB: db, FS: fs}

	srv := httptest.NewServer(http.HandlerFunc(app.upload))
	defer srv.Close()
	pr, pw := io.Pipe()
	mpw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer mpw.Close()
		// Write the parts here
		w, err := mpw.CreateFormFile("file", "1.txt")
		if err != nil {
			t.Errorf("CreateFormFile: %v", err)
		}
		fmt.Fprintf(w, "key: value\nBenchmarkOne 5 ns/op\nkey:value2\nBenchmarkTwo 10 ns/op\n")
	}()
	resp, err := http.Post(srv.URL, mpw.FormDataContentType(), pr)
	if err != nil {
		t.Fatalf("post /upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("post /upload: %v", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("reading /upload response: %v", err)
	}
	t.Logf("/upload response:\n%s", body)

	if len(fs.Files()) != 1 {
		t.Errorf("/upload wrote %d files, want 1", len(fs.Files()))
	}
}
