package exporter

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/xuri/excelize/v2"

	"go.ytsaurus.tech/library/go/core/xerrors"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
)

const (
	// SheetName stores the name of the resulting excel sheet.
	SheetName          = "Sheet1"
	strTimestampFormat = "2006-01-02T15:04:05.999999Z"
	strDateFormat      = "2006-01-02"
	strDatetimeFormat  = "2006-01-02T15:04:05Z07:00"
	maxExcelStrLen     = 32767

	day = 24 * time.Hour

	// TODO: remove these types aliases when they are supported by schema package.
	typeDate32      schema.Type = "date32"
	typeDatetime64  schema.Type = "datetime64"
	typeTimestamp64 schema.Type = "timestamp64"
	typeInterval64  schema.Type = "interval64"
)

var (
	excelEpoch = time.Date(1900, time.January, 0, 0, 0, 0, 0, time.UTC)
	unixEpoch  = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)

	excelEpochOffsetDays         = unixEpoch.Add(day).Sub(excelEpoch).Hours() / 24
	excelEpochOffsetSeconds      = unixEpoch.Add(day).Sub(excelEpoch).Seconds()
	excelEpochOffsetMicroseconds = unixEpoch.Add(day).Sub(excelEpoch).Microseconds()
)

// asInt64 converts v to int64.
// It is used only for YT types with a signed integer representation.
func asInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case int32:
		return int64(val), nil
	case int:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int8:
		return int64(val), nil
	case uint64:
		if val > math.MaxInt64 {
			return 0, xerrors.Errorf("uint64 value %d does not fit in int64", val)
		}
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case uint:
		if uint64(val) > math.MaxInt64 {
			return 0, xerrors.Errorf("uint value %d does not fit in int64", val)
		}
		return int64(val), nil
	default:
		return 0, xerrors.Errorf("expected integer, got %T", v)
	}
}

type converter struct {
	styles              *CellStyles
	numberPrecisionMode NumberPrecisionMode
}

func (c *converter) convertBytes(v any) (excelize.Cell, error) {
	data := v.(string)
	if runes := []rune(data); len(runes) > maxExcelStrLen {
		data = string(runes[:maxExcelStrLen])
	}
	return excelize.Cell{Value: data}, nil
}

func (c *converter) convertString(v any) (excelize.Cell, error) {
	return c.convertBytes(v)
}

func (c *converter) convertSmallIntegers(v any) (excelize.Cell, error) {
	return excelize.Cell{StyleID: c.styles.Number, Value: v}, nil
}

func (c *converter) convertLargeIntegers(v any) (excelize.Cell, error) {
	if fitsInNumber(v) {
		return c.convertSmallIntegers(v)
	}

	switch c.numberPrecisionMode {
	case NumberPrecisionModeError:
		return excelize.Cell{}, xerrors.Errorf("can not fit %d in excel; use another handle of long numbers", v)
	case NumberPrecisionModeString:
		return excelize.Cell{Value: fmt.Sprintf("%v", v)}, nil
	case NumberPrecisionModeLose:
		return c.convertSmallIntegers(v)
	}
	return excelize.Cell{}, xerrors.Errorf("long numbers handle not recognized")
}

func (c *converter) convertFloat(v any) (excelize.Cell, error) {
	if fitsInNumber(v) {
		return excelize.Cell{Value: v}, nil
	}

	switch c.numberPrecisionMode {
	case NumberPrecisionModeError:
		return excelize.Cell{}, xerrors.Errorf("can not fit %g in excel; use another long numbers handle", v)
	case NumberPrecisionModeString:
		return excelize.Cell{Value: fmt.Sprintf("%v", v)}, nil
	case NumberPrecisionModeLose:
		return excelize.Cell{Value: v}, nil
	}
	return excelize.Cell{}, xerrors.Errorf("long numbers handle not recognized")
}

func (c *converter) convertBool(v any) (excelize.Cell, error) {
	return excelize.Cell{Value: v}, nil
}

func (c *converter) convertAny(v any) (excelize.Cell, error) {
	data, err := yson.Marshal(v)
	if err != nil {
		return excelize.Cell{}, xerrors.Errorf("error converting %s to yson: %w", v, err)
	}

	if runes := []rune(string(data)); len(runes) > maxExcelStrLen {
		data = []byte(string(runes[:maxExcelStrLen]))
	}

	return excelize.Cell{Value: data}, nil
}

func (c *converter) convertDate(v any) (excelize.Cell, error) {
	excelDate := v.(uint64) + uint64(excelEpochOffsetDays)
	return excelize.Cell{StyleID: c.styles.Date, Value: excelDate}, nil
}

