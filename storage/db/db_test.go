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
	"time"

	"golang.org/x/perf/storage/benchfmt"
	. "golang.org/x/perf/storage/db"
	"golang.org/x/perf/storage/db/dbtest"
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

// TestUploadIDs verifies that NewUpload generates the correct sequence of upload IDs.
func TestUploadIDs(t *testing.T) {
	ctx := context.Background()

	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

	defer SetNow(time.Time{})

	tests := []struct {
		sec int64
		id  string
	}{
		{0, "19700101.1"},
		{0, "19700101.2"},
		{86400, "19700102.1"},
		{86400, "19700102.2"},
		{86400, "19700102.3"},
		{86400, "19700102.4"},
		{86400, "19700102.5"},
		{86400, "19700102.6"},
		{86400, "19700102.7"},
		{86400, "19700102.8"},
		{86400, "19700102.9"},
		{86400, "19700102.10"},
		{86400, "19700102.11"},
	}
	for _, test := range tests {
		SetNow(time.Unix(test.sec, 0))
		u, err := db.NewUpload(ctx)
		if err != nil {
			t.Fatalf("NewUpload: %v", err)
		}
		if err := u.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if u.ID != test.id {
			t.Fatalf("u.ID = %q, want %q", u.ID, test.id)
		}
	}
}

// TestNewUpload verifies that NewUpload and InsertRecord wrote the correct rows to the database.
func TestNewUpload(t *testing.T) {
	SetNow(time.Unix(0, 0))
	defer SetNow(time.Time{})
	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

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

	if err := u.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
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
		var uploadid string
		var recordid int64
		var name, value string

		if err := rows.Scan(&uploadid, &recordid, &name, &value); err != nil {
			t.Fatalf("rows.Scan: %v", err)
		}
		if uploadid != "19700101.1" {
			t.Errorf("uploadid = %q, want %q", uploadid, "19700101.1")
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
	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

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
	if err := u.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
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
