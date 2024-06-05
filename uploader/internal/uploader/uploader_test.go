package uploader

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yttest"
)

const testSheet = "Sheet1"

func TestMakeUploadRequest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		startRow int64
		rowCount int64
		header   bool
		types    bool
		columns  map[string]string
		append   bool
		create   bool
		expected *UploadRequest
		isError  bool
	}{
		{
			name: "default-overwrite",
			path: `//home/abc`,
			expected: &UploadRequest{
				Path:    "//home/abc",
				allRows: true,
			},
		},
		{
			name:   "default-append",
			path:   `//home/abc`,
			append: true,
			expected: &UploadRequest{
				Path:    "//home/abc",
				allRows: true,
				append:  true,
			},
		},
		{
			name:   "default-create",
			path:   `//home/abc`,
			create: true,
			expected: &UploadRequest{
				Path:    "//home/abc",
				allRows: true,
				create:  true,
			},
		},
		{
			name:   "all-rows-header-append",
			path:   `//home/abc`,
			header: true,
			append: true,
			expected: &UploadRequest{
				Path:     "//home/abc",
				Header:   true,
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
				append:   true,
			},
		},
		{
			name:    "column-mapping",
			path:    `//home/abc`,
			columns: map[string]string{"name": "A"},
			expected: &UploadRequest{
				Path:    "//home/abc",
				Columns: map[string]string{"name": "A"},
				allRows: true,
			},
		},
		{
			name:    "column-mapping-append",
			path:    `//home/abc`,
			columns: map[string]string{"name": "A"},
			append:  true,
			expected: &UploadRequest{
				Path:    "//home/abc",
				Columns: map[string]string{"name": "A"},
				allRows: true,
				append:  true,
			},
		},
		{
			name: "default",
			path: `//home/abc`,
			expected: &UploadRequest{
				Path:    "//home/abc",
				allRows: true,
			},
		},
		{
			name:     "start-row",
			path:     `//home/abc`,
			startRow: 2,
			expected: &UploadRequest{
				Path:     "//home/abc",
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
			},
		},
		{
			name:   "header",
			path:   `//home/abc`,
			header: true,
			expected: &UploadRequest{
				Path:     "//home/abc",
				Header:   true,
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
			},
		},
		{
			name:  "types",
			path:  `//home/abc`,
			types: true,
			expected: &UploadRequest{
				Path:     "//home/abc",
				Types:    true,
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
			},
		},
		{
			name:   "header-types",
			path:   `//home/abc`,
			header: true,
			types:  true,
			expected: &UploadRequest{
				Path:     "//home/abc",
				Header:   true,
				Types:    true,
				StartRow: 3,
				RowCount: ExcelMaxRowCount,
			},
		},
		{
			name:     "row-count",
			path:     `//home/abc`,
			rowCount: 9,
			expected: &UploadRequest{
				Path:     "//home/abc",
				StartRow: 1,
				RowCount: 9,
			},
		},
		{
			name:     "start-row-row-count",
			path:     `//home/abc`,
			startRow: 50,
			rowCount: 100,
			columns:  map[string]string{"name": "A", "id": "B"},
			expected: &UploadRequest{
				Path:     "//home/abc",
				Columns:  map[string]string{"name": "A", "id": "B"},
				StartRow: 50,
				RowCount: 100,
			},
		},
		{
			name:    "bad-path",
			path:    `//home/abc[`,
			isError: true,
		},
		{
			name:     "bad-row-count",
			path:     "//home/abc",
			startRow: 1,
			rowCount: ExcelMaxRowCount + 2,
			isError:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := MakeUploadRequest(tc.path, tc.startRow, tc.rowCount, "", tc.header, tc.types, tc.columns, tc.append, tc.create)
			if tc.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, req)
			}
		})
	}
}

type S1 struct {
	I64  int64  `yson:"i_64"`
	UI64 uint64 `yson:"ui_64"`
}

