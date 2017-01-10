// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package app implements the performance data storage server. Combine
// an App with a database and filesystem to get an HTTP server.
package app

import (
	"errors"
	"net/http"

	"golang.org/x/perf/storage/db"
	"golang.org/x/perf/storage/fs"
)

// App manages the storage server logic. Construct an App instance
// using a literal with DB and FS objects and call RegisterOnMux to
// connect it with an HTTP server.
type App struct {
	DB *db.DB
	FS fs.FS

	// Auth obtains the username for the request.
	// If necessary, it can write its own response (e.g. a
	// redirect) and return ErrResponseWritten.
	Auth func(http.ResponseWriter, *http.Request) (string, error)
}

// ErrResponseWritten can be returned by App.Auth to abort the normal /upload handling.
var ErrResponseWritten = errors.New("response written")

// RegisterOnMux registers the app's URLs on mux.
func (a *App) RegisterOnMux(mux *http.ServeMux) {
	// TODO(quentin): Should we just make the App itself be an http.Handler?
	mux.HandleFunc("/upload", a.upload)
	mux.HandleFunc("/search", a.search)
}
