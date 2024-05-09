// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"fmt"
	"hash/maphash"
	"strings"
	"sync"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchproc/internal/parse"
)

// TODO: If we support comparison operators in filter expressions,
// does it make sense to unify the orders understood by projections
// with the comparison orders supported in filters? One danger is that
// the default order for projections is observation order, but if you
// filter on key<val, you probably want that to be numeric by default
// (it's not clear you ever want a comparison on observation order).

// A ProjectionParser parses one or more related projection expressions.
type ProjectionParser struct {
	configKeys   map[string]bool // Specific .config keys (excluded from .config)
	fullnameKeys []string        // Specific sub-name keys (excluded from .fullname)
	haveConfig   bool            // .config was projected
	haveFullname bool            // .fullname was projected

	// Fields below here are constructed when the first Result is
	// processed.

	fullExtractor extractor
}

// Parse parses a single projection expression, such as ".name,/size".
// A projection expression describes how to extract fields of a
// benchfmt.Result into a Key and how to order the resulting Keys. See
// "go doc golang.org/x/perf/benchproc/syntax" for a description of
// projection syntax.
//
// A projection expression may also imply a filter, for example if
// there's a fixed order like "/size@(1MiB)". Parse will add any filters
// to "filter".
//
// If an application calls Parse multiple times on the same
// ProjectionParser, these form a mutually-exclusive group of
// projections in which specific keys in any projection are excluded
// from group keys in any other projection. The group keys are
// ".config" and ".fullname". For example, given two projections
// ".config" and "commit,date", the specific file configuration keys
// "commit" and "date" are excluded from the group key ".config".
// The result is the same regardless of the order these expressions
// are parsed in.
func (p *ProjectionParser) Parse(projection string, filter *Filter) (*Projection, error) {
	if p.configKeys == nil {
		p.configKeys = make(map[string]bool)
	}

	proj := newProjection()

	// Parse the projection.
	parts, err := parse.ParseProjection(projection)
	if err != nil {
		return nil, err
	}
	var filterParts []filterFn
	for _, part := range parts {
		f, err := p.makeProjection(proj, projection, part)
		if err != nil {
			return nil, err
		}
		if f != nil {
			filterParts = append(filterParts, f)
		}
	}
	// Now that we've ensured the projection is valid, add any
	// filter parts to the filter.
	if len(filterParts) > 0 {
		if filter == nil {
			panic(fmt.Sprintf("projection expression %s contains a filter, but Parse was passed a nil *Filter", projection))
		}
		filterParts = append(filterParts, filter.match)
		filter.match = filterOp(parse.OpAnd, filterParts)
	}

	return proj, nil
}

// ParseWithUnit is like Parse, but the returned Projection has an
// additional field called ".unit" that extracts the unit of each
// individual benchfmt.Value in a benchfmt.Result. It returns the
// Projection and the ".unit" Field.
//
// Typically, callers need to break out individual benchmark values on
// some dimension of a set of Projections. Adding a .unit field makes
// this easy.
//
// Callers should use the ProjectValues method of the returned
// Projection rather than the Project method to project each value
// rather than the whole benchfmt.Result.
func (p *ProjectionParser) ParseWithUnit(projection string, filter *Filter) (*Projection, *Field, error) {
	proj, err := p.Parse(projection, filter)
	if err != nil {
		return nil, nil, err
	}
	field := proj.addField(proj.root, ".unit")
	field.order = make(map[string]int)
	field.cmp = func(a, b string) int {
		return field.order[a] - field.order[b]
	}
	proj.unitField = field
	return proj, field, nil
}

