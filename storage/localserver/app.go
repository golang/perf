// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"

	"golang.org/x/perf/storage/app"
	"golang.org/x/perf/storage/db"
	_ "golang.org/x/perf/storage/db/sqlite3"
	"golang.org/x/perf/storage/fs"
)

var (
	addr        = flag.String("addr", ":8080", "serve HTTP on `address`")
	viewURLBase = flag.String("view_url_base", "", "/upload response with `URL` for viewing")
)

func main() {
	flag.Parse()

	db, err := db.OpenSQL("sqlite3", ":memory:")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	fs := fs.NewMemFS()

	app := &app.App{
		DB:          db,
		FS:          fs,
		ViewURLBase: *viewURLBase,
		Auth:        func(http.ResponseWriter, *http.Request) (string, error) { return "", nil },
	}
	app.RegisterOnMux(http.DefaultServeMux)

	log.Printf("Listening on %s", *addr)

	log.Fatal(http.ListenAndServe(*addr, nil))
}
