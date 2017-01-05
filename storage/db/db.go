// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package db provides the high-level database interface for the
// storage app.
package db

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/net/context"
	"golang.org/x/perf/storage/benchfmt"
)

// DB is a high-level interface to a database for the storage
// app. It's safe for concurrent use by multiple goroutines.
type DB struct {
	sql *sql.DB // underlying database connection
	// prepared statements
	insertUpload *sql.Stmt
	insertRecord *sql.Stmt
}

// OpenSQL creates a DB backed by a SQL database. The parameters are
// the same as the parameters for sql.Open. Only mysql and sqlite3 are
// explicitly supported; other database engines will receive MySQL
// query syntax which may or may not be compatible.
func OpenSQL(driverName, dataSourceName string) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	if hook := openHooks[driverName]; hook != nil {
		if err := hook(db); err != nil {
			return nil, err
		}
	}
	d := &DB{sql: db}
	if err := d.createTables(driverName); err != nil {
		return nil, err
	}
	if err := d.prepareStatements(driverName); err != nil {
		return nil, err
	}
	return d, nil
}

var openHooks = make(map[string]func(*sql.DB) error)

// RegisterOpenHook registers a hook to be called after opening a connection to driverName.
// This is used by the sqlite3 package to register a ConnectHook.
// It must be called from an init function.
func RegisterOpenHook(driverName string, hook func(*sql.DB) error) {
	openHooks[driverName] = hook
}

// createTmpl is the template used to prepare the CREATE statements
// for the database. It is evaluated with . as a map containing one
// entry whose key is the driver name.
var createTmpl = template.Must(template.New("create").Parse(`
CREATE TABLE IF NOT EXISTS Uploads (
	UploadID {{if .sqlite3}}INTEGER PRIMARY KEY AUTOINCREMENT{{else}}SERIAL PRIMARY KEY AUTO_INCREMENT{{end}}
);
CREATE TABLE IF NOT EXISTS Records (
	UploadID BIGINT UNSIGNED,
	RecordID BIGINT UNSIGNED,
	Content BLOB,
	PRIMARY KEY (UploadID, RecordID),
	FOREIGN KEY (UploadID) REFERENCES Uploads(UploadID) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS RecordLabels (
	UploadID BIGINT UNSIGNED,
	RecordID BIGINT UNSIGNED,
	Name VARCHAR(255),
	Value VARCHAR(8192),
{{if not .sqlite3}}
	Index (Name(100), Value(100)),
{{end}}
       FOREIGN KEY (UploadID, RecordID) REFERENCES Records(UploadID, RecordID) ON UPDATE CASCADE ON DELETE CASCADE
);
{{if .sqlite3}}
CREATE INDEX IF NOT EXISTS RecordLabelsNameValue ON RecordLabels(Name, Value);
{{end}}
`))

// createTables creates any missing tables on the connection in
// db.sql. driverName is the same driver name passed to sql.Open and
// is used to select the correct syntax.
func (db *DB) createTables(driverName string) error {
	var buf bytes.Buffer
	if err := createTmpl.Execute(&buf, map[string]bool{driverName: true}); err != nil {
		return err
	}
	for _, q := range strings.Split(buf.String(), ";") {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if _, err := db.sql.Exec(q); err != nil {
			return fmt.Errorf("create table: %v", err)
		}
	}
	return nil
}

// prepareStatements calls db.sql.Prepare on reusable SQL statements.
func (db *DB) prepareStatements(driverName string) error {
	var err error
	q := "INSERT INTO Uploads() VALUES ()"
	if driverName == "sqlite3" {
		q = "INSERT INTO Uploads DEFAULT VALUES"
	}
	db.insertUpload, err = db.sql.Prepare(q)
	if err != nil {
		return err
	}
	db.insertRecord, err = db.sql.Prepare("INSERT INTO Records(UploadID, RecordID, Content) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	return nil
}

// An Upload is a collection of files that share an upload ID.
type Upload struct {
	// ID is the value of the "uploadid" key that should be
	// associated with every record in this upload.
	ID string

	// id is the numeric value used as the primary key. ID is a
	// string for the public API; the underlying table actually
	// uses an integer key. To avoid repeated calls to
	// strconv.Atoi, the int64 is cached here.
	id int64
	// recordid is the index of the next record to insert.
	recordid int64
	// db is the underlying database that this upload is going to.
	db *DB
}

// NewUpload returns an upload for storing new files.
// All records written to the Upload will have the same upload ID.
func (db *DB) NewUpload(ctx context.Context) (*Upload, error) {
	// TODO(quentin): Use a transaction?
	res, err := db.insertUpload.Exec()
	if err != nil {
		return nil, err
	}
	// TODO(quentin): Use a date-based upload ID (YYYYMMDDnnn)
	i, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Upload{
		ID: fmt.Sprint(i),
		id: i,
		db: db,
	}, nil
}

// InsertRecord inserts a single record in an existing upload.
func (u *Upload) InsertRecord(r *benchfmt.Result) (err error) {
	// TODO(quentin): Use a single transaction for the whole upload?
	tx, err := u.db.sql.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	// TODO(quentin): Support multiple lines (slice of results?)
	var buf bytes.Buffer
	if err := benchfmt.NewPrinter(&buf).Print(r); err != nil {
		return err
	}
	if _, err = tx.Stmt(u.db.insertRecord).Exec(u.id, u.recordid, buf.Bytes()); err != nil {
		return err
	}
	var args []interface{}
	for _, k := range r.Labels.Keys() {
		args = append(args, u.id, u.recordid, k, r.Labels[k])
	}
	for _, k := range r.NameLabels.Keys() {
		args = append(args, u.id, u.recordid, k, r.NameLabels[k])
	}
	if len(args) > 0 {
		query := "INSERT INTO RecordLabels VALUES " + strings.Repeat("(?, ?, ?, ?), ", len(args)/4)
		query = strings.TrimSuffix(query, ", ")
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	u.recordid++
	return nil
}

// Close closes the database connections, releasing any open resources.
func (db *DB) Close() error {
	if err := db.insertUpload.Close(); err != nil {
		return err
	}
	if err := db.insertRecord.Close(); err != nil {
		return err
	}
	return db.sql.Close()
}