type S2 struct {
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

func TestUpload_existingTable(t *testing.T) {
	env, cancel := yttest.NewEnv(t)
	defer cancel()

	for _, tc := range []struct {
		name     string
		schema   schema.Schema
		req      *UploadRequest
		expected []any
		error    bool
	}{
		{
			name:   "all-columns",
			schema: schema.MustInfer(&S1{}),
			req: &UploadRequest{
				Path:    ypath.Path("//tmp/all-columns"),
				allRows: true,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 1,
					"A2": 2, "B2": 2,
				}),
			},
			expected: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 2, UI64: 2},
			},
		},
		{
			name:   "column-subset",
			schema: schema.MustInfer(&S1{}),
			req: &UploadRequest{
				Path:     ypath.Path(`//tmp/column-subset`),
				StartRow: 1,
				RowCount: ExcelMaxRowCount,
				Columns:  map[string]string{"i_64": "B", "ui_64": "C"},
				Data: makeExcelFile(t, table{
					"A1": "r", "B1": 1, "C1": 1, "D1": "x",
					"A2": "c", "B2": 2, "C2": 2, "D2": "y",
				}),
			},
			expected: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 2, UI64: 2},
			},
		},
		{
			name:   "row-subset",
			schema: schema.MustInfer(&S1{}),
			req: &UploadRequest{
				Path:     ypath.Path(`//tmp/row-subset`),
				StartRow: 2,
				RowCount: 2,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 1,
					"A2": 2, "B2": 2,
					"A3": 3, "B3": 3,
					"A4": 4, "B4": 4,
				}),
			},
			expected: []any{
				&S1{I64: 2, UI64: 2},
				&S1{I64: 3, UI64: 3},
			},
		},
		{
			name:   "reuse-excel-column",
			schema: schema.MustInfer(&S1{}),
			req: &UploadRequest{
				Path:    ypath.Path("//tmp/reuse-excel-column"),
				Columns: map[string]string{"i_64": "A", "ui_64": "A"},
				allRows: true,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 2,
					"A2": 1, "B2": 2,
				}),
			},
			expected: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 1, UI64: 1},
			},
		},
		{
			name:   "header",
			schema: schema.MustInfer(&S1{}),
			req: &UploadRequest{
				Path:     ypath.Path("//tmp/header"),
				Header:   true,
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
				Data: makeExcelFile(t, table{
					"A1": "ui_64", "B1": "not_yt_column", "C1": "i_64",
					"A2": 1, "C2": 2,
					"A3": 1, "C3": 2,
				}),
			},
			expected: []any{
				&S1{I64: 2, UI64: 1},
				&S1{I64: 2, UI64: 1},
			},
		},
		{
			name:   "missing-header",
			schema: schema.MustInfer(&S1{}),
			req: &UploadRequest{
				Path:     ypath.Path("//tmp/missing-header"),
				Header:   true,
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 2,
					"A2": 1, "B2": 2,
				}),
			},
			error: true,
		},
		{
			name:   "types",
			schema: schema.MustInfer(&S2{}),
			req: &UploadRequest{
				Path:    ypath.Path("//tmp/types"),
				allRows: true,
				Data: makeExcelFile(t, table{
					"A1": -16, "B1": 16,
					"C1": -32, "D1": 32,
					"E1": -64, "F1": 64,
					"G1": 32.3,
					"H1": 42.3,
					"I1": true,
					"J1": "hello",
					"K1": 25569,
					"L1": 25569.5,
					"M1": 25569.5,
					"N1": 1,
					"O1": []byte("[1;2;3]"),
				}),
			},
			expected: []any{
				&S2{
					I16: -16, UI16: 16,
					I32: -32, UI32: 32,
					I64: -64, UI64: 64,
					Float:     32.3,
					Double:    42.3,
					Bool:      true,
					String:    "hello",
					Date:      NewDate(time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)),
					Datetime:  NewDatetime(time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC)),
					Timestamp: NewTimestamp(time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC)),
					Interval:  schema.Interval(1),
					Any:       []any{int64(1), int64(2), int64(3)},
				},
			},
		},
	} {
		t.Run(tc.req.String(), func(t *testing.T) {
			saveExcelFile(t, tc.req.Data, tc.name+".xlsx")

			_, err := yt.CreateTable(env.Ctx, env.YT, tc.req.Path, yt.WithSchema(tc.schema))
			require.NoError(t, err)
			defer func() { _ = env.YT.RemoveNode(env.Ctx, tc.req.Path, nil) }()

			err = Upload(env.Ctx, env.YT, tc.req)
			if !tc.error {
				require.NoError(t, err)

				r, err := env.YT.ReadTable(env.Ctx, tc.req.Path, nil)
				require.NoError(t, err)
				defer func() { _ = r.Close() }()

				for i := 0; r.Next(); i++ {
					require.True(t, i < len(tc.expected))
					row := reflect.New(reflect.TypeOf(tc.expected[i]).Elem()).Interface()
					require.NoError(t, r.Scan(row))
					require.Equal(t, tc.expected[i], row)
				}

				require.NoError(t, r.Err())
			} else {
				require.Error(t, err)
			}
		})
	}
}

