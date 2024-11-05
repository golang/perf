# Go benchmark analysis tools

[![Go Reference](https://pkg.go.dev/badge/golang.org/x/perf.svg)](https://pkg.go.dev/golang.org/x/perf)

This subrepository holds tools and packages for analyzing [Go
benchmark results](https://golang.org/design/14313-benchmark-format),
such as the output of [testing package
benchmarks](https://pkg.go.dev/testing).

## Tools

This subrepository contains command-line tools for analyzing benchmark
result data.

[cmd/benchstat](cmd/benchstat) computes statistical summaries and A/B
comparisons of Go benchmarks.

[cmd/benchfilter](cmd/benchfilter) filters the contents of benchmark
result files.

[cmd/benchsave](cmd/benchsave) publishes benchmark results to
[perf.golang.org](https://perf.golang.org).

To install all of these commands, run
`go install golang.org/x/perf/cmd/...@latest`.
You can also
`git clone https://go.googlesource.com/perf` and run
`go install ./cmd/...` in the checkout.

## Packages

Underlying the above tools are several packages for working with
benchmark data. These are designed to work together, but can also be
used independently.

[benchfmt](benchfmt) reads and writes the Go benchmark format.

[benchunit](benchunit) manipulates benchmark units and formats numbers
in those units.

[benchproc](benchproc) provides tools for filtering, grouping, and
sorting benchmark results.

[benchmath](benchmath) provides tools for computing statistics over
distributions of benchmark measurements.

## Deprecated packages

The following packages are deprecated and no longer supported:

[storage](storage) contains a deprecated version of the
https://perfdata.golang.org/ benchmark result storage system. These
packages have moved to https://golang.org/x/build.

[analysis](analysis) contains a deprecated version of the
https://perf.golang.org/ benchmark result analysis system. These
packages have moved to https://golang.org/x/build.

## Report Issues / Send Patches

This repository uses Gerrit for code changes. To learn how to submit changes to
this repository, see https://go.dev/doc/contribute.

The git repository is https://go.googlesource.com/perf.

The main issue tracker for the perf repository is located at
https://go.dev/issues. Prefix your issue with "x/perf:" in the
subject line, so it is easy to find.
