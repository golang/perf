// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"unicode"
	"unicode/utf8"

	"golang.org/x/perf/benchfmt/internal/bytesconv"
	"golang.org/x/perf/benchunit"
)

// A Reader reads the Go benchmark format.
//
// Its API is modeled on bufio.Scanner. To minimize allocation, a
// Reader retains ownership of everything it creates; a caller should
// copy anything it needs to retain.
//
// To construct a new Reader, either call NewReader, or call Reset on
// a zeroed Reader.
type Reader struct {
	s   *bufio.Scanner
	err error // current I/O error

	// q is the queue of records to return before processing the next
	// input line. qPos is the index of the current record in q. We
	// track the index explicitly rather than slicing q so that we can
	// reuse the q slice when we reach the end.
	q    []Record
	qPos int

	result Result
	units  UnitMetadataMap

	interns map[string]string
}

// A SyntaxError represents a syntax error on a particular line of a
// benchmark results file.
type SyntaxError struct {
	FileName string
	Line     int
	Msg      string
}

func (e *SyntaxError) Pos() (fileName string, line int) {
	return e.FileName, e.Line
}

func (s *SyntaxError) Error() string {
	return fmt.Sprintf("%s:%d: %s", s.FileName, s.Line, s.Msg)
}

var noResult = &SyntaxError{"", 0, "Reader.Scan has not been called"}
var errSkip = &SyntaxError{"", 0, "skip line"}

// NewReader constructs a reader to parse the Go benchmark format from r.
// fileName is used in error messages; it is purely diagnostic.
func NewReader(r io.Reader, fileName string) *Reader {
	reader := new(Reader)
	reader.Reset(r, fileName)
	return reader
}

// newSyntaxError returns a *SyntaxError at the Reader's current position.
func (r *Reader) newSyntaxError(msg string) *SyntaxError {
	return &SyntaxError{r.result.fileName, r.result.line, msg}
}

// Reset resets the reader to begin reading from a new input.
// It also resets all accumulated configuration values.
// It does NOT reset unit metadata because it carries across files.
//
// initConfig is an alternating sequence of keys and values.
// Reset will install these as the initial internal configuration
// before any results are read from the input file.
func (r *Reader) Reset(ior io.Reader, fileName string, initConfig ...string) {
	r.s = bufio.NewScanner(ior)
	if fileName == "" {
		fileName = "<unknown>"
	}
	r.err = nil
	if r.interns == nil {
		r.interns = make(map[string]string)
	}
	if r.units == nil {
		r.units = make(map[UnitMetadataKey]*UnitMetadata)
	}

	// Wipe the queue in case the user hasn't consumed everything from
	// this file.
	r.qPos = 0
	r.q = r.q[:0]

	// Wipe the Result.
	r.result.Config = r.result.Config[:0]
	r.result.Name = r.result.Name[:0]
	r.result.Iters = 0
	r.result.Values = r.result.Values[:0]
	for k := range r.result.configPos {
		delete(r.result.configPos, k)
	}
	r.result.fileName = fileName
	r.result.line = 0

	// Set up initial configuration.
	if len(initConfig)%2 != 0 {
		panic("len(initConfig) must be a multiple of 2")
	}
	for i := 0; i < len(initConfig); i += 2 {
		r.result.SetConfig(initConfig[i], initConfig[i+1])
	}
}

var (
	benchmarkPrefix = []byte("Benchmark")
	unitPrefix      = []byte("Unit")
)

// Scan advances the reader to the next result and reports whether a
// result was read.
// The caller should use the Result method to get the result.
// If Scan reaches EOF or an I/O error occurs, it returns false,
// in which case the caller should use the Err method to check for errors.
func (r *Reader) Scan() bool {
	if r.err != nil {
		return false
	}

	// If there's anything in the queue from an earlier line, just pop
	// the queue and return without consuming any more input.
	if r.qPos+1 < len(r.q) {
		r.qPos++
		return true
	}
	// Otherwise, we've drained the queue and need to parse more input
	// to refill it. Reset it to 0 so we can reuse the space.
	r.qPos = 0
	r.q = r.q[:0]

	// Process lines until we add something to the queue or hit EOF.
	for len(r.q) == 0 && r.s.Scan() {
		r.result.line++
		// We do everything in byte buffers to avoid allocation.
		line := r.s.Bytes()
		// Most lines are benchmark lines, and we can check
		// for that very quickly, so start with that.
		if bytes.HasPrefix(line, benchmarkPrefix) {
			if err := r.parseBenchmarkLine(line); err == nil {
				r.q = append(r.q, &r.result)
			} else if err != errSkip {
				r.q = append(r.q, err)
			}
			continue
		}
		if len(line) > 0 && line[0] == 'U' {
			if nLine, ok := r.isUnitLine(line); ok {
				// Parse unit metadata line. This queues up its own
				// records and errors.
				r.parseUnitLine(nLine)
				continue
			}
		}
		if key, val, ok := parseKeyValueLine(line); ok {
			// Intern key, since there tend to be few
			// unique keys.
			keyStr := r.intern(key)
			if len(val) == 0 {
				r.result.deleteConfig(keyStr)
			} else {
				cfg := r.result.ensureConfig(keyStr, true)
				cfg.Value = append(cfg.Value[:0], val...)
			}
			continue
		}
		// Ignore the line.
	}

	if len(r.q) > 0 {
		// We queued something up to return.
		return true
	}

	// We hit EOF. Check for IO errors.
	if err := r.s.Err(); err != nil {
		r.err = fmt.Errorf("%s:%d: %w", r.result.fileName, r.result.line, err)
		return false
	}
	r.err = nil
	return false
}