type S3 struct {
	I64  int64  `yson:"A"`
	UI64 uint64 `yson:"B"`
}

func TestUpload_createTable(t *testing.T) {
	env, cancel := yttest.NewEnv(t)
	defer cancel()

	for _, tc := range []struct {
		name     string
		req      *UploadRequest
		expected []any
		error    bool
	}{
		{
			name: "excel-column-names",
			req: &UploadRequest{
				Path:    ypath.Path("//tmp/excel-column-names"),
				allRows: true,
				create:  true,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 1,
					"A2": 2, "B2": 2,
				}),
			},
			expected: []any{
				&S3{I64: 1, UI64: 1},
				&S3{I64: 2, UI64: 2},
			},
		},
		{
			name: "column-mapping",
			req: &UploadRequest{
				Path:     ypath.Path(`//tmp/column-mapping`),
				StartRow: 1,
				RowCount: ExcelMaxRowCount,
				create:   true,
				Columns:  map[string]string{"i_64": "B", "ui_64": "C"},
				Data: makeExcelFile(t, table{
					"A1": "r", "B1": 1, "C1": 1, "D1": "x",
					"A2": "c", "B2": 2, "C2": 2, "D2": "y",
				}),
			},
			expected: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 2, UI64: 2},
			},
		},
		{
			name: "reuse-excel-column",
			req: &UploadRequest{
				Path:    ypath.Path("//tmp/reuse-excel-column"),
				Columns: map[string]string{"i_64": "A", "ui_64": "A"},
				allRows: true,
				create:  true,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 2,
					"A2": 1, "B2": 2,
				}),
			},
			expected: []any{
				&S1{I64: 1, UI64: 1},
				&S1{I64: 1, UI64: 1},
			},
		},
		{
			name: "header",
			req: &UploadRequest{
				Path:     ypath.Path("//tmp/header"),
				Header:   true,
				StartRow: 2,
				RowCount: ExcelMaxRowCount,
				create:   true,
				Data: makeExcelFile(t, table{
					"A1": "ui_64", "B1": "i_64",
					"A2": 1, "B2": 2,
					"A3": 1, "B3": 2,
				}),
			},
			expected: []any{
				&S1{I64: 2, UI64: 1},
				&S1{I64: 2, UI64: 1},
			},
		},
		{
			name: "types-first-row",
			req: &UploadRequest{
				Path:     ypath.Path("//tmp/types-first"),
				Types:    true,
				StartRow: 2,
				create:   true,
				Data: makeExcelFile(t, table{
					"A1": "int16", "B1": "uint16", "C1": "int32", "D1": "uint32", "E1": "int64", "F1": "uint64", "G1": "float", "H1": "double", "I1": "boolean", "J1": "utf8", "K1": "date", "L1": "datetime", "M1": "timestamp", "N1": "interval", "O1": "any",
					"A2": -16, "B2": 16, "C2": -32, "D2": 32, "E2": -64, "F2": 64, "G2": 32.3, "H2": 42.3, "I2": true, "J2": "hello", "K2": 25568, "L2": 25568.5, "M2": 25568.5, "N2": 1, "O2": []byte("[1;2;3]"),
				}),
			},
			expected: []any{
				&S2{
					I16: -16, UI16: 16,
					I32: -32, UI32: 32,
					I64: -64, UI64: 64,
					Float:     32.3,
					Double:    42.3,
					Bool:      true,
					String:    "hello",
					Date:      NewDate(time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)),
					Datetime:  NewDatetime(time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC)),
					Timestamp: NewTimestamp(time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC)),
					Interval:  schema.Interval(1),
					Any:       []any{int64(1), int64(2), int64(3)},
				},
			},
		},
		{
			name: "types-second-row",
			req: &UploadRequest{
				Path:     ypath.Path("//tmp/types-second"),
				Header:   true,
				Types:    true,
				StartRow: 3,
				create:   true,
				Data: makeExcelFile(t, table{
					"A1": "i_16", "B1": "ui_16", "C1": "i_32", "D1": "ui_32", "E1": "i_64", "F1": "ui_64", "G1": "float", "H1": "double", "I1": "bool", "J1": "string", "K1": "any",
					"A2": "int16", "B2": "uint16", "C2": "int32", "D2": "uint32", "E2": "int64", "F2": "uint64", "G2": "float", "H2": "double", "I2": "boolean", "J2": "utf8", "K2": "any",
					"A3": -16, "B3": 16, "C3": -32, "D3": 32, "E3": -64, "F3": 64, "G3": 32.3, "H3": 42.3, "I3": true, "J3": "hello", "K3": []byte("[1;2;3]"),
				}),
			},
			expected: []any{
				&S2{
					I16: -16, UI16: 16,
					I32: -32, UI32: 32,
					I64: -64, UI64: 64,
					Float:  32.3,
					Double: 42.3,
					Bool:   true,
					String: "hello",
					Any:    []any{int64(1), int64(2), int64(3)},
				},
			},
		},
		{
			name: "bad-type-row",
			req: &UploadRequest{
				Path:     ypath.Path("//tmp/bad-type-row"),
				Header:   true,
				Types:    true,
				StartRow: 3,
				RowCount: ExcelMaxRowCount,
				create:   true,
				Data: makeExcelFile(t, table{
					"A1": "id", "B1": "age",
					"A2": "int64", "B2": "some-bad-type",
					"A3": 1, "B3": 18,
				}),
			},
			error: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.req.create)

			saveExcelFile(t, tc.req.Data, tc.name+".xlsx")

			err := Upload(env.Ctx, env.YT, tc.req)
			if !tc.error {
				require.NoError(t, err)

				r, err := env.YT.ReadTable(env.Ctx, tc.req.Path, nil)
				require.NoError(t, err)
				defer func() { _ = r.Close() }()

				for i := 0; r.Next(); i++ {
					require.True(t, i < len(tc.expected))
					row := reflect.New(reflect.TypeOf(tc.expected[i]).Elem()).Interface()
					require.NoError(t, r.Scan(row))
					require.Equal(t, tc.expected[i], row)
				}

				require.NoError(t, r.Err())
			} else {
				require.Error(t, err)

				ok, err := env.YT.NodeExists(env.Ctx, tc.req.Path, nil)
				require.NoError(t, err)
				require.False(t, ok)
			}
		})
	}
}

