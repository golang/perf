// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"net/http"

	"golang.org/x/perf/storage/benchfmt"
)

func (a *App) search(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")
	if q == "" {
		http.Error(w, "missing q parameter", 400)
		return
	}

	query := a.DB.Query(q)
	defer query.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	bw := benchfmt.NewPrinter(w)
	for query.Next() {
		if err := bw.Print(query.Result()); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	if err := query.Err(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}
