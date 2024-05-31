package exporter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"go.ytsaurus.tech/library/go/core/log/zap"
	"go.ytsaurus.tech/yt/go/guid"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	"go.ytsaurus.tech/yt/go/ytlog"
	"go.ytsaurus.tech/yt/go/yttest"
)

const QueryResultID = "bc3dace-d71d30a6-51be519e-7692cfc3"

func TestMakeExportRequest(t *testing.T) {
	for _, tc := range []struct {
		reqStr   string
		expected *ExportRequest
		isError  bool
	}{
		{
			reqStr: `//home/abc`,
			expected: &ExportRequest{
				Path:       "//home/abc",
				allColumns: true,
				allRows:    true,
			},
		},
		{
			reqStr: `//home/abc{"id"}`,
			expected: &ExportRequest{
				Path:    "//home/abc",
				Columns: []string{"id"},
				allRows: true,
			},
		},
		{
			reqStr: `//home/abc{"id"}[#50:#150]`,
			expected: &ExportRequest{
				Path:     "//home/abc",
				Columns:  []string{"id"},
				StartRow: 50,
				RowCount: 100,
			},
		},
		{
			reqStr: `<file_name=data.xlsx>//home/abc{"id"}[#50:#150]`,
			expected: &ExportRequest{
				Filename: "data.xlsx",
				Path:     "//home/abc",
				Columns:  []string{"id"},
				StartRow: 50,
				RowCount: 100,
			},
		},
	} {
		t.Run(tc.reqStr, func(t *testing.T) {
			req, err := MakeExportRequest(tc.reqStr, "")
			if tc.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, req)
			}
		})
	}
}

func TestExportRequest_MakeFileName(t *testing.T) {
	for _, tc := range []struct {
		req      *ExportRequest
		filename string
	}{
		{
			req: &ExportRequest{
				Path:       "//home/verytable/tbl",
				allColumns: true,
				allRows:    true,
			},
			filename: "yt__home_verytable_tbl.xlsx",
		},
		{
			req: &ExportRequest{
				Path:       "//home/verytable/tbl",
				allColumns: false,
				Columns:    []string{"id", "name"},
				allRows:    true,
			},
			filename: "yt__home_verytable_tbl__id_name.xlsx",
		},
		{
			req: &ExportRequest{
				Path:       "//home/verytable/tbl",
				allColumns: true,
				StartRow:   10,
				RowCount:   100,
			},
			filename: "yt__home_verytable_tbl__10_110__.xlsx",
		},
		{
			req: &ExportRequest{
				Path:     "//home/verytable/tbl",
				Columns:  []string{"id", "name"},
				StartRow: 10,
				RowCount: 100,
			},
			filename: "yt__home_verytable_tbl__id_name__10_110__.xlsx",
		},
	} {
		t.Run(tc.req.String(), func(t *testing.T) {
			require.Equal(t, tc.filename, tc.req.MakeFileName(""))
		})
	}
}

func TestExportRequest_MakeFileName_LongFilename(t *testing.T) {
	req := &ExportRequest{
		Path:       "//home/verytable/tbl",
		allColumns: true,
		allRows:    true,
	}
	suffix := "_" + strings.Repeat("a", 300)
	filename := fmt.Sprintf("yt__home_verytable_tbl_%s.xlsx", strings.Repeat("a", 135))

	require.Equal(t, filename, req.MakeFileName(suffix))
}

type S1 struct {
	I16  int16  `yson:"i_16"`
	UI16 uint16 `yson:"ui_16"`
	I32  int32  `yson:"i_32"`
	UI32 uint32 `yson:"ui_32"`
	I64  int64  `yson:"i_64"`
	UI64 uint64 `yson:"ui_64"`

	Float  float32 `yson:"float"`
	Double float64 `yson:"double"`
	Bool   bool    `yson:"bool"`
	String string  `yson:"string"`

	Date      schema.Date      `yson:"date"`
	Datetime  schema.Datetime  `yson:"datetime"`
	Timestamp schema.Timestamp `yson:"timestamp"`
	Interval  schema.Interval  `yson:"interval"`

	Any any `yson:"any"`
}

type S2 struct {
	Comment string `yson:"comment"`
	Bytes   []byte `yson:"bytes"`
}

