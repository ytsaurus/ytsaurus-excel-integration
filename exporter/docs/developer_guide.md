# Developer guide

### Build

```
go build -a -v -mod=readonly go.ytsaurus.tech/yt/microservices/excel/exporter/...
```

### Test

```
go test -mod=readonly -race go.ytsaurus.tech/yt/microservices/excel/exporter/... -count=1 -v
```

### Logs

Сервис пишет логи в файл `excel-exporter.log` директории `--log-dir` (`/logs` по умолчанию) и используют почасовую ротацию с архивацией в gz.

С помощью флажка `--log-to-stderr` можно заставить сервис писать логи в stderr.

### Local example

```
./exporter/cmd/excel-exporter/excel-exporter -log-to-stderr -config ./exporter/configs/example-config.yaml
```

Пример запроса:
```
curl -g -v -X GET http://excel-exporter.yt.yandex-team.ru/hahn/api/export\
\?path=//home/szveroboev/schematic-table\{"id","survey_id"\}\[%232:%234\]\
  --cookie 'Session_id=3:16'\
  --output /tmp/book.xlsx
```

### Run benchmarks

```
ya make -r -tt yt/microservices/excel/exporter/internal/exporter --test-stdout --test-param '-test.bench=.' --test-param '-test.run="^$"' --test-param '-test.benchmem=true' --test-param '-test.benchtime=20x'  2> >(grep "/op")
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

В `BenchmarkExport/convert` замеряются статистики конвертации таблиц разного размера.

В `BenchmarkExport/read` замеряются статистики чтения таблиц разного размера.

`small|medium|large` - размеры таблиц, `1000|10000|100000`, соответственно.

`all|subset` - читается/конвертируется ли вся таблица или только набор столбцов.
