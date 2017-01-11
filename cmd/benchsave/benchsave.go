// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Benchsave uploads benchmark results to a storage server.
//
// Usage:
//
//	benchsave [-server https://server.org] a.txt [b.txt ...]
//
// Each input file should contain the output from one or more runs of
// ``go test -bench'', or another tool which uses the same format.
//
// benchsave will upload the input files to the specified server and
// print a URL where they can be viewed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

var (
	server  = flag.String("server", "https://perfdata.golang.org", "perfdata server to upload benchmarks to")
	verbose = flag.Bool("v", false, "verbose")
)

type uploadStatus struct {
	// UploadID is the upload ID assigned to the upload.
	UploadID string `json:"uploadid"`
	// FileIDs is the list of file IDs assigned to the files in the upload.
	FileIDs []string `json:"fileids"`
	// ViewURL is a server-supplied URL to view the results.
	ViewURL string `json:"viewurl"`
}

// writeOneFile reads name and writes it to mpw.
func writeOneFile(mpw *multipart.Writer, name string) {
	w, err := mpw.CreateFormFile("file", filepath.Base(name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Writing upload failed: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Open(name)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		mpw.WriteField("abort", "1")
		// TODO(quentin): Wait until the abort field is written before exiting.
		os.Exit(1)
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		fmt.Fprintf(os.Stderr, "Writing upload failed: %v", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of %s:
%s [flags] file...
`, os.Args[0], os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("benchsave: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		log.Fatal("no files to upload")
	}

	// TODO(quentin): Some servers might not need authentication.
	// We should somehow detect this and not force the user to get a token.
	// Or they might need non-Google authentication.
	hc := oauth2.NewClient(context.Background(), newTokenSource())

	pr, pw := io.Pipe()
	mpw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mpw.Close()

		for _, name := range files {
			writeOneFile(mpw, name)
		}
	}()

	start := time.Now()

	resp, err := hc.Post(*server+"/upload", mpw.FormDataContentType(), pr)
	if err != nil {
		log.Fatalf("upload failed: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("upload failed: %v\n", resp.Status)
		io.Copy(os.Stderr, resp.Body)
		os.Exit(1)
	}

	status := &uploadStatus{}
	if err := json.NewDecoder(resp.Body).Decode(status); err != nil {
		log.Fatalf("cannot parse upload response: %v\n", err)
	}

	if *verbose {
		s := ""
		if len(files) != 1 {
			s = "s"
		}
		log.Printf("%d file%s uploaded in %.2f seconds.\n", len(files), s, time.Since(start).Seconds())
	}
	if status.ViewURL != "" {
		fmt.Printf("%s\n", status.ViewURL)
	}
	// TODO(quentin): Print benchstat-style output, either computed client-side or fetched from a server.
}