func (c *converter) convertDatetime(v any) (excelize.Cell, error) {
	excelDateTime := float64(v.(uint64)+uint64(excelEpochOffsetSeconds)) / 86400
	return excelize.Cell{StyleID: c.styles.Datetime, Value: excelDateTime}, nil
}

// convertTimestamps returns excel cell timestamp representation.
//
// Excel only supports millisecond time format.
// Returned cell will only have Number format for timestamps that have millisecond precision.
// All other timestamps are written as strings without information loss.
func (c *converter) convertTimestamp(v any) (excelize.Cell, error) {
	if v.(uint64)%1000 == 0 {
		excelTimestamp := float64(v.(uint64)+uint64(excelEpochOffsetMicroseconds)) / 86400 / 1e6
		return excelize.Cell{StyleID: c.styles.Timestamp, Value: excelTimestamp}, nil
	}

	t := int64(v.(uint64))
	str := time.Unix(t/1e6, (t%1e6)*1e3).UTC().Format(strTimestampFormat)
	return excelize.Cell{Value: str}, nil
}

func (c *converter) convertInterval(v any) (excelize.Cell, error) {
	return c.convertLargeIntegers(v)
}

// convertDate32 converts YT date32 to an Excel date serial number.
//
// YT date32 is a signed number of days since January 1, 1970.
// Excel date is a number of days since January 1, 1900.
// Unlike date, date32 can represent values before the Unix epoch.
// Values before January 1, 1900 cannot be represented as Excel dates,
// so they are written as ISO date strings without information loss.
func (c *converter) convertDate32(v any) (excelize.Cell, error) {
	days, err := asInt64(v)
	if err != nil {
		return excelize.Cell{}, err
	}
	excelDate := float64(days) + excelEpochOffsetDays
	if excelDate < 1 {
		t := time.Unix(days*86400, 0).UTC()
		return excelize.Cell{Value: t.Format(strDateFormat)}, nil
	}
	return excelize.Cell{StyleID: c.styles.Date, Value: excelDate}, nil
}

// convertDatetime64 converts YT datetime64 to an Excel datetime serial number.
//
// YT datetime64 is a signed number of seconds since January 1, 1970.
// Excel datetime is a floating-point day count since January 1, 1900,
// where the fractional part represents the time of day.
// Unlike datetime, datetime64 can represent values before the Unix epoch.
// Values before January 1, 1900 cannot be represented as Excel datetimes,
// so they are written as ISO datetime strings without information loss.
func (c *converter) convertDatetime64(v any) (excelize.Cell, error) {
	seconds, err := asInt64(v)
	if err != nil {
		return excelize.Cell{}, err
	}
	excelDateTime := (float64(seconds) + excelEpochOffsetSeconds) / 86400
	if excelDateTime < 1 {
		t := time.Unix(seconds, 0).UTC()
		return excelize.Cell{Value: t.Format(strDatetimeFormat)}, nil
	}
	return excelize.Cell{StyleID: c.styles.Datetime, Value: excelDateTime}, nil
}

// convertTimestamp64 returns excel cell timestamp64 representation.
//
// Excel only supports millisecond time format.
// Returned cell will only have Number format for timestamps that have millisecond precision.
// All other timestamps are written as strings without information loss.
// Values before January 1, 1900 cannot be represented as Excel timestamps,
// so they are also written as strings.
func (c *converter) convertTimestamp64(v any) (excelize.Cell, error) {
	micros, err := asInt64(v)
	if err != nil {
		return excelize.Cell{}, err
	}
	excelTimestamp := float64(micros + excelEpochOffsetMicroseconds) / 86400 / 1e6
	if excelTimestamp < 1 || micros%1000 != 0 {
		str := time.Unix(micros/1e6, (micros%1e6)*1e3).UTC().Format(strTimestampFormat)
		return excelize.Cell{Value: str}, nil
	}
	return excelize.Cell{StyleID: c.styles.Timestamp, Value: excelTimestamp}, nil
}

// convertInterval64 converts YT interval64 to an Excel cell.
//
// YT interval64 is a signed number of microseconds between two timestamps.
// It is exported as a plain integer (not an Excel date), using the same
// large-integer handling as interval and int64/uint64.
func (c *converter) convertInterval64(v any) (excelize.Cell, error) {
	return c.convertLargeIntegers(v)
}

