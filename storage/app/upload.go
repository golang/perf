// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"encoding/json"
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

	var status uploadStatus

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
			status.UploadID = uploadid
		}

		// The incoming file needs to be stored in Cloud
		// Storage and it also needs to be indexed. If the file
		// is invalid (contains no valid records) it needs to
		// be rejected and the Cloud Storage upload aborted.
		// TODO(quentin): We might as well do these in parallel.

		meta := fileMetadata(ctx, uploadid, i)

		fw, err := a.FS.NewWriter(ctx, fmt.Sprintf("uploads/%s.txt", meta["fileid"]), meta)
		if err != nil {
			return nil, err
		}

		var keys []string
		for k := range meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, err := fmt.Fprintf(fw, "%s: %s\n", k, meta[k]); err != nil {
				fw.CloseWithError(err)
				return nil, err
			}
		}

		if _, err := io.Copy(fw, p); err != nil {
			fw.CloseWithError(err)
			return nil, err
		}
		// TODO(quentin): Write records to database

		if err := fw.Close(); err != nil {
			return nil, err
		}

		status.FileIDs = append(status.FileIDs, meta["fileid"])
	}

	return &status, nil
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
