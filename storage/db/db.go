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
	"strings"
	"text/template"
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
