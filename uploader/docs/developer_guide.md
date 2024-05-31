# Developer guide

### Build

```
go build -a -v -mod=readonly go.ytsaurus.tech/yt/microservices/excel/uploader/...
```

### Test

```
go test -mod=readonly -race go.ytsaurus.tech/yt/microservices/excel/uploader/... -count=1 -v
```

### Logs

Сервис пишет логи в файл `excel-uploader.log` директории `--log-dir` (`/logs` по умолчанию) и используют почасовую ротацию с архивацией в gz.

С помощью флажка `--log-to-stderr` можно заставить сервис писать логи в stderr.

### Local example

```
./uploader/cmd/excel-uploader/excel-uploader -log-to-stderr -config ./uploader/configs/example-config.yaml
```

Пример запроса:
```
curl -g -v -F 'uploadfile=@/tmp/book.xlsx' http://localhost:6029/hahn/api/upload\?path\=//home/verytable/upload-tests/small-src\&append\=true\&columns\=%7B%22id%22%3A%22B%22%7D\&sheet\="Sheet1"\&start_row\=3\&row_count\=4 --cookie 'Session_id=3:1605443621...' -H 'X-CSRF-Token: fa6951d...'
```

Соответствующий лог curl:
```
*   Trying ::1...
* TCP_NODELAY set
* Connected to localhost (::1) port 6029 (#0)
> POST /hahn/api/upload?path=//home/verytable/upload-tests/small-src[%233:%2310]&append=true&columns=%7B%22id%22%3A%22B%22%7D&sheet=Sheet1 HTTP/1.1
> Host: localhost:6029
> User-Agent: curl/7.58.0
> Accept: */*
> Cookie: Session_id=3:1605443621...
> X-CSRF-Token: fa6951d...
> Content-Length: 372136
> Content-Type: multipart/form-data; boundary=------------------------a2bd3b0262f8ea6d
> Expect: 100-continue
>
< HTTP/1.1 100 Continue
< HTTP/1.1 200 OK
< Vary: Origin
< Date: Sun, 29 Nov 2020 17:16:25 GMT
< Content-Length: 0
<
* Connection #0 to host localhost left intact
```