// parseKeyValueLine attempts to parse line as a key: val pair,
// with ok reporting whether the line could be parsed.
func parseKeyValueLine(line []byte) (key, val []byte, ok bool) {
	for i := 0; i < len(line); {
		r, n := utf8.DecodeRune(line[i:])
		// key begins with a lower case character ...
		if i == 0 && !unicode.IsLower(r) {
			return
		}
		// and contains no space characters nor upper case
		// characters.
		if unicode.IsSpace(r) || unicode.IsUpper(r) {
			return
		}
		if i > 0 && r == ':' {
			key, val = line[:i], line[i+1:]
			break
		}

		i += n
	}
	if len(key) == 0 {
		return
	}
	// Value can be omitted entirely, in which case the colon must
	// still be present, but need not be followed by a space.
	if len(val) == 0 {
		ok = true
		return
	}
	// One or more ASCII space or tab characters separate "key:"
	// from "value."
	for len(val) > 0 && (val[0] == ' ' || val[0] == '\t') {
		val = val[1:]
		ok = true
	}
	return
}

// parseBenchmarkLine parses line as a benchmark result and updates r.result.
// The caller must have already checked that line begins with "Benchmark".
func (r *Reader) parseBenchmarkLine(line []byte) *SyntaxError {
	var f []byte
	var err error

	// Skip "Benchmark"
	line = line[len("Benchmark"):]

	// Read the name.
	lineLen := len(line)
	r.result.Name, line = splitField(line)

	// As a special case, if the name is the entire line, we
	// ignore it. This happens in "go test -v" output, which
	// prints the benchmark name immediately followed by a newline
	// when the benchmark starts.
	if len(line) == 0 && len(r.result.Name) == lineLen {
		return errSkip
	}

	// Read the iteration count.
	f, line = splitField(line)
	if len(f) == 0 {
		return r.newSyntaxError("missing iteration count")
	}
	r.result.Iters, err = bytesconv.Atoi(f)
	switch err := err.(type) {
	case nil:
		// ok
	case *bytesconv.NumError:
		return r.newSyntaxError("parsing iteration count: " + err.Err.Error())
	default:
		return r.newSyntaxError(err.Error())
	}

	// Read value/unit pairs.
	r.result.Values = r.result.Values[:0]
	for {
		f, line = splitField(line)
		if len(f) == 0 {
			if len(r.result.Values) > 0 {
				break
			}
			return r.newSyntaxError("missing measurements")
		}
		val, err := atof(f)
		switch err := err.(type) {
		case nil:
			// ok
		case *bytesconv.NumError:
			return r.newSyntaxError("parsing measurement: " + err.Err.Error())
		default:
			return r.newSyntaxError(err.Error())
		}
		f, line = splitField(line)
		if len(f) == 0 {
			return r.newSyntaxError("missing units")
		}
		unit := r.intern(f)

		// Tidy the value.
		tidyVal, tidyUnit := benchunit.Tidy(val, unit)
		var v Value
		if tidyVal == val {
			v = Value{Value: val, Unit: unit}
		} else {
			v = Value{Value: tidyVal, Unit: tidyUnit, OrigValue: val, OrigUnit: unit}
		}

		r.result.Values = append(r.result.Values, v)
	}

	return nil
}

// isUnitLine tests whether line is a unit metadata line. If it is, it
// returns the line after the "Unit" literal and true.
func (r *Reader) isUnitLine(line []byte) (rest []byte, ok bool) {
	var f []byte
	// Is this a unit metadata line?
	f, line = splitField(line)
	if bytes.Equal(f, unitPrefix) {
		return line, true
	}
	return nil, false
}