func TestExportFile(t *testing.T) {
	env, cancel := yttest.NewEnv(t)
	defer cancel()

	for _, tc := range []struct {
		name   string
		schema schema.Schema
		rows   []any
		req    *ExportRequest
		opts   *ExportOptions
		error  bool
	}{
		{
			name:   "types",
			schema: schema.MustInfer(&S1{}),
			rows: []any{
				&S1{I16: -16, UI16: 16, I32: -32, UI32: 32, I64: -64, UI64: 64, Float: 32.5, Double: 42.5},
				&S1{I16: -160, UI16: 160, I32: -320, UI32: 320, I64: -640, UI64: 640, Float: 325.0, Double: 425.0},
				&S1{I16: -16, UI16: 16, I32: -32, UI32: 32, I64: -64, UI64: 64, Float: 32.50000005, Double: 42.50000005},
				&S1{I64: 4291747199999999, UI64: 4291747199999999},
				&S1{I64: 4291747200000000, UI64: 4291747200000000},
				&S1{I64: 4, UI64: 4},
				&S1{Double: 0.00100000000000000016},
				&S1{Double: 0.00000000000000000016},
				&S1{String: "abacaba"},
				&S1{String: ""},
				&S1{String: "123"},
				&S1{String: "{" + strings.Repeat("a", maxExcelStrLen-2) + "}"},
				&S1{String: "{" + strings.Repeat("a", maxExcelStrLen) + "}"},
				&S1{Bool: true},
				&S1{Bool: false},
				&S1{Bool: true},
				&S1{Date: NewDate(time.Now())},
				&S1{Date: NewDate(time.Date(2000, time.December, 12, 10, 22, 17, 0, time.UTC))},
				&S1{Datetime: NewDatetime(time.Now())},
				&S1{Datetime: NewDatetime(time.Date(2000, time.December, 12, 10, 22, 17, 0, time.UTC))},
				&S1{Timestamp: NewTimestamp(time.Now())},
				&S1{Timestamp: NewTimestamp(time.Date(2000, time.December, 12, 10, 22, 17, 302000000, time.UTC))},
				&S1{Timestamp: NewTimestamp(time.Date(2000, time.December, 12, 10, 22, 17, 302001001, time.UTC))},
				&S1{Interval: NewInterval(time.Hour + time.Minute*32)},
				&S1{Interval: NewInterval(time.Duration(4291747199999999000))},
				&S1{Any: []int{1, 2, 3}},
				&S1{Any: "str"},
				&S1{Any: struct {
					Name string `yson:"name"`
					Age  int32  `yson:"age"`
				}{Name: "tst", Age: 12}},
			},
			req: &ExportRequest{
				Path: ypath.Path("//tmp/types"),
				Columns: []string{"i_16", "ui_16", "i_32", "ui_32", "i_64", "ui_64", "float", "double",
					"string", "bool", "date", "datetime", "timestamp", "interval", "any"},
				StartRow: 0,
				RowCount: MaxRowCount,
			},
		},
		{
			name:   "all-columns",
			schema: schema.MustInfer(&S1{}),
			rows: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 2, UI64: 2},
			},
			req: &ExportRequest{
				Path:     ypath.Path("//tmp/all-columns"),
				StartRow: 0,
				RowCount: MaxRowCount,
			},
		},
		{
			name:   "bytes",
			schema: schema.MustInfer(&S2{}),
			rows: []any{
				&S2{Comment: "unprintable bytes", Bytes: []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}},
				&S2{Comment: "stripped", Bytes: append(bytes.Repeat([]byte{'b'}, maxExcelStrLen-1), '}')},
				&S2{Comment: "not stripped", Bytes: append(bytes.Repeat([]byte{'b'}, maxExcelStrLen), '}')},
			},
			req: &ExportRequest{
				Path:     ypath.Path("//tmp/bytes"),
				Columns:  []string{"comment", "bytes"},
				StartRow: 0,
				RowCount: MaxRowCount,
			},
		},
		{
			name:   "max-file-size-exceeded",
			schema: schema.MustInfer(&S1{}),
			rows: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 2, UI64: 2},
			},
			req: &ExportRequest{
				Path:     ypath.Path("//tmp/max-file-size-exceeded"),
				Columns:  []string{"i_64", "ui_64"},
				StartRow: 0,
				RowCount: MaxRowCount,
			},
			opts:  &ExportOptions{MaxExcelFileSize: 31}, // 31 < 8 * 4
			error: true,
		},
	} {
		t.Run(tc.req.String(), func(t *testing.T) {
			tc.req.NumberPrecisionMode = NumberPrecisionModeString

			_, err := yt.CreateTable(env.Ctx, env.YT, tc.req.Path, yt.WithSchema(tc.schema))
			require.NoError(t, err)

			writer, err := env.YT.WriteTable(env.Ctx, tc.req.Path, nil)
			require.NoError(t, err)

			for _, r := range tc.rows {
				err = writer.Write(r)
				require.NoError(t, err)
			}

			err = writer.Commit()
			require.NoError(t, err)

			if tc.opts == nil {
				tc.opts = &ExportOptions{MaxExcelFileSize: 1024 * 1024 * 10}
			}
			f, err := Export(env.Ctx, env.YT, tc.req, tc.opts)

			if !tc.error {
				require.NoError(t, err)

				outFilename := OutputPath(tc.name + ".xlsx")
				outFile, err := os.Create(outFilename)
				require.NoError(t, err)
				t.Logf("Saving excel file to %q", outFilename)
				require.NoError(t, f.File.Write(outFile))
			} else {
				require.Error(t, err)
			}
		})
	}
}

