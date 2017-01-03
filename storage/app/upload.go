// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"

	"golang.org/x/net/context"
)

// upload is the handler for the /upload endpoint. It serves a form on
// GET requests and processes files in a multipart/x-form-data POST
// request.
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	// TODO(quentin): Authentication

	if r.Method == http.MethodGet {
		http.ServeFile(w, r, "static/upload.html")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "/upload must be called as a POST request", http.StatusMethodNotAllowed)
		return
	}

	// We use r.MultipartReader instead of r.ParseForm to avoid
	// storing uploaded data in memory.
	mr, err := r.MultipartReader()
	if err != nil {
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	result, err := a.processUpload(ctx, mr)
	if err != nil {
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}
}

// uploadStatus is the response to an /upload POST served as JSON.
type uploadStatus struct {
	// UploadID is the upload ID assigned to the upload.
	UploadID string `json:"uploadid"`
	// FileIDs is the list of file IDs assigned to the files in the upload.
	FileIDs []string `json:"fileids"`
}

// processUpload takes one or more files from a multipart.Reader,
// writes them to the filesystem, and indexes their content.
func (a *App) processUpload(ctx context.Context, mr *multipart.Reader) (*uploadStatus, error) {
	var uploadid string
	var fileids []string

	for i := 0; ; i++ {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}

		name := p.FormName()
		if name != "file" {
			return nil, fmt.Errorf("unexpected field %q", name)
		}

		if uploadid == "" {
			var err error
			uploadid, err = a.DB.ReserveUploadID(ctx)
			if err != nil {
				return nil, err
			}
		}

		// The incoming file needs to be stored in Cloud
		// Storage and it also needs to be indexed. If the file
		// is invalid (contains no valid records) it needs to
		// be rejected and the Cloud Storage upload aborted.

		meta := fileMetadata(ctx, uploadid, i)

		// We need to do two things with the incoming data:
		// - Write it to permanent storage via a.FS
		// - Write index records to a.DB
		// AND if anything fails, attempt to clean up both the
		// FS and the index records.

		if err := a.indexFile(ctx, p, meta); err != nil {
			return nil, err
		}

		fileids = append(fileids, meta["fileid"])
	}

	return &uploadStatus{uploadid, fileids}, nil
}

func (a *App) indexFile(ctx context.Context, p io.Reader, meta map[string]string) (err error) {
	fw, err := a.FS.NewWriter(ctx, fmt.Sprintf("uploads/%s.txt", meta["fileid"]), meta)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			fw.CloseWithError(err)
		} else {
			err = fw.Close()
		}
	}()
	var keys []string
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(fw, "%s: %s\n", k, meta[k]); err != nil {
			return err
		}
	}

	// TODO(quentin): Add a separate goroutine and buffer for writes to fw?
	tr := io.TeeReader(p, fw)
	br := NewBenchmarkReader(tr)
	br.AddLabels(meta)
	i := 0
	for {
		result, err := br.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			if i == 0 {
				return errors.New("no valid benchmark lines found")
			}
			return nil
		}
		i++
		// TODO(quentin): Write records to database
		_ = result
	}
}

// fileMetadata returns the extra metadata fields associated with an
// uploaded file. It obtains the uploader's e-mail address from the
// Context.
func fileMetadata(_ context.Context, uploadid string, filenum int) map[string]string {
	// TODO(quentin): Add the name of the uploader.
	// TODO(quentin): Add the upload time.
	// TODO(quentin): Add other fields?
	return map[string]string{
		"uploadid": uploadid,
		"fileid":   fmt.Sprintf("%s/%d", uploadid, filenum),
	}
}
