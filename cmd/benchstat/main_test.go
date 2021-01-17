// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSV(t *testing.T) {
	golden(t, "csvOldNew", "-format", "csv", "old.txt", "new.txt")
	golden(t, "csvErrors", "-format", "csv", "-row", ".name", "new.txt")
}

func TestCRC(t *testing.T) {
	// These have a "note" that "unexpectedly" splits the tables,
	// and also two units.
	golden(t, "crcOldNew", "crc-old.txt", "crc-new.txt")
	// "Fix" the split by note.
	golden(t, "crcIgnore", "-ignore", "note", "crc-old.txt", "crc-new.txt")

	// Filter to aligned, put size on the X axis and poly on the Y axis.
	golden(t, "crcSizeVsPoly", "-filter", "/align:0", "-row", "/size", "-col", "/poly", "crc-new.txt")
}

func TestUnits(t *testing.T) {
	// Test unit metadata. This tests exact assumptions and
	// warnings for inexact distributions.
	golden(t, "units", "-col", "note", "units.txt")
}

func TestZero(t *testing.T) {
	// Test printing of near-zero deltas.
	golden(t, "zero", "-col", "note", "zero.txt")
}

func TestSmallSample(t *testing.T) {
	// These benchmarks don't have enough samples to compute a CI
	// or delta.
	golden(t, "smallSample", "-col", "note", "smallSample.txt")
}

func TestIssue19565(t *testing.T) {
	// Benchmark sets are inconsistent between columns. We show
	// all results, but warn that the geomeans may not be
	// comparable. To further stress things, the columns have the
	// same *number* of benchmarks, but different sets.
	golden(t, "issue19565", "-col", "note", "issue19565.txt")
}

func TestIssue19634(t *testing.T) {
	golden(t, "issue19634", "-col", "note", "issue19634.txt")
}

func golden(t *testing.T, name string, args ...string) {
	t.Helper()
	// TODO: If benchfmt.Files supported fs.FS, we wouldn't need this.
	if err := os.Chdir("testdata"); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir("..")

	// Get the benchstat output.
	var got, gotErr bytes.Buffer
	t.Logf("benchstat %s", strings.Join(args, " "))
	if err := benchstat(&got, &gotErr, args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Compare to the golden output.
	compare(t, name, "stdout", got.Bytes())
	compare(t, name, "stderr", gotErr.Bytes())
}

func compare(t *testing.T, name, sub string, got []byte) {
	t.Helper()

	wantPath := name + "." + sub
	want, err := os.ReadFile(wantPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Treat a missing file as empty.
			want = nil
		} else {
			t.Fatal(err)
		}
	}

	if !diff(t, want, got) {
		return
	}
	// diff printed the error.

	// Write a "got" file for reference.
	gotPath := name + ".got-" + sub
	if err := os.WriteFile(gotPath, got, 0666); err != nil {
		t.Fatalf("error writing %s: %s", gotPath, err)
	}
}

func diff(t *testing.T, want, got []byte) bool {
	t.Helper()
	if bytes.Equal(want, got) {
		return false
	}

	d := t.TempDir()
	wantPath, gotPath := filepath.Join(d, "want"), filepath.Join(d, "got")
	if err := os.WriteFile(wantPath, want, 0666); err != nil {
		t.Fatalf("error writing %s: %s", wantPath, err)
	}
	if err := os.WriteFile(gotPath, got, 0666); err != nil {
		t.Fatalf("error writing %s: %s", gotPath, err)
	}

	cmd := exec.Command("diff", "-Nu", "want", "got")
	cmd.Dir = d
	data, _ := cmd.CombinedOutput()
	if len(data) > 0 {
		t.Errorf("\n%s", data)
	} else {
		// Most likely, "diff not found" so print the bad
		// output so there is something.
		t.Errorf("want:\n%sgot:\n%s", want, got)
	}
	return true
}
