// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"html/template"
)

var htmlTemplate = template.Must(template.New("").Parse(`
{{with index . 0}}
<table class='benchstat {{if .OldNewDelta}}oldnew{{end}}'>
{{if .OldNewDelta -}}
{{- else if eq (len .Configs) 1}}
{{- else -}}
<tr class='configs'><th>{{range .Configs}}<th>{{.}}{{end}}
{{end}}
{{end}}
{{- range $i, $table := .}}
<tbody>
{{if .OldNewDelta -}}
<tr><th><th>old {{.Metric}}<th>new {{.Metric}}<th>delta<th>
{{else if eq (len .Configs) 1}}
<tr><th><th>{{.Metric}}
{{else -}}
<tr><th><th colspan='{{len .Configs}}' class='metric'>{{.Metric}}
{{end}}{{range $row := $table.Rows -}}
{{if $table.OldNewDelta -}}
<tr class='{{if eq .Change 1}}better{{else if eq .Change -1}}worse{{else}}unchanged{{end}}'>
{{- else -}}
<tr>
{{- end -}}
<td>{{.Benchmark}}{{range .Metrics}}<td>{{.Format $row.Scaler}}{{end}}{{if $table.OldNewDelta}}<td{{if eq .Delta "~"}} class='nodelta'{{end}}>{{.Delta}}<td class='note'>{{.Note}}{{end}}
{{end -}}
<tr><td>&nbsp;
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
