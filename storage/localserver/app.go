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

var host = flag.String("port", ":8080", "(host and) port to bind on")

func main() {
	flag.Parse()

	db, err := db.OpenSQL("sqlite3", ":memory:")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	fs := fs.NewMemFS()

	app := &app.App{DB: db, FS: fs}
	app.RegisterOnMux(http.DefaultServeMux)

	log.Printf("Listening on %s", *host)

	log.Fatal(http.ListenAndServe(*host, nil))
}
