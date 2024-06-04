# API

## Upload Excel spreadsheet to YTsaurus static table.

**GET \<cluster\>/api/v1/upload** — upload data from an Excel spreadsheet into a YTsaurus Static table

### Request

The Excel file is passed via `multipart/form-data`; form name — `uploadfile`.

The control part of the request is passed via URL params:
* (required) **path** — ypath path to YTsaurus table
* (optional) **start_row** — first row to upload; optional; default — 1
* (optional) **row_count** — number of rows to upload; optional; default — all 
* (optional) **sheet** — Excel spreadsheet name; optional; default — first spreadsheet
* (optional) **header** — boolean flag to read (YTsaurus -> Excel) column mapping from the first row of Excel spreadsheet; default — false
* (optional) **types** — boolean flag to read column types from the first or the second row of Excel spreadsheet; default — false
* (optional) **columns** — (YTsaurus -> Excel) column mapping, for example `{"name":"A", "name2": "A", "id": "D"}`
* (optional) **append** — boolean flag to append new rows to the table instead of overwriting; default — false, the table will be overwritten
* (optional) **create** — boolean flag to create table by inferring columns from request; default — false, the table is expected to be pre-created

If the row range is not specified (`start_row=0 && row_count=0`) and `header=true`, then the first row will not be uploaded.

Default (YTsaurus -> Excel) column mapping matches them by position: the first column in the table schema will be matched with column `A`, the second with column `B`, etc.

When creating a table (`create==true`), the following logic is used to determine the names of YTsaurus table columns:
1. Column names from **columns** are used, if provided.
2. if `header==true` the the names from the first Excel row are used
3. Excel column names are used: `A, B, C...`

By default all Excel columns are exported as YTsaurus type `any`.
If `types==true && header=false` then types are read from the first Excel row.
If `types==true && header=true` then types are read from the second Excel row.

### Response

Successful request results in 200 Ok. In case of error 400 or 500 is returned with a json error message.

The error is additionally added to the http headers: `X-Yt-Error`, `X-Yt-Response-Code` and `X-Yt-Response-Message`.

Example error:
```
{
  "code": 1,
  "message": "Error uploading Path: //home/verytable/upload-tests/small-src, Columns: Map[id:B], StartRow: 3, RowCount: 7, Append: True",
  "attributes": {
    "host": "verytable",
    "request_id": "f1a52e34-7c43184c-9e2d0683-26bf198f"
  },
  "inner_errors": [
    {
      "code": 1,
      "message": "Bad request",
      "inner_errors": [
        {
          "code": 1,
          "message": "Unable to read rows of sheet \"abc\"",
          "inner_errors": [
            {
              "code": 1,
              "message": "Sheet abc is not exist"
            }
          ]
        }
      ]
    }
  ]
}
```

### Limits

* Only one excel sheet is uploaded
* Max number of rows — 1048576
* Max number of columns — 16384
