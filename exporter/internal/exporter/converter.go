package exporter

import (
	"fmt"
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
	maxExcelStrLen     = 32767

	day = 24 * time.Hour
)

var (
	excelEpoch = time.Date(1900, time.January, 0, 0, 0, 0, 0, time.UTC)
	unixEpoch  = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
)

type converter struct {
	styles              *CellStyles
	numberPrecisionMode NumberPrecisionMode
}

func (c *converter) convertBytes(v any) (excelize.Cell, error) {
	data := v.(string)
	if len(data) > maxExcelStrLen {
		data = data[:maxExcelStrLen]
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

	if len(data) > maxExcelStrLen {
		data = data[:maxExcelStrLen]
	}

	return excelize.Cell{Value: data}, nil
}

func (c *converter) convertDate(v any) (excelize.Cell, error) {
	excelDate := v.(uint64) + uint64(unixEpoch.Add(day).Sub(excelEpoch).Hours()/24)
	return excelize.Cell{StyleID: c.styles.Date, Value: excelDate}, nil
}

func (c *converter) convertDatetime(v any) (excelize.Cell, error) {
	excelDateTime := float64(v.(uint64)+uint64(unixEpoch.Add(day).Sub(excelEpoch).Seconds())) / 86400
	return excelize.Cell{StyleID: c.styles.Datetime, Value: excelDateTime}, nil
}

// convertTimestamps returns excel cell timestamp representation.
//
// Excel only supports millisecond time format.
// Returned cell will only have Number format for timestamps that have millisecond precision.
// All other timestamps are written as strings without information loss.
func (c *converter) convertTimestamp(v any) (excelize.Cell, error) {
	if v.(uint64)%1000 == 0 {
		excelTimestamp := float64(v.(uint64)+uint64(unixEpoch.Add(day).Sub(excelEpoch).Microseconds())) / 86400 / 1e6
		return excelize.Cell{StyleID: c.styles.Timestamp, Value: excelTimestamp}, nil
	}

	t := int64(v.(uint64))
	str := time.Unix(t/1e6, (t%1e6)*1e3).UTC().Format(strTimestampFormat)
	return excelize.Cell{Value: str}, nil
}

func (c *converter) convertInterval(v any) (excelize.Cell, error) {
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
	case schema.TypeAny:
		return c.convertAny(v)
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
}

func Convert(r yt.TableReader, opts *ConvertOptions) (*excelize.File, error) {
	out := excelize.NewFile()

	nameToCol := makeHeader(opts.Columns, opts.Schema)
	if err := writeHeader(nameToCol, out); err != nil {
		return nil, err
	}

	styles, err := registerCellStyles(out)
	if err != nil {
		return nil, err
	}

	c := &converter{styles: styles, numberPrecisionMode: opts.NumberPrecisionMode}

	totalRowWeight := 0

	excelRowNumber := 3
	for r.Next() {
		var row map[string]any
		err = r.Scan(&row)
		if err != nil {
			return nil, xerrors.Errorf("error reading table row: %w", err)
		}

		excelRow := make([]any, len(row))
		for k, v := range row {
			col, ok := nameToCol[k]
			if !ok {
				return nil, xerrors.Errorf("unable to find column %s in schema %+v", k, nameToCol)
			}

			if v == nil {
				excelRow[col.Index-1] = nil
				continue
			}

			cell, err := c.convert(col.Type, v)
			if err != nil {
				return nil, fmt.Errorf("error converting value from column %s and row %d: %w", k, excelRowNumber-3, err)
			}

			excelRow[col.Index-1] = cell
		}

		for i, v := range excelRow {
			if v == nil {
				continue
			}
			cell := v.(excelize.Cell)
			axis, _ := excelize.CoordinatesToCellName(i+1, excelRowNumber)
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

// writeHeader writes column names on the first row of the sheet and
// their types on the second.
func writeHeader(header map[string]*Column, w *excelize.File) error {
	for name, col := range header {
		axis, _ := excelize.CoordinatesToCellName(col.Index, 1)
		if err := w.SetCellValue(SheetName, axis, name); err != nil {
			return err
		}

		axis, _ = excelize.CoordinatesToCellName(col.Index, 2)
		if err := w.SetCellValue(SheetName, axis, col.Column.Type); err != nil {
			return err
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