// Residue returns a projection for any field not yet projected by any
// projection parsed by p. The resulting Projection does not have a
// meaningful order.
//
// For example, following calls to p.Parse("goos") and
// p.Parse(".fullname"), Reside would return a Projection with fields
// for all file configuration fields except goos.
//
// The intended use of this is to report when a user may have
// over-aggregated results. Specifically, track the residues of all of
// the benchfmt.Results that are aggregated together (e.g., into a
// single table cell). If there's more than one distinct residue, report
// that those results differed in some field. Typically this is used
// with NonSingularFields to report exactly which fields differ.
func (p *ProjectionParser) Residue() *Projection {
	s := newProjection()

	// The .config and .fullname groups together cover the
	// projection space. If they haven't already been specified,
	// then these groups (with any specific keys excluded) exactly
	// form the remainder.
	if !p.haveConfig {
		p.makeProjection(s, "", parse.Field{Key: ".config", Order: "first"})
	}
	if !p.haveFullname {
		p.makeProjection(s, "", parse.Field{Key: ".fullname", Order: "first"})
	}

	return s
}

func (p *ProjectionParser) makeProjection(s *Projection, q string, proj parse.Field) (filterFn, error) {
	// Construct the order function.
	var initField func(field *Field)
	var filter filterFn
	makeFilter := func(ext extractor) {}
	if proj.Order == "fixed" {
		fixedMap := make(map[string]int, len(proj.Fixed))
		for i, s := range proj.Fixed {
			fixedMap[s] = i
		}
		initField = func(field *Field) {
			field.cmp = func(a, b string) int {
				return fixedMap[a] - fixedMap[b]
			}
		}
		makeFilter = func(ext extractor) {
			filter = func(res *benchfmt.Result) (mask, bool) {
				_, ok := fixedMap[string(ext(res))]
				return nil, ok
			}
		}
	} else if proj.Order == "first" {
		initField = func(field *Field) {
			field.order = make(map[string]int)
			field.cmp = func(a, b string) int {
				return field.order[a] - field.order[b]
			}
		}
	} else if cmp, ok := builtinOrders[proj.Order]; ok {
		initField = func(field *Field) {
			field.cmp = cmp
		}
	} else {
		return nil, &parse.SyntaxError{q, proj.OrderOff, fmt.Sprintf("unknown order %q", proj.Order)}
	}

	var project func(*benchfmt.Result, *[]string)
	switch proj.Key {
	case ".config":
		// File configuration, excluding any more
		// specific file keys.
		if proj.Order == "fixed" {
			// Fixed orders don't make sense for a whole tuple.
			return nil, &parse.SyntaxError{q, proj.OrderOff, "fixed order not allowed for .config"}
		}

		p.haveConfig = true
		group := s.addGroup(s.root, ".config")
		seen := make(map[string]*Field)
		project = func(r *benchfmt.Result, row *[]string) {
			for _, cfg := range r.Config {
				if !cfg.File {
					continue
				}

				// Have we already seen this key? If so, use its already
				// assigned field index.
				field, ok := seen[cfg.Key]
				if !ok {
					// This closure doesn't get called until we've
					// parsed all projections, so p.configKeys is fully
					// populated from all parsed projections.
					if p.configKeys[cfg.Key] {
						// This key was explicitly specified in another
						// projection, so omit it from .config.
						continue
					}
					// Create a new field for this new key.
					field = s.addField(group, cfg.Key)
					initField(field)
					seen[cfg.Key] = field
				}

				(*row)[field.idx] = s.intern(cfg.Value)
			}
		}

	case ".fullname":
		// Full benchmark name, including name config.
		// We want to exclude any more specific keys,
		// including keys from later projections, so
		// we delay constructing the extractor until
		// we process the first Result.
		p.haveFullname = true
		field := s.addField(s.root, ".fullname")
		initField(field)
		makeFilter(extractFull)

		project = func(r *benchfmt.Result, row *[]string) {
			if p.fullExtractor == nil {
				p.fullExtractor = newExtractorFullName(p.fullnameKeys)
			}
			val := p.fullExtractor(r)
			(*row)[field.idx] = s.intern(val)
		}

	case ".unit":
		return nil, &parse.SyntaxError{q, proj.KeyOff, ".unit is only allowed in filters"}

	default:
		// This is a specific sub-name or file key. Add it
		// to the excludes.
		if proj.Key == ".name" || strings.HasPrefix(proj.Key, "/") {
			p.fullnameKeys = append(p.fullnameKeys, proj.Key)
		} else {
			p.configKeys[proj.Key] = true
		}
		ext, err := newExtractor(proj.Key)
		if err != nil {
			return nil, &parse.SyntaxError{q, proj.KeyOff, err.Error()}
		}
		field := s.addField(s.root, proj.Key)
		initField(field)
		makeFilter(ext)
		project = func(r *benchfmt.Result, row *[]string) {
			val := ext(r)
			(*row)[field.idx] = s.intern(val)
		}
	}
	s.project = append(s.project, project)
	return filter, nil
}

