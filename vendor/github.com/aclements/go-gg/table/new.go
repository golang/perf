// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"reflect"
	"strconv"

	"github.com/aclements/go-gg/generic"
)

// TableFromStructs converts a []T where T is a struct to a Table
// where the columns of the table correspond to T's exported fields.
func TableFromStructs(structs Slice) *Table {
	s := reflectSlice(structs)
	st := s.Type()
	if st.Elem().Kind() != reflect.Struct {
		panic(&generic.TypeError{st, nil, "is not a slice of struct"})
	}

	var t Builder
	rows := s.Len()
	var rec func(reflect.Type, []int)
	rec = func(typ reflect.Type, index []int) {
		for fn := 0; fn < typ.NumField(); fn++ {
			field := typ.Field(fn)
			if field.PkgPath != "" {
				continue
			}
			oldIndexLen := len(index)
			index = append(index, field.Index...)
			if field.Anonymous {
				rec(field.Type, index)
			} else {
				col := reflect.MakeSlice(reflect.SliceOf(field.Type), rows, rows)
				for i := 0; i < rows; i++ {
					col.Index(i).Set(s.Index(i).FieldByIndex(index))
				}
				t.Add(field.Name, col.Interface())
			}
			index = index[:oldIndexLen]
		}
	}
	rec(st.Elem(), []int{})
	return t.Done()
}

// TableFromStrings converts a [][]string to a Table. This is intended
// for processing external data, such as from CSV files. If coerce is
// true, TableFromStrings will convert columns to []int or []float
// when every string in that column is accepted by strconv.ParseInt or
// strconv.ParseFloat, respectively.
func TableFromStrings(cols []string, rows [][]string, coerce bool) *Table {
	var t Builder
	for i, col := range cols {
		slice := make([]string, len(rows))
		for j, row := range rows {
			slice[j] = row[i]
		}

		var colData interface{} = slice
		switch {
		case coerce && len(slice) > 0:
			// Try []int.
			var err error
			for _, str := range slice {
				_, err = strconv.ParseInt(str, 10, 0)
				if err != nil {
					break
				}
			}
			if err == nil {
				nslice := make([]int, len(rows))
				for i, str := range slice {
					v, _ := strconv.ParseInt(str, 10, 0)
					nslice[i] = int(v)
				}
				colData = nslice
				break
			}

			// Try []float64. This must be done after
			// []int. It's also more expensive.
			for _, str := range slice {
				_, err = strconv.ParseFloat(str, 64)
				if err != nil {
					break
				}
			}
			if err == nil {
				nslice := make([]float64, len(rows))
				for i, str := range slice {
					nslice[i], _ = strconv.ParseFloat(str, 64)
				}
				colData = nslice
				break
			}
		}

		t.Add(col, colData)
	}
	return t.Done()
}
