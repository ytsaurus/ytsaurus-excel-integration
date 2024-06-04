# excel-exporter

Service for downloading data from YTaurus Static tables and QueryTracker results as Excel spreadsheets

* [API](./docs/api.md)
* [Authorization](./docs/auth.md)

## Developer guide

### Build

```
go build -a -v -mod=readonly go.ytsaurus.tech/yt/microservices/excel/exporter/...
```

### Test

```
go test -mod=readonly -race go.ytsaurus.tech/yt/microservices/excel/exporter/... -count=1 -v
```

### Logs

The service writes logs to the `excel-exporter.log` file of the `--log-dir` directory (`/logs` by default) and uses hourly rotation with archiving to gz.

Using the `--log-to-stderr` flag, you can force the service to write logs to stderr.

### Local example

```
./exporter/cmd/excel-exporter/excel-exporter -log-to-stderr -config ./exporter/configs/example-config.yaml
```

Example query:
```
curl -g -v -X GET http://localhost:6029/hahn/api/export\
\?path=//home/someone/schematic-table\{"id","survey_id"\}\[%232:%234\]\
  --cookie 'Session_id=3:16'\
  --output /tmp/book.xlsx
```

### Run benchmarks

Requires running YTsaurus cluster.

```
YT_PROXY=my-cluster YT_TOKEN=my-token go test -mod=readonly go.ytsaurus.tech/yt/microservices/excel/exporter/... -v -test.bench=. -test.run="^$"  -test.benchmem=true  -test.benchtime=20x
BenchmarkExport/read/all/small-8                      20          43517066 ns/op         1479603 B/op      43390 allocs/op
BenchmarkExport/convert/all/small-8                   20          30471422 ns/op         4131890 B/op     113322 allocs/op
BenchmarkExport/read/all/medium-8                     20         104344948 ns/op        14007010 B/op     431376 allocs/op
BenchmarkExport/convert/all/medium-8                  20         209874960 ns/op        39863017 B/op    1095320 allocs/op
BenchmarkExport/read/all/large-8                      20        1170088643 ns/op        158604540 B/op   4311485 allocs/op
BenchmarkExport/convert/all/large-8                   20        2281841094 ns/op        415786785 B/op  10915485 allocs/op
BenchmarkExport/read/subset/small-8                   20          26205289 ns/op         1067652 B/op      16307 allocs/op
BenchmarkExport/convert/subset/small-8                20          46603657 ns/op         1967019 B/op      37120 allocs/op
BenchmarkExport/read/subset/medium-8                  20          57926262 ns/op         9886826 B/op     160500 allocs/op
BenchmarkExport/convert/subset/medium-8               20          72128645 ns/op        18301712 B/op     334347 allocs/op
BenchmarkExport/read/subset/large-8                   20         407521985 ns/op        97948088 B/op    1601672 allocs/op
BenchmarkExport/convert/subset/large-8                20         719223986 ns/op        179296687 B/op   3305535 allocs/op
```

`BenchmarkExport/convert` measures conversion statistics for tables of different sizes.

`BenchmarkExport/read` measures statistics for reading YTsaurus static tables of different sizes.

`small|medium|large` — table size, `1000|10000|100000`, respectively.

`all|subset` specifies whether the entire table is being read/converted or just a set of columns.

## Monitoring

The service supplies prometheus metrics on the configurable port `debug_http_addr: ":6060"`.

View raw metrics of a specific instance:
```
curl http://localhost:6060/prometheus
```