// parseUnitLine parses line as a unit metadata line, starting
// after "Unit". It updates r.q.
// If there are syntax errors on the line, it will attempt to parse
// what it can and return a non-nil error.
func (r *Reader) parseUnitLine(line []byte) {
	var f []byte
	// isUnitLine already consumed the literal "Unit".
	// Consume the next field, which is the unit.
	f, line = splitField(line)
	if len(f) == 0 {
		r.q = append(r.q, r.newSyntaxError("missing unit"))
		return
	}
	unit := r.intern(f)

	// The metadata map is indexed by tidied units because we want to
	// support lookups by tidy units and there's no way to "untidy" a
	// unit.
	_, tidyUnit := benchunit.Tidy(1, unit)

	// Consume key=value pairs.
	for {
		f, line = splitField(line)
		if len(f) == 0 {
			break
		}
		eq := bytes.IndexByte(f, '=')
		if eq <= 0 {
			r.q = append(r.q, r.newSyntaxError("expected key=value"))
			continue
		}
		key := UnitMetadataKey{tidyUnit, r.intern(f[:eq])}
		value := r.intern(f[eq+1:])

		if have, ok := r.units[key]; ok {
			if have.Value == value {
				// We already have this unit metadata. Ignore.
				continue
			}
			// Report incompatible unit metadata.
			r.q = append(r.q, r.newSyntaxError(fmt.Sprintf("metadata %s of unit %s already set to %s", key.Key, unit, have.Value)))
			continue
		}

		metadata := &UnitMetadata{key, unit, value, r.result.fileName, r.result.line}
		r.units[key] = metadata
		r.q = append(r.q, metadata)
	}
}

func (r *Reader) intern(x []byte) string {
	const maxIntern = 1024
	if s, ok := r.interns[string(x)]; ok {
		return s
	}
	if len(r.interns) >= maxIntern {
		// Evict a random item from the interns table.
		// Map iteration order is unspecified, but both
		// the gc and libgo runtimes both provide random
		// iteration order. The choice of item to evict doesn't
		// affect correctness, so we do the simple thing.
		for k := range r.interns {
			delete(r.interns, k)
			break
		}
	}
	s := string(x)
	r.interns[s] = s
	return s
}

// A Record is a single record read from a benchmark file. It may be a
// *Result or a *SyntaxError.
type Record interface {
	// Pos returns the position of this record as a file name and a
	// 1-based line number within that file. If this record was not read
	// from a file, it returns "", 0.
	Pos() (fileName string, line int)
}

var _ Record = (*Result)(nil)
var _ Record = (*SyntaxError)(nil)
var _ Record = (*UnitMetadata)(nil)

// Result returns the record that was just read by Scan. This is either
// a *Result, a *UnitMetadata, or a *SyntaxError indicating a parse error.
// It may return more types in the future.
//
// Parse errors are non-fatal, so the caller can continue to call
// Scan.
//
// If this returns a *Result, the caller should not retain the Result,
// as it will be overwritten by the next call to Scan.
func (r *Reader) Result() Record {
	if r.qPos >= len(r.q) {
		// This should only happen if Scan has never been called.
		return noResult
	}
	return r.q[r.qPos]
}

// Err returns the first non-EOF I/O error that was encountered by the
// Reader.
func (r *Reader) Err() error {
	return r.err
}

// Units returns the accumulated unit metadata.
//
// Callers that want to consume the entire stream of benchmark results
// and then process units can use this instead of monitoring
// *UnitMetadata Records.
func (r *Reader) Units() UnitMetadataMap {
	return r.units
}

// Parsing helpers.
//
// These are designed to leverage common fast paths. The ASCII fast
// path is especially important, and more than doubles the performance
// of the parser.

// atof is a wrapper for bytesconv.ParseFloat that optimizes for
// numbers that are usually integers.
func atof(x []byte) (float64, error) {
	// Try parsing as an integer.
	var val int64
	for _, ch := range x {
		digit := ch - '0'
		if digit >= 10 {
			goto fail
		}
		if val > (math.MaxInt64-10)/10 {
			goto fail // avoid int64 overflow
		}
		val = (val * 10) + int64(digit)
	}
	return float64(val), nil

fail:
	// The fast path failed. Parse it as a float.
	return bytesconv.ParseFloat(x, 64)
}

const isSpace uint64 = 1<<'\t' | 1<<'\n' | 1<<'\v' | 1<<'\f' | 1<<'\r' | 1<<' '

// splitField consumes and returns non-whitespace in x as field,
// consumes whitespace following the field, and then returns the
// remaining bytes of x.
func splitField(x []byte) (field, rest []byte) {
	// Collect non-whitespace into field.
	var i int
	for i = 0; i < len(x); {
		if x[i] < utf8.RuneSelf {
			// Fast path for ASCII
			if (isSpace>>x[i])&1 != 0 {
				rest = x[i+1:]
				break

			}
			i++
		} else {
			// Slow path for Unicode
			r, n := utf8.DecodeRune(x[i:])
			if unicode.IsSpace(r) {
				rest = x[i+n:]
				break
			}
			i += n
		}
	}
	field = x[:i]

	// Strip whitespace from rest.
	for len(rest) > 0 {
		if rest[0] < utf8.RuneSelf {
			if (isSpace>>rest[0])&1 == 0 {
				break
			}
			rest = rest[1:]
		} else {
			r, n := utf8.DecodeRune(rest)
			if !unicode.IsSpace(r) {
				break
			}
			rest = rest[n:]
		}
	}
	return
}
