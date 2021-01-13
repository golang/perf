// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// benchfilter reads Go benchmark results from input files, filters
// them, and writes filtered benchmark results to stdout. If no inputs
// are provided, it reads from stdin.
//
// The filter language is described at
// https://pkg.go.dev/golang.org/x/perf/cmd/benchstat#Filtering
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchproc"
)

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), `Usage: benchfilter query [inputs...]

benchfilter reads Go benchmark results from input files, filters them,
and writes filtered benchmark results to stdout. If no inputs are
provided, it reads from stdin.

The filter language is described at
https://pkg.go.dev/golang.org/x/perf/cmd/benchstat#Filtering
`)
	flag.PrintDefaults()
}

func main() {
	log.SetPrefix("")
	log.SetFlags(0)

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	// TODO: Consider adding filtering on values, like "@ns/op>=100".

	filter, err := benchproc.NewFilter(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	writer := benchfmt.NewWriter(os.Stdout)
	files := benchfmt.Files{Paths: flag.Args()[1:], AllowStdin: true, AllowLabels: true}
	for files.Scan() {
		rec := files.Result()
		switch rec := rec.(type) {
		case *benchfmt.SyntaxError:
			// Non-fatal result parse error. Warn
			// but keep going.
			fmt.Fprintln(os.Stderr, rec)
			continue
		case *benchfmt.Result:
			if ok, err := filter.Apply(rec); !ok {
				if err != nil {
					// Print the reason we rejected this result.
					fmt.Fprintln(os.Stderr, err)
				}
				continue
			}
		}

		err = writer.Write(rec)
		if err != nil {
			log.Fatal("writing output: ", err)
		}
	}
	if err := files.Err(); err != nil {
		log.Fatal(err)
	}
}
