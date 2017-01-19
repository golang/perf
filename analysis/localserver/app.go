// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Localserver runs an HTTP server for benchmark analysis.
//
// Usage:
//
//     localserver [-addr address] [-storage url]
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/perf/analysis/app"
	"golang.org/x/perf/storage"
)

var (
	addr       = flag.String("addr", "localhost:8080", "serve HTTP on `address`")
	storageURL = flag.String("storage", "https://perfdata.golang.org", "storage server base `url`")
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of localserver:
	localserver [flags]
`)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("localserver: ")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
	}

	app := &app.App{StorageClient: &storage.Client{BaseURL: *storageURL}}
	app.RegisterOnMux(http.DefaultServeMux)

	log.Printf("Listening on %s", *addr)

	log.Fatal(http.ListenAndServe(*addr, nil))
}