type UIntAndDouble struct {
	UI64   uint64  `yson:"ui_64"`
	Double float64 `yson:"double"`
}

func TestLosePrecision(t *testing.T) {
	env, cancel := yttest.NewEnv(t)
	defer cancel()

	req := &ExportRequest{
		Path:                ypath.Path("//tmp/lose-precision"),
		Columns:             []string{"ui_64", "double"},
		StartRow:            0,
		RowCount:            MaxRowCount,
		NumberPrecisionMode: NumberPrecisionModeLose,
	}

	_, err := yt.CreateTable(env.Ctx, env.YT, req.Path, yt.WithSchema(schema.MustInfer(&UIntAndDouble{})))
	require.NoError(t, err)

	writer, err := env.YT.WriteTable(env.Ctx, req.Path, nil)
	require.NoError(t, err)

	err = writer.Write(&UIntAndDouble{
		UI64:   4291747199999999,
		Double: 0.00100000000000000016,
	})
	require.NoError(t, err)

	err = writer.Commit()
	require.NoError(t, err)

	f, err := Export(env.Ctx, env.YT, req, &ExportOptions{MaxExcelFileSize: 1024 * 1024 * 10})
	require.NoError(t, err)

	uintVal, err := f.File.GetCellValue("Sheet1", "A3")
	require.NoError(t, err)
	doubleVal, err := f.File.GetCellValue("Sheet1", "B3")
	require.NoError(t, err)

	require.Equal(t, "4291747200000000", uintVal)
	require.Equal(t, "0.001", doubleVal)
}

