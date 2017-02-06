// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slice

import (
	"reflect"
	"sort"
	"time"

	"github.com/aclements/go-gg/generic"
)

// CanSort returns whether the value v can be sorted.
func CanSort(v interface{}) bool {
	switch v.(type) {
	case sort.Interface, []time.Time:
		return true
	}
	return generic.CanOrderR(reflect.TypeOf(v).Elem().Kind())
}

// Sort sorts v in increasing order. v must implement sort.Interface,
// be a slice whose elements are orderable, or be a []time.Time.
func Sort(v interface{}) {
	sort.Sort(Sorter(v))
}

// Sorter returns a sort.Interface for sorting v. v must implement
// sort.Interface, be a slice whose elements are orderable, or be a
// []time.Time.
func Sorter(v interface{}) sort.Interface {
	switch v := v.(type) {
	case []int:
		return sort.IntSlice(v)
	case []float64:
		return sort.Float64Slice(v)
	case []string:
		return sort.StringSlice(v)
	case []time.Time:
		return sortTimeSlice(v)
	case sort.Interface:
		return v
	}

	rv := reflectSlice(v)
	switch rv.Type().Elem().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return sortIntSlice{rv}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return sortUintSlice{rv}
	case reflect.Float32, reflect.Float64:
		return sortFloatSlice{rv}
	case reflect.String:
		return sortStringSlice{rv}
	}
	panic(&generic.TypeError{rv.Type().Elem(), nil, "is not orderable"})
}

type sortIntSlice struct {
	reflect.Value
}

func (s sortIntSlice) Len() int {
	return s.Value.Len()
}

func (s sortIntSlice) Less(i, j int) bool {
	return s.Index(i).Int() < s.Index(j).Int()
}

func (s sortIntSlice) Swap(i, j int) {
	a, b := s.Index(i).Int(), s.Index(j).Int()
	s.Index(i).SetInt(b)
	s.Index(j).SetInt(a)
}

type sortUintSlice struct {
	reflect.Value
}

func (s sortUintSlice) Len() int {
	return s.Value.Len()
}

func (s sortUintSlice) Less(i, j int) bool {
	return s.Index(i).Uint() < s.Index(j).Uint()
}

func (s sortUintSlice) Swap(i, j int) {
	a, b := s.Index(i).Uint(), s.Index(j).Uint()
	s.Index(i).SetUint(b)
	s.Index(j).SetUint(a)
}

type sortFloatSlice struct {
	reflect.Value
}

func (s sortFloatSlice) Len() int {
	return s.Value.Len()
}

func (s sortFloatSlice) Less(i, j int) bool {
	return s.Index(i).Float() < s.Index(j).Float()
}

func (s sortFloatSlice) Swap(i, j int) {
	a, b := s.Index(i).Float(), s.Index(j).Float()
	s.Index(i).SetFloat(b)
	s.Index(j).SetFloat(a)
}

type sortStringSlice struct {
	reflect.Value
}

func (s sortStringSlice) Len() int {
	return s.Value.Len()
}

func (s sortStringSlice) Less(i, j int) bool {
	return s.Index(i).String() < s.Index(j).String()
}

func (s sortStringSlice) Swap(i, j int) {
	a, b := s.Index(i).String(), s.Index(j).String()
	s.Index(i).SetString(b)
	s.Index(j).SetString(a)
}

type sortTimeSlice []time.Time

func (s sortTimeSlice) Len() int           { return len(s) }
func (s sortTimeSlice) Less(i, j int) bool { return s[i].Before(s[j]) }
func (s sortTimeSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