func TestMakeSchema(t *testing.T) {
	for _, tc := range []struct {
		name     string
		req      *UploadRequest
		expected *schema.Schema
		error    bool
	}{
		{
			name: "column-mapping",
			req: &UploadRequest{
				Sheet:   testSheet,
				Columns: map[string]string{"id": "B", "age": "C"},
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 1,
					"A2": 2, "B2": 2,
				}),
			},
			// In this testcase types are not inferred at all.
			// Thus we don't care that there are no "C" column in data.
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "id", Type: schema.TypeAny},
					{Name: "age", Type: schema.TypeAny},
				},
			},
		},
		{
			name: "header",
			req: &UploadRequest{
				Sheet:    testSheet,
				Header:   true,
				StartRow: 2,
				Data: makeExcelFile(t, table{
					"A1": "id", "B1": "age", "D1": "name",
					"A2": 1, "B2": 1,
					"A3": 2, "B3": 2,
				}),
			},
			// In this testcase types are not inferred at all.
			// Column with empty name is skipped.
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "id", Type: schema.TypeAny},
					{Name: "age", Type: schema.TypeAny},
					{Name: "name", Type: schema.TypeAny},
				},
			},
		},
		{
			name: "excel-column-names",
			req: &UploadRequest{
				Sheet:    testSheet,
				StartRow: 1,
				Data: makeExcelFile(t, table{
					"A1": 1, "B1": 1,
					"A2": 2, "B2": 2,
				}),
			},
			// In this testcase types are not inferred at all.
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "A", Type: schema.TypeAny},
					{Name: "B", Type: schema.TypeAny},
				},
			},
		},
		{
			name: "type-row-first",
			req: &UploadRequest{
				Sheet:    testSheet,
				Types:    true,
				StartRow: 2,
				Data: makeExcelFile(t, table{
					"A1": "int64", "B1": "int32", "C1": "utf8",
					"A2": 42, "B2": 29, "C2": "Gopher",
				}),
			},
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "A", Type: schema.TypeInt64},
					{Name: "B", Type: schema.TypeInt32},
					{Name: "C", Type: schema.TypeString},
				},
			},
		},
		{
			name: "type-row-second",
			req: &UploadRequest{
				Sheet:    testSheet,
				Header:   true,
				Types:    true,
				StartRow: 3,
				Data: makeExcelFile(t, table{
					"A1": "id", "B1": "age", "C1": "name",
					"A2": "int64", "B2": "int32", "C2": "utf8",
					"A3": 42, "B3": 29, "C3": "Gopher",
				}),
			},
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "id", Type: schema.TypeInt64},
					{Name: "age", Type: schema.TypeInt32},
					{Name: "name", Type: schema.TypeString},
				},
			},
		},
		{
			name: "missing-header-row",
			req: &UploadRequest{
				Sheet:    testSheet,
				Header:   true,
				StartRow: 2,
				Data:     makeExcelFile(t, table{}),
			},
			error: true,
		},
		{
			name: "missing-type-row",
			req: &UploadRequest{
				Sheet:    testSheet,
				Header:   true,
				Types:    true,
				StartRow: 3,
				Data: makeExcelFile(t, table{
					"A1": "id", "B1": "age", "C1": "name",
				}),
			},
			error: true,
		},
		{
			name: "poor-type-row",
			req: &UploadRequest{
				Sheet:    testSheet,
				Header:   true,
				Types:    true,
				StartRow: 3,
				Columns:  map[string]string{"id": "A", "age": "B", "name": "C"},
				Data: makeExcelFile(t, table{
					"A1": "id", "B1": "age", // ignored
					"A2": "int64", "B2": "int32",
					"A3": 42, "B3": 29,
				}),
			},
			// In this testcase type row has no value in "C" column.
			// Thus YT column "name" has default type any.
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "id", Type: schema.TypeInt64},
					{Name: "age", Type: schema.TypeInt32},
					{Name: "name", Type: schema.TypeAny},
				},
			},
		},
		{
			name: "poor-column-mapping",
			req: &UploadRequest{
				Sheet:    testSheet,
				Header:   true,
				Types:    true,
				StartRow: 3,
				Columns:  map[string]string{"id": "A", "age": "B"},
				Data: makeExcelFile(t, table{
					"A1": "id", "B1": "age", "C1": "name", // ignored
					"A2": "int64", "B2": "int32", "C2": "utf8",
					"A3": 42, "B3": 29, "C3": "Gopher",
				}),
			},
			// In this testcase there are more columns in type row than in column mapping.
			// Column mapping is preferred.
			expected: &schema.Schema{
				Columns: []schema.Column{
					{Name: "id", Type: schema.TypeInt64},
					{Name: "age", Type: schema.TypeInt32},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := MakeSchema(tc.req)
			if tc.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, s)
			}
		})
	}
}

