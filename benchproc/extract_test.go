// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"testing"

	"golang.org/x/perf/benchfmt"
)

func checkNameExtractor(t *testing.T, x extractor, fullName string, want string) {
	t.Helper()
	res := &benchfmt.Result{Name: benchfmt.Name(fullName)}
	got := string(x(res))
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExtractName(t *testing.T) {
	check := checkNameExtractor

	x, err := newExtractor(".name")
	if err != nil {
		t.Fatal(err)
	}
	check(t, x, "Test", "Test")
	check(t, x, "Test/a", "Test")
	check(t, x, "Test-4", "Test")
	check(t, x, "Test/a-4", "Test")
}

func TestExtractFullName(t *testing.T) {
	check := checkNameExtractor

	t.Run("basic", func(t *testing.T) {
		x, err := newExtractor(".fullname")
		if err != nil {
			t.Fatal(err)
		}
		check(t, x, "Test", "Test")
		check(t, x, "Test/a=123", "Test/a=123")
		check(t, x, "Test-2", "Test-2")
	})

	t.Run("excludeA", func(t *testing.T) {
		x := newExtractorFullName([]string{"/a"})
		check(t, x, "Test", "Test")
		check(t, x, "Test/a=123", "Test")
		check(t, x, "Test/b=123/a=123", "Test/b=123")
		check(t, x, "Test/a=123/b=123", "Test/b=123")
		check(t, x, "Test/a=123/a=123", "Test")
		check(t, x, "Test/a=123-2", "Test-2")
	})

	t.Run("excludeName", func(t *testing.T) {
		x := newExtractorFullName([]string{".name"})
		check(t, x, "Test", "*")
		check(t, x, "Test/a=123", "*/a=123")
		x = newExtractorFullName([]string{".name", "/a"})
		check(t, x, "Test", "*")
		check(t, x, "Test/a=123", "*")
		check(t, x, "Test/a=123/b=123", "*/b=123")
	})

	t.Run("excludeGomaxprocs", func(t *testing.T) {
		x := newExtractorFullName([]string{"/gomaxprocs"})
		check(t, x, "Test", "Test")
		check(t, x, "Test/a=123", "Test/a=123")
		check(t, x, "Test/a=123-2", "Test/a=123")
		check(t, x, "Test/gomaxprocs=123", "Test")
	})
}

func TestExtractNameKey(t *testing.T) {
	check := checkNameExtractor

	t.Run("basic", func(t *testing.T) {
		x, err := newExtractor("/a")
		if err != nil {
			t.Fatal(err)
		}
		check(t, x, "Test", "")
		check(t, x, "Test/a=1", "1")
		check(t, x, "Test/aa=1", "")
		check(t, x, "Test/a=1/b=2", "1")
		check(t, x, "Test/b=1/a=2", "2")
		check(t, x, "Test/b=1/a=2-4", "2")
	})

	t.Run("gomaxprocs", func(t *testing.T) {
		x, err := newExtractor("/gomaxprocs")
		if err != nil {
			t.Fatal(err)
		}
		check(t, x, "Test", "")
		check(t, x, "Test/gomaxprocs=4", "4")
		check(t, x, "Test-4", "4")
		check(t, x, "Test/a-4", "4")
	})
}

func TestExtractFileKey(t *testing.T) {
	x, err := newExtractor("file-key")
	if err != nil {
		t.Fatal(err)
	}

	res := r(t, "Name", "file-key", "123", "other-key", "456")
	got := string(x(res))
	want := "123"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	res = r(t, "Name", "other-key", "456")
	got = string(x(res))
	want = ""
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExtractBadKey(t *testing.T) {
	check := func(t *testing.T, got error, want string) {
		t.Helper()
		if got == nil || got.Error() != want {
			t.Errorf("got error %s, want error %s", got, want)
		}
	}
	_, err := newExtractor("")
	check(t, err, "key must not be empty")
}
