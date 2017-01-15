// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package db provides the high-level database interface for the
// storage app.
package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"golang.org/x/net/context"
	"golang.org/x/perf/storage/benchfmt"
)

// TODO(quentin): Add Context to every function when App Engine supports Go >=1.8.

// DB is a high-level interface to a database for the storage
// app. It's safe for concurrent use by multiple goroutines.
type DB struct {
	sql *sql.DB // underlying database connection
	// prepared statements
	lastUpload   *sql.Stmt
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
	UploadID VARCHAR(20) PRIMARY KEY,
	Day VARCHAR(8),
	Seq BIGINT UNSIGNED
{{if not .sqlite3}}
	, Index (Day, Seq)
{{end}}
);
{{if .sqlite3}}
CREATE INDEX IF NOT EXISTS UploadDaySeq ON Uploads(Day, Seq);
{{end}}
CREATE TABLE IF NOT EXISTS Records (
	UploadID VARCHAR(20),
	RecordID BIGINT UNSIGNED,
	Content BLOB,
	PRIMARY KEY (UploadID, RecordID),
	FOREIGN KEY (UploadID) REFERENCES Uploads(UploadID) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS RecordLabels (
	UploadID VARCHAR(20),
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
	query := "SELECT UploadID FROM Uploads ORDER BY Day DESC, Seq DESC LIMIT 1"
	if driverName != "sqlite3" {
		query += " FOR UPDATE"
	}
	db.lastUpload, err = db.sql.Prepare(query)
	if err != nil {
		return err
	}
	db.insertUpload, err = db.sql.Prepare("INSERT INTO Uploads(UploadID, Day, Seq) VALUES (?, ?, ?)")
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

	// recordid is the index of the next record to insert.
	recordid int64
	// db is the underlying database that this upload is going to.
	db *DB
	// tx is the transaction used by the upload.
	tx *sql.Tx
}

// now is a hook for testing
var now = time.Now

// NewUpload returns an upload for storing new files.
// All records written to the Upload will have the same upload ID.
func (db *DB) NewUpload(ctx context.Context) (*Upload, error) {
	day := now().UTC().Format("20060102")

	num := 0

	tx, err := db.sql.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	var lastID string
	err = tx.Stmt(db.lastUpload).QueryRow().Scan(&lastID)
	switch err {
	case sql.ErrNoRows:
	case nil:
		if strings.HasPrefix(lastID, day) {
			num, err = strconv.Atoi(lastID[len(day)+1:])
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, err
	}

	num++

	id := fmt.Sprintf("%s.%d", day, num)

	_, err = tx.Stmt(db.insertUpload).Exec(id, day, num)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil

	utx, err := db.sql.Begin()
	if err != nil {
		return nil, err
	}
	return &Upload{
		ID: id,
		db: db,
		tx: utx,
	}, nil
}

// InsertRecord inserts a single record in an existing upload.
// If InsertRecord returns a non-nil error, the Upload has failed and u.Abort() must be called.
func (u *Upload) InsertRecord(r *benchfmt.Result) error {
	// TODO(quentin): Support multiple lines (slice of results?)
	var buf bytes.Buffer
	if err := benchfmt.NewPrinter(&buf).Print(r); err != nil {
		return err
	}
	if _, err := u.tx.Stmt(u.db.insertRecord).Exec(u.ID, u.recordid, buf.Bytes()); err != nil {
		return err
	}
	var args []interface{}
	for _, k := range r.Labels.Keys() {
		args = append(args, u.ID, u.recordid, k, r.Labels[k])
	}
	for _, k := range r.NameLabels.Keys() {
		args = append(args, u.ID, u.recordid, k, r.NameLabels[k])
	}
	if len(args) > 0 {
		query := "INSERT INTO RecordLabels VALUES " + strings.Repeat("(?, ?, ?, ?), ", len(args)/4)
		query = strings.TrimSuffix(query, ", ")
		if _, err := u.tx.Exec(query, args...); err != nil {
			return err
		}
	}
	u.recordid++
	return nil
}

// Commit finishes processing the upload.
func (u *Upload) Commit() error {
	return u.tx.Commit()
}

// Abort cleans up resources associated with the upload.
// It does not attempt to clean up partial database state.
func (u *Upload) Abort() error {
	return u.tx.Rollback()
}

// Query searches for results matching the given query string.
//
// The query string is first parsed into quoted words (as in the shell)
// and then each word must be formatted as one of the following:
// key:value - exact match on label "key" = "value"
// key>value - value greater than (useful for dates)
// key<value - value less than (also useful for dates)
func (db *DB) Query(q string) *Query {
	qparts := splitQueryWords(q)

	var args []interface{}
Words:
	for _, part := range qparts {
		for i, c := range part {
			switch {
			case c == ':':
				args = append(args, part[:i], part[i+1:])
				continue Words
			case c == '>' || c == '<':
				// TODO
				return &Query{err: errors.New("unsupported operator")}
			case unicode.IsSpace(c) || unicode.IsUpper(c):
				return &Query{err: fmt.Errorf("query part %q has invalid key", part)}
			}
		}
		return &Query{err: fmt.Errorf("query part %q is missing operator", part)}
	}

	query := "SELECT r.Content FROM "
	for i := 0; i < len(args)/2; i++ {
		if i > 0 {
			query += " INNER JOIN "
		}
		query += fmt.Sprintf("(SELECT UploadID, RecordID FROM RecordLabels WHERE Name = ? AND Value = ?) t%d", i)
		if i > 0 {
			query += " USING (UploadID, RecordID)"
		}
	}

	// TODO(quentin): Handle empty query string.

	query += " LEFT JOIN Records r USING (UploadID, RecordID)"

	rows, err := db.sql.Query(query, args...)
	if err != nil {
		return &Query{err: err}
	}
	return &Query{rows: rows}
}

// splitQueryWords splits q into words using shell syntax (whitespace
// can be escaped with double quotes or with a backslash).
func splitQueryWords(q string) []string {
	var words []string
	word := make([]byte, len(q))
	w := 0
	quoting := false
	for r := 0; r < len(q); r++ {
		switch c := q[r]; {
		case c == '"' && quoting:
			quoting = false
		case quoting:
			if c == '\\' {
				r++
			}
			if r < len(q) {
				word[w] = q[r]
				w++
			}
		case c == '"':
			quoting = true
		case c == ' ', c == '\t':
			if w > 0 {
				words = append(words, string(word[:w]))
			}
			w = 0
		case c == '\\':
			r++
			fallthrough
		default:
			if r < len(q) {
				word[w] = q[r]
				w++
			}
		}
	}
	if w > 0 {
		words = append(words, string(word[:w]))
	}
	return words
}

// Query is the result of a query.
// Use Next to advance through the rows, making sure to call Close when done:
//
//   q, err := db.Query("key:value")
//   defer q.Close()
//   for q.Next() {
//     res := q.Result()
//     ...
//   }
//   err = q.Err() // get any error encountered during iteration
//   ...
type Query struct {
	rows *sql.Rows
	// from last call to Next
	result *benchfmt.Result
	err    error
}

// Next prepares the next result for reading with the Result
// method. It returns false when there are no more results, either by
// reaching the end of the input or an error.
func (q *Query) Next() bool {
	if q.err != nil {
		return false
	}
	if !q.rows.Next() {
		return false
	}
	var content []byte
	q.err = q.rows.Scan(&content)
	if q.err != nil {
		return false
	}
	// TODO(quentin): Needs to change when one row contains multiple Results.
	q.result, q.err = benchfmt.NewReader(bytes.NewReader(content)).Next()
	return q.err == nil
}

// Result returns the most recent result generated by a call to Next.
func (q *Query) Result() *benchfmt.Result {
	return q.result
}

// Err returns the error state of the query.
func (q *Query) Err() error {
	if q.err == io.EOF {
		return nil
	}
	return q.err
}

// Close frees resources associated with the query.
func (q *Query) Close() error {
	if q.rows != nil {
		return q.rows.Close()
	}
	return q.err
}

// CountUploads returns the number of uploads in the database.
func (db *DB) CountUploads() (int, error) {
	var uploads int
	err := db.sql.QueryRow("SELECT COUNT(*) FROM Uploads").Scan(&uploads)
	return uploads, err
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