func TestGetColumnType(t *testing.T) {
	for _, tc := range []struct {
		typeStr  string
		expected schema.Type
		error    bool
	}{
		{typeStr: "int64", expected: schema.TypeInt64},
		{typeStr: "int32", expected: schema.TypeInt32},
		{typeStr: "int16", expected: schema.TypeInt16},
		{typeStr: "int8", expected: schema.TypeInt8},
		{typeStr: "uint64", expected: schema.TypeUint64},
		{typeStr: "uint32", expected: schema.TypeUint32},
		{typeStr: "uint16", expected: schema.TypeUint16},
		{typeStr: "uint8", expected: schema.TypeUint8},
		{typeStr: "float", expected: schema.TypeFloat32},
		{typeStr: "double", expected: schema.TypeFloat64},
		{typeStr: "boolean", expected: schema.TypeBoolean},
		{typeStr: "string", expected: schema.TypeBytes},
		{typeStr: "utf8", expected: schema.TypeString},
		{typeStr: "any", expected: schema.TypeAny},
		{typeStr: "date", expected: schema.TypeDate},
		{typeStr: "datetime", expected: schema.TypeDatetime},
		{typeStr: "timestamp", expected: schema.TypeTimestamp},
		{typeStr: "interval", expected: schema.TypeInterval},
		{typeStr: "some-bad-type", expected: schema.Type("some-bad-type")}, // no error
	} {
		t.Run(tc.typeStr, func(t *testing.T) {
			typ, err := GetColumnType(tc.typeStr)
			if tc.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, typ)
			}
		})
	}
}

