// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dbtest

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"flag"
	"fmt"
	"testing"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
	"golang.org/x/perf/storage/db"
	_ "golang.org/x/perf/storage/db/sqlite3"
)

var cloud = flag.Bool("cloud", false, "connect to Cloud SQL database instead of in-memory SQLite")
var cloudsql = flag.String("cloudsql", "golang-org:us-central1:golang-org", "name of Cloud SQL instance to run tests on")

// createEmptyCloudDB makes a new, empty database for the test.
func createEmptyCloudDB(t *testing.T) (dsn string, cleanup func()) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}

	name := "perfdata-test-" + base64.RawURLEncoding.EncodeToString(buf)

	prefix := fmt.Sprintf("root:@cloudsql(%s)/", *cloudsql)

	db, err := sql.Open("mysql", prefix)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", name)); err != nil {
		db.Close()
		t.Fatal(err)
	}

	t.Logf("Using database %q", name)

	return prefix + name, func() {
		if _, err := db.Exec(fmt.Sprintf("DROP DATABASE `%s`", name)); err != nil {
			t.Error(err)
		}
		db.Close()
	}
}

// NewDB makes a connection to a testing database, either sqlite3 or
// Cloud SQL depending on the -cloud flag. cleanup must be called when
// done with the testing database, instead of calling db.Close()
func NewDB(t *testing.T) (*db.DB, func()) {
	driverName, dataSourceName := "sqlite3", ":memory:"
	var cloudCleanup func()
	if *cloud {
		driverName = "mysql"
		dataSourceName, cloudCleanup = createEmptyCloudDB(t)
	}
	d, err := db.OpenSQL(driverName, dataSourceName)
	if err != nil {
		if cloudCleanup != nil {
			cloudCleanup()
		}
		t.Fatalf("open database: %v", err)
	}

	cleanup := func() {
		if cloudCleanup != nil {
			cloudCleanup()
		}
		d.Close()
	}
	// Make sure the database really is empty.
	uploads, err := d.CountUploads()
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	if uploads != 0 {
		cleanup()
		t.Fatalf("found %d row(s) in Uploads, want 0", uploads)
	}
	return d, cleanup
}
