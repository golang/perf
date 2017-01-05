// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db

import "database/sql"

var SplitQueryWords = splitQueryWords

func DBSQL(db *DB) *sql.DB {
	return db.sql
}
