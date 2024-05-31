package exporter

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/yson"
)

func TestMakeHeader(t *testing.T) {
	for _, tc := range []struct {
		name    string
		columns []string
		schema  *schema.Schema
		header  map[string]*Column
	}{
		{
			name:    "subset",
			columns: []string{"age", "name"},
			schema: &schema.Schema{
				Columns: []schema.Column{
					{Name: "id", Type: schema.TypeInt64},
					{Name: "name", Type: schema.TypeString},
					{Name: "date", Type: schema.TypeDate},
					{Name: "age", Type: schema.TypeInt32},
					{Name: "extra", Type: schema.TypeAny},
				},
			},
			header: map[string]*Column{
				"name": {Index: 1, Column: schema.Column{Name: "name", Type: schema.TypeString}},
				"age":  {Index: 2, Column: schema.Column{Name: "age", Type: schema.TypeInt32}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.header, makeHeader(tc.columns, tc.schema))
		})
	}
}

func TestRegisterCellStyles(t *testing.T) {
	f := excelize.NewFile()
	_, err := registerCellStyles(f)
	require.NoError(t, err)
}

func TestFitsInNumber(t *testing.T) {
	for _, tc := range []struct {
		in   any
		fits bool
	}{
		{in: int16(-16), fits: true},
		{in: int64(-64), fits: true},
		{in: int64(-4291747100000000), fits: true},
		{in: int64(-4291747100000001), fits: false},
		{in: int64(4291747100000000), fits: true},
		{in: int64(4291747100000001), fits: false},
		{in: uint64(4291747100000000), fits: true},
		{in: uint64(4291747100000001), fits: false},
		{in: 0.000000000000000016, fits: true},
		{in: 0.001000000000000016, fits: false},
	} {
		t.Run(fmt.Sprintf("%T_%v_%t", tc.in, tc.in, tc.fits), func(t *testing.T) {
			require.Equal(t, tc.fits, fitsInNumber(tc.in))
		})
	}
}

