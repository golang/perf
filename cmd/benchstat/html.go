// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"html/template"
)

var htmlTemplate = template.Must(template.New("").Parse(`
<table class='benchstat'>
{{- range $i, $table := .}}
<tbody {{if .OldNewDelta}}class='oldnew'{{end}}>
{{if .OldNewDelta -}}
<tr><th>name<th>old {{.Metric}}<th>new {{.Metric}}<th>delta
{{else if eq (len .Configs) 1}}
<tr><th>name<th>{{.Metric}}
{{else -}}
<tr><th>name \ {{.Metric}}{{range .Configs}}<th>{{.}}{{end}}
{{end}}{{range $row := $table.Rows -}}
{{if $table.OldNewDelta -}}
<tr class='{{if eq .Change 1}}better{{else if eq .Change -1}}worse{{else}}unchanged{{end}}'>
{{- else -}}
<tr>
{{- end -}}
<td>{{.Benchmark}}{{range .Metrics}}<td>{{.Format $row.Scaler}}{{end}}{{if $table.OldNewDelta}}<td>{{.Delta}}<td class='note'>{{.Note}}{{end}}
{{end -}}
</tbody>
{{end}}
</table>
`))

// FormatHTML appends an HTML formatting of the tables to buf.
func FormatHTML(buf *bytes.Buffer, tables []*Table) {
	err := htmlTemplate.Execute(buf, tables)
	if err != nil {
		// Only possible errors here are template not matching data structure.
		// Don't make caller check - it's our fault.
		panic(err)
	}
}
