// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Benchsave uploads benchmark results to a storage server.
//
// Usage:
//
//	benchsave [-v] [-header file] [-server url] file...
//
// Each input file should contain the output from one or more runs of
// ``go test -bench'', or another tool which uses the same format.
//
// Benchsave will upload the input files to the specified server and
// print a URL where they can be viewed.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

var (
	server  = flag.String("server", "https://perfdata.golang.org", "upload benchmarks to server at `url`")
	verbose = flag.Bool("v", false, "print verbose log messages")
	header  = flag.String("header", "", "insert `file` at the beginning of each uploaded file")
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
func writeOneFile(mpw *multipart.Writer, name string, header []byte) error {
	w, err := mpw.CreateFormFile("file", filepath.Base(name))
	if err != nil {
		return err
	}
	if len(header) > 0 {
		if _, err := w.Write(header); err != nil {
			return err
		}
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of benchsave:
	benchsave [flags] file...
`)
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

	var headerData []byte
	if *header != "" {
		var err error
		headerData, err = ioutil.ReadFile(*header)
		if err != nil {
			log.Fatal(err)
		}
		headerData = append(bytes.TrimRight(headerData, "\n"), '\n', '\n')
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
			if err := writeOneFile(mpw, name, headerData); err != nil {
				log.Print(err)
				mpw.WriteField("abort", "1")
				// Writing the 'abort' field will cause the server to send back an error response,
				// which will cause the main goroutine to  below.
				return
			}
		}

		mpw.WriteField("commit", "1")
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