func TestExportQueryResult(t *testing.T) {
	proxy := os.Getenv("TEST_YT_PROXY")
	t.Logf("This test talks to yt.")
	if proxy == "" {
		t.Skip("TEST_YT_PROXY env variable is not set")
	}

	l := &zap.Logger{L: zaptest.NewLogger(t)}

	yc, err := ythttp.NewClient(&yt.Config{
		Proxy:             proxy,
		ReadTokenFromFile: true,
		Logger:            l,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	opts := &ExportOptions{MaxExcelFileSize: 1024 * 1024 * 10}

	guid, err := guid.ParseString(QueryResultID)
	require.NoError(t, err)
	req := &ExportQueryResultRequest{
		ID:                  yt.QueryID(guid),
		NumberPrecisionMode: NumberPrecisionModeString,
	}

	f, err := ExportQueryResult(ctx, yc, req, opts)
	require.NoError(t, err)
	outFilename := OutputPath("export_query_result.xlsx")
	outFile, err := os.Create(outFilename)
	require.NoError(t, err)
	t.Logf("Saving excel file to %q", outFilename)
	require.NoError(t, f.File.Write(outFile))
}

func BenchmarkExport(b *testing.B) {
	env, cancel := yttest.NewEnv(b, yttest.WithLogger(ytlog.Must()))
	defer cancel()

	for _, bm := range []struct {
		name           string
		prepareRequest func() *ExportRequest
	}{
		{
			name: "all/small",
			prepareRequest: func() *ExportRequest {
				return &ExportRequest{Path: makeTestTable(b, env, 1000), RowCount: MaxRowCount}
			},
		},
		{
			name: "all/medium",
			prepareRequest: func() *ExportRequest {
				return &ExportRequest{Path: makeTestTable(b, env, 10000), RowCount: MaxRowCount}
			},
		},
		{
			name: "all/large",
			prepareRequest: func() *ExportRequest {
				return &ExportRequest{Path: makeTestTable(b, env, 100000), RowCount: MaxRowCount}
			},
		},
		{
			name: "subset/small",
			prepareRequest: func() *ExportRequest {
				return &ExportRequest{
					Path:     makeTestTable(b, env, 1000),
					Columns:  []string{"id", "integer"},
					RowCount: MaxRowCount,
				}
			},
		},
		{
			name: "subset/medium",
			prepareRequest: func() *ExportRequest {
				return &ExportRequest{
					Path:     makeTestTable(b, env, 10000),
					Columns:  []string{"id", "integer"},
					RowCount: MaxRowCount,
				}
			},
		},
		{
			name: "subset/large",
			prepareRequest: func() *ExportRequest {
				return &ExportRequest{
					Path:     makeTestTable(b, env, 100000),
					Columns:  []string{"id", "integer"},
					RowCount: MaxRowCount,
				}
			},
		},
	} {
		b.Run("read/"+bm.name, func(b *testing.B) {
			req := bm.prepareRequest()
			b.ResetTimer()

			path := req.MakePath()

			runBenchmark := func() {
				r, err := env.YT.ReadTable(env.Ctx, path, nil)
				require.NoError(b, err)
				defer func() { _ = r.Close() }()

				for r.Next() {
					var row map[string]any
					require.NoError(b, r.Scan(&row))
				}

				require.NoError(b, r.Err())
			}

			for i := 0; i < b.N; i++ {
				runBenchmark()
			}
		})

		b.Run("convert/"+bm.name, func(b *testing.B) {
			req := bm.prepareRequest()
			opts := &ExportOptions{MaxExcelFileSize: 1024 * 1024 * 100}
			b.ResetTimer()

			runBenchmark := func() {
				_, err := Export(env.Ctx, env.YT, req, opts)
				require.NoError(b, err)
			}

			for i := 0; i < b.N; i++ {
				runBenchmark()
			}
		})
	}
}

// makeTestTable writes sample table to "//tmp".
func makeTestTable(b *testing.B, env *yttest.Env, size int) ypath.Path {
	b.Helper()

	type S struct {
		ID      string  `yson:"id"`
		S1      string  `yson:"s1"`
		S2      string  `yson:"s2"`
		S3      string  `yson:"s3"`
		S4      string  `yson:"s4"`
		Integer int64   `yson:"integer"`
		Float   float64 `yson:"float"`
		Bool    bool    `yson:"bool"`
	}

	name := env.TmpPath()
	_, err := yt.CreateTable(env.Ctx, env.YT, name, yt.WithSchema(schema.MustInfer(&S{})))
	require.NoError(b, err)

	w, err := env.YT.WriteTable(env.Ctx, name, nil)
	require.NoError(b, err)

	for i := 0; i < size; i++ {
		row := S{
			ID:      randomName(),
			S1:      strings.Repeat("foobar", 10),
			S2:      randomName(),
			S3:      randomName(),
			S4:      randomName(),
			Integer: int64(i),
			Float:   float64(i * i),
			Bool:    i%2 == 0,
		}
		if err := w.Write(row); err != nil {
			b.Fatal(err)
		}
	}

	if err := w.Commit(); err != nil {
		b.Fatal(err)
	}

	return name
}

func NewDate(t time.Time) schema.Date {
	d, _ := schema.NewDate(t)
	return d
}

func NewDatetime(t time.Time) schema.Datetime {
	dt, _ := schema.NewDatetime(t)
	return dt
}

func NewTimestamp(t time.Time) schema.Timestamp {
	ts, _ := schema.NewTimestamp(t)
	return ts
}

func NewInterval(d time.Duration) schema.Interval {
	i, _ := schema.NewInterval(d)
	return i
}
