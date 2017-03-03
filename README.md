# Go performance measurement, storage, and analysis tools

This subrepository holds the source for various packages and tools
related to performance measurement, storage, and analysis.

[cmd/benchstat](cmd/benchstat) contains a command-line tool that
computes and compares statistics about benchmarks.

[cmd/benchsave](cmd/benchsave) contains a command-line tool for
publishing benchmark results.

[storage](storage) contains the https://perfdata.golang.org/ benchmark
result storage system.

[analysis](analysis) contains the https://perf.golang.org/ benchmark
result analysis system.

Both storage and analysis can be run locally; the following commands will run
the complete stack on your machine with an in-memory datastore.

```
go get -u golang.org/x/perf/storage/localperfdata
go get -u golang.org/x/perf/analysis/localperf
localperfdata -addr=:8081 -view_url_base=http://localhost:8080/search?q=upload: &
localperf -addr=:8080 -storage=localhost:8081
```

The storage system is designed to have a
[standardized API](storage/appengine/static/index.html), and we
encourage additional analysis tools to be written against the API. A
client can be found in [storage/client](storage/client).

--

Contributions to Go are appreciated. See http://golang.org/doc/contribute.html.

* Bugs can be filed at the [Go issue tracker](https://golang.org/issue/new?title=x/perf:+).
