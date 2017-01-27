// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"html/template"
)

var htmlTemplate = template.Must(template.New("").Parse(
	`{{range $i, $table := .}}{{if gt $i 0}}
{{end}}<style>.benchstat tbody td:nth-child(1n+2) { text-align: right; padding: 0em 1em; }</style>
<table class='benchstat'>
{{if .OldNewDelta}}<tr><th>name</th><th>old {{.Metric}}</th><th>new {{.Metric}}</th><th>delta</th>
{{else if eq (len .Configs) 1}}<tr><th>name</th><th>{{.Metric}}</th>
{{else}}<tr><th>name \ {{.Metric}}</th>{{range .Configs}}<th>{{.}}</th>{{end}}
{{end}}{{range $row := $table.Rows}}<tr><td>{{.Benchmark}}</td>{{range $m := .Metrics}}<td>{{$m.Format $row.Scaler}}</td>{{end}}{{if $table.OldNewDelta}}<td>{{.Delta}}</td><td>{{.Note}}</td>{{end}}
{{end -}}
</table>
{{end}}`))

// FormatHTML appends an HTML formatting of the tables to buf.
func FormatHTML(buf *bytes.Buffer, tables []*Table) {
	err := htmlTemplate.Execute(buf, tables)
	if err != nil {
		// Only possible errors here are template not matching data structure.
		// Don't make caller check - it's our fault.
		panic(err)
	}
}
