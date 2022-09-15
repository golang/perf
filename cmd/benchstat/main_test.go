// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func dotSlash(s string) string {
	return "." + string(filepath.Separator) + s
}

func TestGolden(t *testing.T) {
	if err := os.Chdir("testdata"); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir("..")
	check(t, "exampleold", "exampleold.txt")
	check(t, "example", "exampleold.txt", "examplenew.txt")
	if t.Failed() {
		t.Fatal("skipping other tests")
	}
	check(t, "exampleoldhtml", "-html", "exampleold.txt")
	check(t, "examplehtml", "-html", "exampleold.txt", "examplenew.txt")
	if t.Failed() {
		t.Fatal("skipping other tests")
	}
	check(t, "all", "new.txt", "old.txt", "slashslash4.txt", "x386.txt")
	check(t, "allnosplit", "-split", "", "new.txt", "old.txt", "slashslash4.txt", "x386.txt")
	check(t, "oldnew", "old.txt", "new.txt")
	check(t, "oldnewgeo", "-geomean", "old.txt", "new.txt")
	check(t, "new4", "new.txt", "slashslash4.txt")
	check(t, "oldnewhtml", "-html", "old.txt", "new.txt")
	check(t, "oldnew4html", "-html", "old.txt", "new.txt", "slashslash4.txt")
	check(t, "oldnewttest", "-delta-test=ttest", "old.txt", "new.txt")
	check(t, "packagesold", "packagesold.txt")
	check(t, "packages", "packagesold.txt", "packagesnew.txt")
	check(t, "units", "units-old.txt", "units-new.txt")
	check(t, "zero", "-delta-test=none", "zero-old.txt", "zero-new.txt")
	check(t, "namesort", "-sort=name", "old.txt", "new.txt")
	check(t, "deltasort", "-sort=delta", "old.txt", "new.txt")
	check(t, "rdeltasort", "-sort=-delta", "old.txt", "new.txt")

	check(t, "oldcsv", "-geomean", "-csv", "old.txt")
	check(t, "allcsv", "-geomean", "-csv", "old.txt", "new.txt", "slashslash4.txt", "x386.txt")
	check(t, "allnosplitcsv", "-geomean", "-csv", "-split", "", dotSlash("new.txt"), dotSlash("old.txt"), dotSlash("slashslash4.txt"), dotSlash("x386.txt")) // note order: new old slashslash4 x386; dotSlash tests common prefix removal

	if dotSlash("x") == "./x" { // Golden files have hardcoded "/" path separators in them
		check(t, "allnorangecsv", "-geomean", "-csv", "-norange", dotSlash("old.txt"), dotSlash("new.txt"), "slashslash4.txt", "x386.txt") // Mixed ./ tests common prefix non-removal
	}

}

func check(t *testing.T, name string, files ...string) {
	t.Run(name, func(t *testing.T) {
		os.Args = append([]string{"benchstat"}, files...)
		t.Logf("running %v", os.Args)
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		c := make(chan []byte)
		go func() {
			data, err := io.ReadAll(r)
			if err != nil {
				t.Error(err)
			}
			c <- data
		}()
		stdout := os.Stdout
		stderr := os.Stderr
		os.Stdout = w
		os.Stderr = w
		exit = func(code int) { t.Fatalf("exit %d during main", code) }
		*flagGeomean = false
		*flagHTML = false
		*flagNoRange = false
		*flagDeltaTest = "utest"
		*flagSplit = flag.Lookup("split").DefValue

		main()

		w.Close()
		os.Stdout = stdout
		os.Stderr = stderr
		exit = os.Exit

		data := <-c
		golden, err := os.ReadFile(name + ".golden")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, golden) {
			t.Errorf("wrong output: diff have want:\n%s", diff(t, data, golden))
		}
	})
}

// diff returns the output of 'diff -u old new'.
func diff(t *testing.T, old, new []byte) string {
	data, err := exec.Command("diff", "-u", writeTemp(t, old), writeTemp(t, new)).CombinedOutput()
	if len(data) > 0 {
		return string(data)
	}
	// Most likely, "diff not found" so print the bad output so there is something.
	return "ERROR: " + err.Error() + ": test output = \n" + string(old)
}

func writeTemp(t *testing.T, data []byte) string {
	f, err := os.CreateTemp("", "benchstat_test")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	name := f.Name()
	f.Close()
	return name
}
