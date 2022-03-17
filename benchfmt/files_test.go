// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestFiles(t *testing.T) {
	// Switch to testdata/files directory.
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir("testdata/files"); err != nil {
		t.Fatal(err)
	}

	check := func(f *Files, want ...string) {
		t.Helper()
		for f.Scan() {
			switch res := f.Result(); res := res.(type) {
			default:
				t.Fatalf("unexpected result type %T", res)
			case *SyntaxError:
				t.Fatalf("unexpected Result error %s", res)
				return
			case *Result:
				if len(want) == 0 {
					t.Errorf("got result, want end of stream")
					return
				}
				got := res.GetConfig(".file") + " " + string(res.Name.Full())
				if got != want[0] {
					t.Errorf("got %q, want %q", got, want[0])
				}
				want = want[1:]
			}
		}

		err := f.Err()
		noent := errors.Is(err, fs.ErrNotExist)
		wantNoent := len(want) == 1 && strings.HasPrefix(want[0], "ErrNotExist")
		if wantNoent {
			want = want[1:]
		}
		if err != nil && !noent {
			t.Errorf("got unexpected error %s", err)
		} else if noent && !wantNoent {
			t.Errorf("got %s, want success", err)
		} else if !noent && wantNoent {
			t.Errorf("got success, want ErrNotExist")
		}

		if len(want) != 0 {
			t.Errorf("got end of stream, want %v", want)
		}
	}

	// Basic tests.
	check(
		&Files{Paths: []string{"a", "b"}},
		"a X", "a Y", "b Z",
	)
	check(
		&Files{Paths: []string{"a", "b", "c", "d"}},
		"a X", "a Y", "b Z", "ErrNotExist",
	)

	// Ambiguous paths.
	check(
		&Files{Paths: []string{"a", "b", "a"}},
		"a#0 X", "a#0 Y", "b Z", "a#1 X", "a#1 Y",
	)

	// AllowStdin.
	check(
		&Files{Paths: []string{"-"}},
		"ErrNotExist",
	)
	fakeStdin("BenchmarkIn 1 1 ns/op\n", func() {
		check(
			&Files{
				Paths:      []string{"-"},
				AllowStdin: true,
			},
			"- In",
		)
	})

	// Labels.
	check(
		&Files{
			Paths:       []string{"a", "b"},
			AllowLabels: true,
		},
		"a X", "a Y", "b Z",
	)
	check(
		&Files{
			Paths:       []string{"foo=a", "b"},
			AllowLabels: true,
		},
		"foo X", "foo Y", "b Z",
	)
	fakeStdin("BenchmarkIn 1 1 ns/op\n", func() {
		check(
			&Files{
				Paths:       []string{"foo=-"},
				AllowStdin:  true,
				AllowLabels: true,
			},
			"foo In",
		)
	})

	// Ambiguous labels don't get disambiguated.
	check(
		&Files{
			Paths:       []string{"foo=a", "foo=a"},
			AllowLabels: true,
		},
		"foo X", "foo Y", "foo X", "foo Y",
	)
}

func fakeStdin(content string, cb func()) {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	go func() {
		defer w.Close()
		w.WriteString(content)
	}()
	defer r.Close()
	defer func(orig *os.File) { os.Stdin = orig }(os.Stdin)
	os.Stdin = r
	cb()
}