// A Projection extracts some subset of the fields of a benchfmt.Result
// into a Key.
//
// A Projection also implies a sort order over Keys that is
// lexicographic over the fields of the Projection. The sort order of
// each individual field is specified by the projection expression and
// defaults to the order in which values of that field were first
// observed.
type Projection struct {
	root    *Field
	nFields int

	// unitField, if non-nil, is the ".unit" field used to project
	// the values of a benchmark result.
	unitField *Field

	// project is a set of functions that project a Result into
	// row.
	//
	// These take a pointer to row because these functions may
	// grow the set of fields, so the row slice may grow.
	project []func(r *benchfmt.Result, row *[]string)

	// row is the buffer used to construct a projection.
	row []string

	// flatCache is a cache of the flattened sort fields in tuple
	// comparison order.
	flatCache     []*Field
	flatCacheOnce *sync.Once

	// interns is used to intern the []byte to string conversion. It's
	// keyed by string because we can't key a map on []byte, but the
	// compiler elides the string allocation in interns[string(x)], so
	// lookups are still cheap. These strings are always referenced in
	// keys, so this doesn't cause any over-retention.
	interns map[string]string

	// keys are the interned Keys of this Projection.
	keys map[uint64][]*keyNode
}

func newProjection() *Projection {
	var p Projection
	p.root = &Field{idx: -1}
	p.flatCacheOnce = new(sync.Once)
	p.interns = make(map[string]string)
	p.keys = make(map[uint64][]*keyNode)
	return &p
}

func (p *Projection) addField(group *Field, name string) *Field {
	if group.idx != -1 {
		panic("field's parent is not a group")
	}

	// Assign this field an index.
	field := &Field{Name: name, proj: p, idx: p.nFields}
	p.nFields++
	group.Sub = append(group.Sub, field)
	// Clear the flat cache.
	if p.flatCache != nil {
		p.flatCache = nil
		p.flatCacheOnce = new(sync.Once)
	}
	// Add to the row buffer.
	p.row = append(p.row, "")
	return field
}

func (p *Projection) addGroup(group *Field, name string) *Field {
	field := &Field{Name: name, IsTuple: true, proj: p, idx: -1}
	group.Sub = append(group.Sub, field)
	return field
}

// Fields returns the fields of p. These correspond exactly to the
// fields in the Projection's projection expression.
//
// The caller must not modify the returned slice.
func (p *Projection) Fields() []*Field {
	return p.root.Sub
}

// FlattenedFields is like Fields, but expands tuple Fields
// (specifically, ".config") into their sub-Fields. This is also the
// sequence of Fields used for sorting Keys returned from this
// Projection.
//
// The caller must not modify the returned slice.
func (p *Projection) FlattenedFields() []*Field {
	// This can reasonably be called in parallel after all results have
	// been projected, so we make sure it's thread-safe.
	p.flatCacheOnce.Do(func() {
		p.flatCache = []*Field{}
		var walk func(f *Field)
		walk = func(f *Field) {
			if f.idx != -1 {
				p.flatCache = append(p.flatCache, f)
				return
			}
			for _, sub := range f.Sub {
				walk(sub)
			}
		}
		walk(p.root)
	})
	return p.flatCache
}

