// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchproc

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchunit"
)

// Example shows a complete benchmark processing pipeline that uses
// filtering, projection, accumulation, and sorting.
func Example() {
	// Open the example benchmark data.
	f, err := os.Open("testdata/suffixarray.bench")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Create a filter that extracts just "BenchmarkNew" on the value
	// "go" of the name key "text". Typically, the filter expression
	// would come from a command-line flag.
	filter, err := NewFilter(".name:New /text:go")
	if err != nil {
		log.Fatal(err)
	}
	// Create a projection. This projection extracts "/bits=" and
	// "/size=" from the benchmark name. It sorts bits in the
	// default, first-observation order and size numerically.
	// Typically, the projection expression would come from a
	// command-line flag.
	var pp ProjectionParser
	projection, err := pp.Parse("/bits,/size@num", filter)
	if err != nil {
		log.Fatal(err)
	}
	// Create a projection that captures all configuration not
	// captured by the above projection. We'll use this to check
	// if there's unexpected variation in other configuration
	// fields and report it.
	residue := pp.Residue()

	// We'll accumulate benchmark results by their projection.
	// Projections create Keys, which are == if the projected
	// values are ==, so they can be used as map keys.
	bySize := make(map[Key][]float64)
	var keys []Key
	var residues []Key

	// Read the benchmark results.
	r := benchfmt.NewReader(f, "example")
	for r.Scan() {
		var res *benchfmt.Result
		switch rec := r.Result(); rec := rec.(type) {
		case *benchfmt.Result:
			res = rec
		case *benchfmt.SyntaxError:
			// Report a non-fatal parse error.
			log.Print(err)
			continue
		default:
			// Unknown record type. Ignore.
			continue
		}

		// Step 1: If necessary, transform the Result, for example to
		// add configuration keys that could be used in filters and
		// projections. This example doesn't need any transformation.

		// Step 2: Filter the result.
		if match, err := filter.Apply(res); !match {
			// Result was fully excluded by the filter.
			if err != nil {
				// Print the reason we rejected this result.
				log.Print(err)
			}
			continue
		}

		// Step 3: Project the result. This produces a Key
		// that captures the "size" and "bits" from the result.
		key := projection.Project(res)

		// Accumulate the results by configuration.
		speed, ok := res.Value("sec/op")
		if !ok {
			continue
		}
		if _, ok := bySize[key]; !ok {
			keys = append(keys, key)
		}
		bySize[key] = append(bySize[key], speed)

		// Collect residue configurations.
		resConfig := residue.Project(res)
		residues = append(residues, resConfig)
	}
	// Check for I/O errors.
	if err := r.Err(); err != nil {
		log.Fatal(err)
	}

	// Step 4: Sort the collected configurations using the order
	// specified by the projection.
	SortKeys(keys)

	// Print the results.
	fmt.Printf("%-24s %s\n", "config", "sec/op")
	for _, config := range keys {
		fmt.Printf("%-24s %s\n", config, benchunit.Scale(mean(bySize[config]), benchunit.Decimal))
	}

	// Check if there was variation in any other configuration
	// fields that wasn't captured by the projection and warn the
	// user that something may be unexpected.
	nonsingular := NonSingularFields(residues)
	if len(nonsingular) > 0 {
		fmt.Printf("warning: results vary in %s\n", nonsingular)
	}

	// Output:
	// config                   sec/op
	// /bits:32 /size:100K      4.650m
	// /bits:32 /size:500K      26.18m
	// /bits:32 /size:1M        51.39m
	// /bits:32 /size:5M        306.7m
	// /bits:32 /size:10M       753.0m
	// /bits:32 /size:50M       5.814
	// /bits:64 /size:100K      5.081m
	// /bits:64 /size:500K      26.43m
	// /bits:64 /size:1M        55.60m
	// /bits:64 /size:5M        366.6m
	// /bits:64 /size:10M       821.2m
	// /bits:64 /size:50M       6.390
}
