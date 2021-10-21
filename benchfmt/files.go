// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"fmt"
	"os"
	"strings"
)

// A Files reads benchmark results from a sequence of input files.
//
// This reader adds a ".file" configuration key to the output Results
// corresponding to each path read in. By default, this will be the
// file name directly from Paths, except that duplicate strings will
// be disambiguated by appending "#N". If AllowLabels is true, then
// entries in Path may be of the form label=path, and the label part
// will be used for .file (without any disambiguation).
type Files struct {
	// Paths is the list of file names to read in.
	//
	// If AllowLabels is set, these strings may be of the form
	// label=path, and the label part will be used for the
	// ".file" key in the results.
	Paths []string

	// AllowStdin indicates that the path "-" should be treated as
	// stdin and if the file list is empty, it should be treated
	// as consisting of stdin.
	//
	// This is generally the desired behavior when the file list
	// comes from command-line flags.
	AllowStdin bool

	// AllowLabels indicates that custom labels are allowed in
	// Paths.
	//
	// This is generally the desired behavior when the file list
	// comes from command-line flags, as it allows users to
	// override .file.
	AllowLabels bool

	// inputs is the sequence of remaining inputs, or nil if this
	// Files has not started yet. Note that this distinguishes nil
	// from length 0.
	inputs []input

	reader  Reader
	file    *os.File
	isStdin bool
	err     error
}

type input struct {
	path      string
	label     string
	isStdin   bool
	isLabeled bool
}

// init does first-use initialization of f.
func (f *Files) init() {
	// Set f.inputs to a non-nil slice to indicate initialization
	// has happened.
	f.inputs = []input{}

	// Parse the paths. Doing this first simplifies iteration and
	// disambiguation.
	pathCount := make(map[string]int)
	if f.AllowStdin && len(f.Paths) == 0 {
		f.inputs = append(f.inputs, input{"-", "-", true, false})
	}
	for _, path := range f.Paths {
		// Parse the label.
		label := path
		isLabeled := false
		if i := strings.Index(path, "="); f.AllowLabels && i >= 0 {
			label, path = path[:i], path[i+1:]
			isLabeled = true
		} else {
			pathCount[path]++
		}

		isStdin := f.AllowStdin && path == "-"
		f.inputs = append(f.inputs, input{path, label, isStdin, isLabeled})
	}

	// If the same path is given multiple times, disambiguate its
	// .file. Otherwise, the results have indistinguishable
	// configurations, which just doubles up samples, which is
	// generally not what users are expecting. For overridden
	// labels, we do exactly what the user says.
	pathI := make(map[string]int)
	for i := range f.inputs {
		inp := &f.inputs[i]
		if inp.isLabeled || pathCount[inp.path] == 1 {
			continue
		}
		// Disambiguate.
		inp.label = fmt.Sprintf("%s#%d", inp.path, pathI[inp.path])
		pathI[inp.path]++
	}
}

// Scan advances the reader to the next result in the sequence of
// files and reports whether a result was read. The caller should use
// the Result method to get the result. If Scan reaches the end of the
// file sequence, or if an I/O error occurs, it returns false. In this
// case, the caller should use the Err method to check for errors.
func (f *Files) Scan() bool {
	if f.err != nil {
		return false
	}

	if f.inputs == nil {
		f.init()
	}

	for {
		if f.file == nil {
			// Open the next file.
			if len(f.inputs) == 0 {
				// We're out of inputs.
				return false
			}
			inp := f.inputs[0]
			f.inputs = f.inputs[1:]

			if inp.isStdin {
				f.isStdin, f.file = true, os.Stdin
			} else {
				file, err := os.Open(inp.path)
				if err != nil {
					f.err = err
					return false
				}
				f.isStdin, f.file = false, file
			}

			// Prepare the reader. Because ".file" is not
			// valid syntax for file configuration keys in
			// the file itself, there's no danger of it
			// being overwritten.
			f.reader.Reset(f.file, inp.path, ".file", inp.label)
		}

		// Try to get the next result.
		if f.reader.Scan() {
			return true
		}
		err := f.reader.Err()
		if err != nil {
			f.err = err
			break
		}
		// Just an EOF. Close this file and open the next.
		if !f.isStdin {
			f.file.Close()
		}
		f.file = nil
	}
	// We're out of files.
	return false
}

// Result returns the record that was just read by Scan.
// See Reader.Result.
func (f *Files) Result() Record {
	return f.reader.Result()
}

// Err returns the I/O error that stopped Scan, if any.
// If Scan stopped because it read each file to completion,
// or if Scan has not yet returned false, Err returns nil.
func (f *Files) Err() error {
	return f.err
}

// Units returns the accumulated unit metadata.
// See Reader.Units.
func (f *Files) Units() map[UnitMetadataKey]*UnitMetadata {
	return f.reader.Units()
}
