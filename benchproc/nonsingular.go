// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

// NonSingularFields returns the subset of Fields for which at least two of keys
// have different values.
//
// This is useful for warning the user if aggregating a set of results has
// resulted in potentially hiding important configuration differences. Typically
// these keys are "residue" keys produced by ProjectionParser.Residue.
func NonSingularFields(keys []Key) []*Field {
	// TODO: This is generally used on residue keys, but those might
	// just have ".fullname" (generally with implicit exclusions).
	// Telling the user that a set of benchmarks varies in ".fullname"
	// isn't nearly as useful as listing out the specific subfields. We
	// should synthesize "/N" subfields for ".fullname" for this (this
	// makes even more sense if we make .fullname a group field that
	// already has these subfields).

	if len(keys) <= 1 {
		// There can't be any differences.
		return nil
	}
	var out []*Field
	fields := commonProjection(keys).FlattenedFields()
	for _, f := range fields {
		base := keys[0].Get(f)
		for _, k := range keys[1:] {
			if k.Get(f) != base {
				out = append(out, f)
				break
			}
		}
	}
	return out
}