func (c *converter) convert(t schema.Type, v any) (excelize.Cell, error) {
	switch t {
	case schema.TypeBytes:
		return c.convertBytes(v)
	case schema.TypeString:
		return c.convertString(v)
	case schema.TypeInt8, schema.TypeUint8, schema.TypeInt16, schema.TypeUint16,
		schema.TypeInt32, schema.TypeUint32:
		return c.convertSmallIntegers(v)
	case schema.TypeInt64, schema.TypeUint64:
		return c.convertLargeIntegers(v)
	case schema.TypeFloat32:
		return c.convertFloat(v)
	case schema.TypeFloat64:
		return c.convertFloat(v)
	case schema.TypeBoolean:
		return c.convertBool(v)
	case schema.TypeDate:
		return c.convertDate(v)
	case schema.TypeDatetime:
		return c.convertDatetime(v)
	case schema.TypeTimestamp:
		return c.convertTimestamp(v)
	case schema.TypeInterval:
		return c.convertInterval(v)
	case typeDate32:
		return c.convertDate32(v)
	case typeDatetime64:
		return c.convertDatetime64(v)
	case typeTimestamp64:
		return c.convertTimestamp64(v)
	case typeInterval64:
		return c.convertInterval64(v)
	case schema.TypeAny:
		return c.convertAny(v)
	default:
		return excelize.Cell{Value: "UNSUPPORTED"}, nil
	}
}

func (c *converter) convertAuto(v any) (excelize.Cell, error) {
	switch val := v.(type) {
	case nil:
		return excelize.Cell{}, nil
	case string:
		return c.convertString(val)
	case []byte:
		return c.convertBytes(string(val))
	case int, int8, int16, int32, uint, uint8, uint16, uint32:
		return c.convertSmallIntegers(val)
	case int64, uint64:
		return c.convertLargeIntegers(val)
	case float32, float64:
		return c.convertFloat(val)
	case bool:
		return c.convertBool(val)
	case any:
		return c.convertAny(val)
	default:
		return excelize.Cell{Value: "UNSUPPORTED"}, nil
	}
}

// Column is a schema.Column with additional index excel field.
type Column struct {
	Index int
	schema.Column
}

type ConvertOptions struct {
	Columns             []string
	Schema              *schema.Schema
	ExportOptions       *ExportOptions
	NumberPrecisionMode NumberPrecisionMode
	OmitTypes           bool
}

func Convert(r yt.TableReader, opts *ConvertOptions) (*excelize.File, error) {
	out := excelize.NewFile()

	hasSchema := opts.Schema != nil && len(opts.Schema.Columns) > 0

	var nameToCol map[string]*Column
	var nextColIndex int

	if hasSchema {
		nameToCol = makeHeader(opts.Columns, opts.Schema)
		if err := writeHeader(nameToCol, out, opts.OmitTypes); err != nil {
			return nil, err
		}
		for _, col := range nameToCol {
			if col.Index >= nextColIndex {
				nextColIndex = col.Index + 1
			}
		}
	} else {
		nameToCol = map[string]*Column{}
		nextColIndex = 1
	}

	updateHeaders := func(row map[string]any) error {
		var newKeys []string

		for k := range row {
			if _, ok := nameToCol[k]; !ok {
				newKeys = append(newKeys, k)
			}
		}

		slices.Sort(newKeys)

		for _, k := range newKeys {
			col := &Column{Index: nextColIndex}
			nameToCol[k] = col

			axis, _ := excelize.CoordinatesToCellName(col.Index, 1)
			if err := out.SetCellValue(SheetName, axis, k); err != nil {
				return err
			}
			nextColIndex++
		}
		return nil
	}

	styles, err := registerCellStyles(out)
	if err != nil {
		return nil, err
	}

	c := &converter{styles: styles, numberPrecisionMode: opts.NumberPrecisionMode}

	convert := func(col *Column, v any) (excelize.Cell, error) {
		if hasSchema {
			return c.convert(col.Type, v)
		}
		return c.convertAuto(v)
	}

	totalRowWeight := 0
	excelRowNumber := 2
	if hasSchema && !opts.OmitTypes {
		excelRowNumber++
	}

	for r.Next() {
		var row map[string]any
		err = r.Scan(&row)
		if err != nil {
			return nil, xerrors.Errorf("error reading table row: %w", err)
		}

		if !hasSchema {
			if err := updateHeaders(row); err != nil {
				return nil, err
			}
		}

		excelRow := make(map[int]excelize.Cell)
		for k, v := range row {
			if v == nil {
				continue
			}

			col, ok := nameToCol[k]
			if !ok {
				return nil, xerrors.Errorf("unable to find column %s in schema %+v", k, nameToCol)
			}

			cell, err := convert(col, v)
			if err != nil {
				errRowIndex := excelRowNumber - 1
				if hasSchema {
					errRowIndex = excelRowNumber - 2
					if !opts.OmitTypes {
						errRowIndex = excelRowNumber - 3
					}
				}
				return nil, fmt.Errorf("error converting value from column %s and row %d: %w", k, errRowIndex, err)
			}

			excelRow[col.Index] = cell
		}

		for i, cell := range excelRow {
			axis, _ := excelize.CoordinatesToCellName(i, excelRowNumber)
			if err := out.SetCellStyle(SheetName, axis, axis, cell.StyleID); err != nil {
				return nil, err
			}
			if err := out.SetCellValue(SheetName, axis, cell.Value); err != nil {
				return nil, err
			}
		}

		// Even if stream writer is used excelize will materialize the whole excel file in memory on write.
		// That's why a special heuristic is used to control the output file size.
		// todo remove when https://github.com/360EntSecGroup-Skylar/excelize/issues/650 is resolved.
		totalRowWeight += rowWeight(excelRow)
		if totalRowWeight >= opts.ExportOptions.MaxExcelFileSize {
			return nil, xerrors.Errorf("max total row weight exceeded: %v >= %v; "+
				"try specifying a smaller range of rows or exclude unneeded columns",
				datasize.ByteSize(totalRowWeight).HumanReadable(),
				datasize.ByteSize(opts.ExportOptions.MaxExcelFileSize).HumanReadable())
		}

		excelRowNumber++
	}

	if r.Err() != nil {
		return nil, xerrors.Errorf("error reading data: %w", r.Err())
	}

	return out, nil
}