func TestConverter(t *testing.T) {
	styles := &CellStyles{Number: 1, Date: 2, Datetime: 3, Timestamp: 4}

	c := converter{styles: styles}

	for _, tc := range []struct {
		name    string
		colType schema.Type
		in      any
		cell    excelize.Cell
	}{
		{
			name:    "int16",
			colType: schema.TypeInt16,
			in:      int16(-16),
			cell:    excelize.Cell{StyleID: styles.Number, Value: int16(-16)},
		},
		{
			name:    "uint16",
			colType: schema.TypeUint16,
			in:      uint16(16),
			cell:    excelize.Cell{StyleID: styles.Number, Value: uint16(16)},
		}, {
			name:    "int32",
			colType: schema.TypeInt32,
			in:      int32(-32),
			cell:    excelize.Cell{StyleID: styles.Number, Value: int32(-32)},
		},
		{
			name:    "uint32",
			colType: schema.TypeUint32,
			in:      uint32(32),
			cell:    excelize.Cell{StyleID: styles.Number, Value: uint32(32)},
		},
		{
			name:    "small-int64",
			colType: schema.TypeInt64,
			in:      int64(-64),
			cell:    excelize.Cell{StyleID: styles.Number, Value: int64(-64)},
		},
		{
			name:    "small-uint64",
			colType: schema.TypeUint64,
			in:      uint64(64),
			cell:    excelize.Cell{StyleID: styles.Number, Value: uint64(64)},
		},
		{
			name:    "large-int64",
			colType: schema.TypeInt64,
			in:      int64(-4291747199999999),
			cell:    excelize.Cell{Value: "-4291747199999999"},
		},
		{
			name:    "large-uint64",
			colType: schema.TypeUint64,
			in:      uint64(4291747199999999),
			cell:    excelize.Cell{Value: "4291747199999999"},
		},
		{
			name:    "small-precision-float",
			colType: schema.TypeFloat32,
			in:      0.00016,
			cell:    excelize.Cell{Value: 0.00016},
		},
		{
			name:    "small-precision-double",
			colType: schema.TypeFloat64,
			in:      0.000000000000000016,
			cell:    excelize.Cell{Value: 0.000000000000000016},
		},
		{
			name:    "large-precision-double",
			colType: schema.TypeFloat64,
			in:      0.001000000000000016,
			cell:    excelize.Cell{Value: "0.001000000000000016"},
		},
		{
			name:    "bool",
			colType: schema.TypeBoolean,
			in:      true,
			cell:    excelize.Cell{Value: true},
		},
		{
			name:    "small-string",
			colType: schema.TypeString,
			in:      "hello",
			cell:    excelize.Cell{Value: "hello"},
		},
		{
			name:    "large-string",
			colType: schema.TypeString,
			in:      strings.Repeat("a", maxExcelStrLen+1),
			cell:    excelize.Cell{Value: strings.Repeat("a", maxExcelStrLen)},
		},
		{
			name:    "date",
			colType: schema.TypeDate,
			in:      uint64(time.Date(2000, time.December, 15, 12, 00, 00, 0, time.UTC).Unix() / 86400),
			cell:    excelize.Cell{StyleID: styles.Date, Value: uint64(36875)},
		},
		{
			name:    "datetime",
			colType: schema.TypeDatetime,
			in:      uint64(time.Date(2000, time.December, 15, 12, 00, 00, 0, time.UTC).Unix()),
			cell:    excelize.Cell{StyleID: styles.Datetime, Value: 36875.5},
		},
		{
			name:    "millisecond-timestamp",
			colType: schema.TypeTimestamp,
			in:      uint64(time.Date(2000, time.December, 15, 12, 00, 00, 0, time.UTC).UnixNano() / 1e3),
			cell:    excelize.Cell{StyleID: styles.Timestamp, Value: 36875.5},
		},
		{
			name:    "microsecond-timestamp",
			colType: schema.TypeTimestamp,
			in:      uint64(time.Date(2000, time.December, 15, 12, 00, 00, 1100, time.UTC).UnixNano()) / 1e3,
			cell:    excelize.Cell{Value: "2000-12-15T12:00:00.000001Z"},
		},
		{
			name:    "small-interval",
			colType: schema.TypeInterval,
			in:      NewInterval(time.Hour),
			cell:    excelize.Cell{StyleID: styles.Number, Value: schema.Interval(60 * 60 * 1000 * 1000)},
		},
		{
			name:    "large-interval",
			colType: schema.TypeInterval,
			in:      NewInterval(time.Duration(4291747199999999000)),
			cell:    excelize.Cell{Value: "4291747199999999"},
		},
		{
			name:    "any-struct",
			colType: schema.TypeAny,
			in: struct {
				Age int
			}{Age: 42},
			cell: excelize.Cell{Value: []byte("{Age=42;}")},
		},
		{
			name:    "any-raw-yson",
			colType: schema.TypeAny,
			in:      yson.RawValue("{Name=var;}"),
			cell:    excelize.Cell{Value: []byte("{Name=var;}")},
		},
		{
			name:    "any-large-string",
			colType: schema.TypeAny,
			in:      strings.Repeat("a", maxExcelStrLen+1),
			cell:    excelize.Cell{Value: []byte(strings.Repeat("a", maxExcelStrLen))},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c.numberPrecisionMode = NumberPrecisionModeString
			cell, err := c.convert(tc.colType, tc.in)

			require.NoError(t, err)
			require.Equal(t, tc.cell, cell)
		})
	}
}

func TestConverterError(t *testing.T) {
	c := converter{numberPrecisionMode: NumberPrecisionModeError}

	for _, tc := range []struct {
		name    string
		colType schema.Type
		in      any
	}{
		{
			name:    "large-uint64-with-error",
			colType: schema.TypeUint64,
			in:      uint64(4291747199999999),
		},
		{
			name:    "large-precision-double-with-error",
			colType: schema.TypeFloat64,
			in:      0.001000000000000016,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.convert(tc.colType, tc.in)
			require.Error(t, err)
		})
	}
}