func TestConvertDate(t *testing.T) {
	for _, tc := range []struct {
		value    string
		expected schema.Date
		error    bool
	}{
		{value: "25569", expected: NewDate(time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC))},
		{value: "1.5", error: true},
		{value: "-1", error: true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			date, err := convertDate(tc.value)
			if tc.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(tc.expected), int64(date))
			}
		})
	}
}

func TestConvertDatetime(t *testing.T) {
	for _, tc := range []struct {
		value    string
		expected schema.Datetime
		error    bool
	}{
		{value: "25569.5", expected: NewDatetime(time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC))},
		{value: "-1", error: true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			datetime, err := convertDatetime(tc.value)
			if tc.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, datetime)
			}
		})
	}
}

func TestConvertTimestamp(t *testing.T) {
	for _, tc := range []struct {
		value    string
		expected schema.Timestamp
		error    bool
	}{
		{value: "25569", expected: NewTimestamp(time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC))},
		{value: "-1", error: true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			timestamp, err := convertTimestamp(tc.value)
			if tc.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, timestamp)
			}
		})
	}
}

type (
	axis  string
	table map[axis]any
)

func makeExcelFile(t *testing.T, table table) *excelize.File {
	t.Helper()

	f := excelize.NewFile()
	for axis, value := range table {
		require.NoError(t, f.SetCellValue(testSheet, string(axis), value))
	}

	return f
}

func saveExcelFile(t *testing.T, f *excelize.File, path string) {
	t.Helper()

	filename := OutputPath(path)
	outFile, err := os.Create(filename)
	require.NoError(t, err)
	t.Logf("Saving excel file to %q", filename)
	require.NoError(t, f.Write(outFile))
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