// makeHeader creates mapping from column name to indexed excel column.
//
// Indexing is based on the column order of the table schema.
func makeHeader(columns []string, s *schema.Schema) map[string]*Column {
	columnSet := make(map[string]struct{})
	for _, col := range columns {
		columnSet[col] = struct{}{}
	}

	header := make(map[string]*Column)
	index := 0
	for _, c := range s.Columns {
		if _, ok := columnSet[c.Name]; !ok {
			continue
		}

		index++
		header[c.Name] = &Column{
			Index:  index,
			Column: c,
		}

	}

	return header
}

// writeHeader writes column names on the first row of the sheet and,
// unless omitTypes is set, their types on the second.
func writeHeader(header map[string]*Column, w *excelize.File, omitTypes bool) error {
	for name, col := range header {
		axis, _ := excelize.CoordinatesToCellName(col.Index, 1)
		if err := w.SetCellValue(SheetName, axis, name); err != nil {
			return err
		}

		if !omitTypes {
			axis, _ = excelize.CoordinatesToCellName(col.Index, 2)
			if err := w.SetCellValue(SheetName, axis, col.Column.Type); err != nil {
				return err
			}
		}
	}

	return nil
}

type CellStyles struct {
	Number, Date, Datetime, Timestamp int
}

func registerCellStyles(f *excelize.File) (*CellStyles, error) {
	numberNumFmt := "0"
	numberFormat, err := f.NewStyle(&excelize.Style{CustomNumFmt: &numberNumFmt})
	if err != nil {
		return nil, err
	}

	dateNumFmt := "yyyy-mm-dd"
	dateFormat, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateNumFmt})
	if err != nil {
		return nil, err
	}

	datetimeNumFmt := "yyyy-mm-ddThh:mm:ssZ"
	datetimeFormat, err := f.NewStyle(&excelize.Style{CustomNumFmt: &datetimeNumFmt})
	if err != nil {
		return nil, err
	}

	timestampNumFmt := "yyyy-mm-ddThh:mm:ss.000Z"
	timestampFormat, err := f.NewStyle(&excelize.Style{CustomNumFmt: &timestampNumFmt})
	if err != nil {
		return nil, err
	}

	s := &CellStyles{
		Number:    numberFormat,
		Date:      dateFormat,
		Datetime:  datetimeFormat,
		Timestamp: timestampFormat,
	}

	return s, nil
}

// fitsInNumber checks whether numeric type can be converted to excel number type,
// which is 64-bit float value with 15 digit precision.
func fitsInNumber(f any) bool {
	s := fmt.Sprintf("%v", f)
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return r == '0'
	})
	s = strings.TrimPrefix(s, ".")
	s = strings.TrimRightFunc(s, func(r rune) bool {
		return r == '0'
	})
	return len(s) <= 15
}