// A Field is a single field of a Projection.
//
// For example, in the projection ".name,/gomaxprocs", ".name" and
// "/gomaxprocs" are both Fields.
//
// A Field may be a group field with sub-Fields.
type Field struct {
	Name string

	// IsTuple indicates that this Field is a tuple that does not itself
	// have a string value.
	IsTuple bool

	// Sub is the sequence of sub-Fields for a group field.
	Sub []*Field

	proj *Projection

	// idx gives the index of this field's values in a keyNode.
	//
	// Indexes are assigned sequentially as new sub-Fields are added to
	// group Fields. This allows the set of Fields to grow without
	// invalidating existing Keys.
	//
	// idx is -1 for Fields that are not directly stored in a keyNode,
	// such as the root Field and ".config".
	idx int

	// cmp is the comparison function for values of this field. It
	// returns <0 if a < b, >0 if a > b, or 0 if a == b or a and b
	// are unorderable.
	cmp func(a, b string) int

	// order, if non-nil, records the observation order of this
	// field.
	order map[string]int
}

// String returns the name of Field f.
func (f Field) String() string {
	return f.Name
}

var keySeed = maphash.MakeSeed()

// Project extracts fields from benchmark Result r according to
// Projection s and returns them as a Key.
//
// Two Keys produced by Project will be == if and only if their
// projected fields have the same values. Notably, this means Keys can
// be used as Go map keys, which is useful for grouping benchmark
// results.
//
// Calling Project may add new sub-Fields to group Fields in this
// Projection. For example, if the Projection has a ".config" field and
// r has a never-before-seen file configuration key, this will add a new
// sub-Field to the ".config" Field.
//
// If this Projection includes a .units field, it will be left as "" in
// the resulting Key. The caller should use ProjectValues instead.
func (p *Projection) Project(r *benchfmt.Result) Key {
	p.populateRow(r)
	return p.internRow()
}

// ProjectValues is like Project, but for each benchmark value of
// r.Values individually. The returned slice corresponds to the
// r.Values slice.
//
// If this Projection includes a .unit field, it will differ between
// these Keys. If not, then all of the Keys will be identical
// because the benchmark values vary only on .unit.
func (p *Projection) ProjectValues(r *benchfmt.Result) []Key {
	p.populateRow(r)
	out := make([]Key, len(r.Values))
	if p.unitField == nil {
		// There's no .unit, so the Keys will all be the same.
		key := p.internRow()
		for i := range out {
			out[i] = key
		}
		return out
	}
	// Vary the .unit field.
	for i, val := range r.Values {
		p.row[p.unitField.idx] = val.Unit
		out[i] = p.internRow()
	}
	return out
}

func (p *Projection) populateRow(r *benchfmt.Result) {
	// Clear the row buffer.
	for i := range p.row {
		p.row[i] = ""
	}

	// Run the projection functions to fill in row.
	for _, proj := range p.project {
		// proj may add fields and grow row.
		proj(r, &p.row)
	}
}

func (p *Projection) internRow() Key {
	// Hash the row. This must be invariant to unused trailing fields: the
	// field set can grow, and if those new fields are later cleared,
	// we want Keys from before the growth to equal Keys from after the growth.
	row := p.row
	for len(row) > 0 && row[len(row)-1] == "" {
		row = row[:len(row)-1]
	}
	var h maphash.Hash
	h.SetSeed(keySeed)
	for _, val := range row {
		h.WriteString(val)
	}
	hash := h.Sum64()

	// Check if we already have this key.
	keys := p.keys[hash]
	for _, key := range keys {
		if key.equalRow(row) {
			return Key{key}
		}
	}

	// Update observation orders.
	for _, field := range p.Fields() {
		if field.order == nil {
			// Not tracking observation order for this field.
			continue
		}
		var val string
		if field.idx < len(row) {
			val = row[field.idx]
		}
		if _, ok := field.order[val]; !ok {
			field.order[val] = len(field.order)
		}
	}

	// Save the key.
	key := &keyNode{p, append([]string(nil), row...)}
	p.keys[hash] = append(p.keys[hash], key)
	return Key{key}
}

func (p *Projection) intern(b []byte) string {
	if str, ok := p.interns[string(b)]; ok {
		return str
	}
	str := string(b)
	p.interns[str] = str
	return str
}
