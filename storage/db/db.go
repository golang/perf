// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package db provides the high-level database interface for the
// storage app.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"golang.org/x/net/context"
)

// DB is a high-level interface to a database for the storage
// app. It's safe for concurrent use by multiple goroutines.
type DB struct {
	sql          *sql.DB
	insertUpload *sql.Stmt
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
	d := &DB{sql: db}
	if err := d.createTables(driverName); err != nil {
		return nil, err
	}
	if err := d.prepareStatements(driverName); err != nil {
		return nil, err
	}
	return d, nil
}

// createTables creates any missing tables on the connection in
// db.sql. driverName is the same driver name passed to sql.Open and
// is used to select the correct syntax.
func (db *DB) createTables(driverName string) error {
	var schema string
	switch driverName {
	case "sqlite3":
		schema = `
CREATE TABLE IF NOT EXISTS Uploads (
       UploadId INTEGER PRIMARY KEY AUTOINCREMENT
);
`
	default: // MySQL syntax
		schema = `
CREATE TABLE IF NOT EXISTS Uploads (
       UploadId SERIAL PRIMARY KEY AUTO_INCREMENT
);`
	}
	for _, q := range strings.Split(schema, ";") {
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
	return nil
}

// ReserveUploadID returns an upload ID which can be used for storing new files.
func (db *DB) ReserveUploadID(ctx context.Context) (string, error) {
	// TODO(quentin): Use a transaction?
	res, err := db.insertUpload.Exec()
	if err != nil {
		return "", err
	}
	// TODO(quentin): Use a date-based upload ID (YYYYMMDDnnn)
	i, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprint(i), nil
}

// TODO(quentin): Implement
// func (db *DB) InsertRecord(uploadid string, fields map[string]string, lines map[int]string) error

// Close closes the database connections, releasing any open resources.
func (db *DB) Close() error {
	return db.sql.Close()
}
