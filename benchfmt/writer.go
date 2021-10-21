// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchfmt

import (
	"bytes"
	"fmt"
	"io"
)

// A Writer writes the Go benchmark format.
type Writer struct {
	w   io.Writer
	buf bytes.Buffer

	first      bool
	fileConfig map[string]Config
	order      []string
}

// NewWriter returns a writer that writes Go benchmark results to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w, first: true, fileConfig: make(map[string]Config)}
}

// Write writes Record rec to w. If rec is a *Result and rec's file
// configuration differs from the current file configuration in w, it
// first emits the appropriate file configuration lines. For
// Result.Values that have a non-zero OrigUnit, this uses OrigValue and
// OrigUnit in order to better reproduce the original input.
func (w *Writer) Write(rec Record) error {
	switch rec := rec.(type) {
	case *Result:
		w.writeResult(rec)
	case *UnitMetadata:
		w.writeUnitMetadata(rec)
	case *SyntaxError:
		// Ignore
		return nil
	default:
		return fmt.Errorf("unknown Record type %T", rec)
	}

	// Flush the buffer out to the io.Writer. Write to the buffer
	// can't fail, so we only have to check if this fails.
	_, err := w.w.Write(w.buf.Bytes())
	w.buf.Reset()
	return err
}

func (w *Writer) writeResult(res *Result) {
	// If any file config changed, write out the changes.
	if len(w.fileConfig) != len(res.Config) {
		w.writeFileConfig(res)
	} else {
		for _, cfg := range res.Config {
			if have, ok := w.fileConfig[cfg.Key]; !ok || !bytes.Equal(cfg.Value, have.Value) || cfg.File != have.File {
				w.writeFileConfig(res)
				break
			}
		}
	}

	// Print the benchmark line.
	fmt.Fprintf(&w.buf, "Benchmark%s %d", res.Name, res.Iters)
	for _, val := range res.Values {
		if val.OrigUnit == "" {
			fmt.Fprintf(&w.buf, " %v %s", val.Value, val.Unit)
		} else {
			fmt.Fprintf(&w.buf, " %v %s", val.OrigValue, val.OrigUnit)
		}
	}
	w.buf.WriteByte('\n')

	w.first = false
}

func (w *Writer) writeFileConfig(res *Result) {
	if !w.first {
		// Configuration blocks after results get an extra blank.
		w.buf.WriteByte('\n')
		w.first = true
	}

	// Walk keys we know to find changes and deletions.
	for i := 0; i < len(w.order); i++ {
		key := w.order[i]
		have := w.fileConfig[key]
		idx, ok := res.ConfigIndex(key)
		if !ok {
			// Key was deleted.
			fmt.Fprintf(&w.buf, "%s:\n", key)
			delete(w.fileConfig, key)
			copy(w.order[i:], w.order[i+1:])
			w.order = w.order[:len(w.order)-1]
			i--
			continue
		}
		cfg := &res.Config[idx]
		if bytes.Equal(have.Value, cfg.Value) && have.File == cfg.File {
			// Value did not change.
			continue
		}
		// Value changed.
		if cfg.File {
			// Omit internal config.
			fmt.Fprintf(&w.buf, "%s: %s\n", key, cfg.Value)
		}
		have.Value = append(have.Value[:0], cfg.Value...)
		have.File = cfg.File
		w.fileConfig[key] = have
	}

	// Find new keys.
	if len(w.fileConfig) != len(res.Config) {
		for _, cfg := range res.Config {
			if _, ok := w.fileConfig[cfg.Key]; ok {
				continue
			}
			// New key.
			if cfg.File {
				fmt.Fprintf(&w.buf, "%s: %s\n", cfg.Key, cfg.Value)
			}
			w.fileConfig[cfg.Key] = Config{cfg.Key, append([]byte(nil), cfg.Value...), cfg.File}
			w.order = append(w.order, cfg.Key)
		}
	}

	w.buf.WriteByte('\n')
}

func (w *Writer) writeUnitMetadata(m *UnitMetadata) {
	fmt.Fprintf(&w.buf, "Unit %s %s=%s\n", m.OrigUnit, m.Key, m.Value)
}
