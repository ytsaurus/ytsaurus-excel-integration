# excel-uploader ![Build](https://github.com/ytsaurus/ytsaurus-excel-integration/actions/workflows/go.yaml/badge.svg)

Service for uploading Excel spreadsheets to YTsaurus static tables.

* [API](./docs/api.md)
* [Authorization](./docs/auth.md)

## Developer guide

### Build

```
go build -a -v -mod=readonly go.ytsaurus.tech/yt/microservices/excel/uploader/...
```

### Test

```
go test -mod=readonly -race go.ytsaurus.tech/yt/microservices/excel/uploader/... -count=1 -v
```

### Logs

The service writes logs to the `excel-uploader.log` file of the `--log-dir` directory (`/logs` by default) and uses hourly rotation with archiving to gz.

Using the `--log-to-stderr` flag, you can force the service to write logs to stderr.

### Local example

```
./uploader/cmd/excel-uploader/excel-uploader -log-to-stderr -config ./uploader/configs/example-config.yaml
```

Example request:
```
curl -g -v -F 'uploadfile=@/tmp/book.xlsx' http://localhost:6029/minisaurus/api/upload\?path\=//home/verytable/upload-tests/small-src\&append\=true\&columns\=%7B%22id%22%3A%22B%22%7D\&sheet\="Sheet1"\&start_row\=3\&row_count\=4 --cookie 'Session_id=3:1605443621...' -H 'X-CSRF-Token: fa6951d...'
```

## Monitoring

The service supplies prometheus metrics on the configurable port `debug_http_addr: ":6060"`.

View raw metrics of a specific instance:
```
curl http://localhost:6060/prometheus
```
