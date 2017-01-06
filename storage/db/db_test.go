// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/perf/storage/benchfmt"
	. "golang.org/x/perf/storage/db"
	_ "golang.org/x/perf/storage/db/sqlite3"
)

// Most of the db package is tested via the end-to-end-tests in perf/storage/app.

func TestSplitQueryWords(t *testing.T) {
	for _, test := range []struct {
		q    string
		want []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"hello\\ world", []string{"hello world"}},
		{`"key:value two" and\ more`, []string{"key:value two", "and more"}},
		{`one" two"\ three four`, []string{"one two three", "four"}},
		{`"4'7\""`, []string{`4'7"`}},
	} {
		have := SplitQueryWords(test.q)
		if !reflect.DeepEqual(have, test.want) {
			t.Errorf("splitQueryWords(%q) = %+v, want %+v", test.q, have, test.want)
		}
	}
}

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

func TestQuery(t *testing.T) {
	db, err := OpenSQL("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	u, err := db.NewUpload(context.Background())
	if err != nil {
		t.Fatalf("NewUpload: %v", err)
	}

	for i := 0; i < 1024; i++ {
		r := &benchfmt.Result{Labels: make(map[string]string), NameLabels: make(map[string]string), Content: "BenchmarkName 1 ns/op"}
		for j := uint(0); j < 10; j++ {
			r.Labels[fmt.Sprintf("label%d", j)] = fmt.Sprintf("%d", i/(1<<j))
		}
		r.NameLabels["name"] = "Name"
		if err := u.InsertRecord(r); err != nil {
			t.Fatalf("InsertRecord: %v", err)
		}
	}

	tests := []struct {
		q    string
		want []int // nil means we want an error
	}{
		{"label0:0", []int{0}},
		{"label1:0", []int{0, 1}},
		{"label0:5 name:Name", []int{5}},
		{"label0:0 label0:5", []int{}},
		{"bogus query", nil},
	}
	for _, test := range tests {
		t.Run("query="+test.q, func(t *testing.T) {
			q := db.Query(test.q)
			if test.want == nil {
				if q.Next() {
					t.Fatal("Next() = true, want false")
				}
				if err := q.Err(); err == nil {
					t.Fatal("Err() = nil, want error")
				}
				return
			}
			defer func() {
				if err := q.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			for i, num := range test.want {
				if !q.Next() {
					t.Fatalf("#%d: Next() = false", i)
				}
				r := q.Result()
				if r.Labels["label0"] != fmt.Sprintf("%d", num) {
					t.Errorf("result[%d].label0 = %q, want %d", i, r.Labels["label0"], num)
				}
				if r.NameLabels["name"] != "Name" {
					t.Errorf("result[%d].name = %q, want %q", i, r.NameLabels["name"], "Name")
				}
			}
			if err := q.Err(); err != nil {
				t.Errorf("Err() = %v, want nil", err)
			}
		})
	}
}
