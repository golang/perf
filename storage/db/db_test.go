// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db_test

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/perf/storage/benchfmt"
	. "golang.org/x/perf/storage/db"
	_ "golang.org/x/perf/storage/db/sqlite3"
)

// Most of the db package is tested via the end-to-end-tests in perf/storage/app.

// TestNewUpload verifies that NewUpload and InsertRecord wrote the correct rows to the database.
func TestNewUpload(t *testing.T) {
	db, err := OpenSQL("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	u, err := db.NewUpload(context.Background())
	if err != nil {
		t.Fatalf("NewUpload: %v", err)
	}

	br := benchfmt.NewReader(strings.NewReader(`
key: value
BenchmarkName 1 ns/op
`))

	r, err := br.Next()
	if err != nil {
		t.Fatalf("BenchmarkReader.Next: %v", err)
	}

	if err := u.InsertRecord(r); err != nil {
		t.Fatalf("InsertRecord: %v", err)
	}

	rows, err := DBSQL(db).Query("SELECT UploadId, RecordId, Name, Value FROM RecordLabels")
	if err != nil {
		t.Fatalf("sql.Query: %v", err)
	}
	defer rows.Close()

	want := map[string]string{
		"key":  "value",
		"name": "Name",
	}

	i := 0

	for rows.Next() {
		var uploadid, recordid int64
		var name, value string

		if err := rows.Scan(&uploadid, &recordid, &name, &value); err != nil {
			t.Fatalf("rows.Scan: %v")
		}
		if uploadid != 1 {
			t.Errorf("uploadid = %d, want 1", uploadid)
		}
		if recordid != 0 {
			t.Errorf("recordid = %d, want 0", recordid)
		}
		if want[name] != value {
			t.Errorf("%s = %q, want %q", name, value, want[name])
		}
		i++
	}
	if i != len(want) {
		t.Errorf("have %d labels, want %d", i, len(want))
	}

	if err := rows.Err(); err != nil {
		t.Errorf("rows.Err: %v", err)
	}
}
