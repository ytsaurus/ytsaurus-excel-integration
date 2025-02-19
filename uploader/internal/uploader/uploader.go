package uploader

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"go.ytsaurus.tech/library/go/core/xerrors"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
)

const (
	ExcelMaxRowCount = 1048576
	ExcelMaxColCount = 16384

	day = 24 * time.Hour
)

var (
	excelEpoch = time.Date(1900, time.January, 0, 0, 0, 0, 0, time.UTC)
	unixEpoch  = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// UploadRequest represents a request to upload excel file to static yt table with strict schema.
type UploadRequest struct {
	Path  ypath.Path `json:"path"`
	Sheet string     `json:"sheet"`

	Header   bool              `json:"header"`
	Types    bool              `json:"types"`
	Columns  map[string]string `json:"columns"`
	allRows  bool
	StartRow int64 `json:"start_row"`
	RowCount int64 `json:"row_count"`

	append bool
	create bool

	Data *excelize.File `json:"-"`
}

// MakeUploadRequest creates and validates request object.
//
// Example paths:
//
//	//home/example[#10:#999]
//	//home/example
//
// sheet is an optional excel sheet name. If not specified the first one is used.
//
// columns is an optional mapping from YT column name to excel column.
//
// header is a flag that makes uploader read column mapping from the first excel row.
//
// types is a flag that makes uploader read type mapping from the second excel row if header is true,
// or the first row if header is false.
//
// If neither columns nor header are used columns are mapped by position.
//
// append controls append/overwrite behaviour.
//
// create is a flag that makes uploader create table with schema inferred from header or column mapping.
func MakeUploadRequest(
	path string,
	startRow, rowCount int64,
	sheet string,
	header bool,
	types bool,
	columns map[string]string,
	append bool,
	create bool,
) (*UploadRequest, error) {
	p, err := ypath.Parse(path)
	if err != nil {
		return nil, err
	}

	r := &UploadRequest{
		Path:     p.Path,
		StartRow: startRow,
		RowCount: rowCount,
		Sheet:    sheet,
		Header:   header,
		Types:    types,
		Columns:  columns,
		allRows:  true,
		append:   append,
		create:   create,
	}

	r.allRows = startRow == 0 && rowCount == 0
	if !r.allRows {
		if r.StartRow == 0 {
			r.StartRow = 1
		}
		if r.RowCount == 0 {
			r.RowCount = ExcelMaxRowCount
		}
	}

	if r.allRows {
		startRow := int64(1)
		if r.Header {
			startRow++
		}
		if r.Types {
			startRow++
		}

		if startRow >= 2 {
			r.StartRow = startRow
			r.allRows = false
			r.RowCount = ExcelMaxRowCount
		}
	}

	if r.StartRow < 0 {
		return nil, xerrors.Errorf("start row cannot be negative; got %d", r.StartRow)
	}

	if r.RowCount > ExcelMaxRowCount {
		return nil, xerrors.Errorf("too many rows to upload; max is %d", ExcelMaxRowCount)
	}

	for _, col := range r.Columns {
		if _, err := excelize.ColumnNameToNumber(col); err != nil {
			return nil, xerrors.Errorf("invalid column name %q: %w", col, err)
		}
	}

	return r, nil
}

// EnsureSheetName sets request sheet name.
//
// Does nothing if r.Sheet is not empty.
//
// Uses the first sheet with not empty name.
func (r *UploadRequest) EnsureSheetName() {
	if r.Sheet != "" {
		return
	}

	for _, sheet := range r.Data.GetSheetList() {
		if r.Data.GetSheetVisible(sheet) {
			r.Sheet = sheet
			break
		}
	}
}

// MakeColumnMapping creates column mapping.
func (r *UploadRequest) MakeColumnMapping(s *schema.Schema) error {
	if r.Header {
		return r.makeColumnMappingFromHeader(s)
	}
	return r.makeDefaultColumnMapping(s)
}

// makeDefaultColumnMapping reads column mapping from the first excel row.
func (r *UploadRequest) makeColumnMappingFromHeader(s *schema.Schema) error {
	row, err := r.readFirstRow()
	if err != nil {
		return err
	}

	ytColumnSet := makeColumnSet(s)

	mapping := make(map[string]string)
	for i, col := range row {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return xerrors.Errorf("unable to convert number %d to excel column: %w", i+1, err)
		}
		if _, ok := ytColumnSet[col]; ok {
			mapping[col] = name
		}
	}

	r.Columns = mapping
	return nil
}

func (r *UploadRequest) readFirstRow() ([]string, error) {
	rows, err := r.Data.Rows(r.Sheet)
	if err != nil {
		return nil, err
	}

	if rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			return nil, ErrBadRequest.Wrap(xerrors.Errorf("unable to read first row of sheet %q: %w", r.Sheet, err))
		}
		return row, nil
	}

	return nil, ErrBadRequest.Wrap(xerrors.Errorf("unable to read first row of sheet %q: %w", r.Sheet, rows.Error()))
}

func (r *UploadRequest) readSecondRow() ([]string, error) {
	rows, err := r.Data.Rows(r.Sheet)
	if err != nil {
		return nil, err
	}

	rows.Next()
	_, _ = rows.Columns()

	if rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			return nil, ErrBadRequest.Wrap(xerrors.Errorf("unable to read first row of sheet %q: %w", r.Sheet, err))
		}
		return row, nil
	}

	return nil, ErrBadRequest.Wrap(xerrors.Errorf("unable to read second row of sheet %q: %w", r.Sheet, rows.Error()))
}

func makeColumnSet(s *schema.Schema) map[string]struct{} {
	ret := make(map[string]struct{})
	for _, col := range s.Columns {
		ret[col.Name] = struct{}{}
	}
	return ret
}

// makeDefaultColumnMapping maps columns from schema to first excel column names e.g. A, B, C...
func (r *UploadRequest) makeDefaultColumnMapping(s *schema.Schema) error {
	mapping := make(map[string]string)
	for i, col := range s.Columns {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return xerrors.Errorf("unable to convert number %d to excel column: %w", i+1, err)
		}
		mapping[col.Name] = name
	}
	r.Columns = mapping
	return nil
}

func (r *UploadRequest) String() string {
	return fmt.Sprintf("Path: %s, Columns: %s, StartRow: %d, RowCount: %d, Append: %v",
		r.Path, r.Columns, r.StartRow, r.RowCount, r.append)
}

// ErrBadRequest is an error that signals that the upload has failed due to bad request.
var ErrBadRequest = xerrors.NewSentinel("bad request")

// ErrUnauthorized is an error that signals that uploader is missing some permissions to make an upload.
var ErrUnauthorized = xerrors.NewSentinel("unauthorized")

// Upload executes given upload request.
func Upload(ctx context.Context, yc yt.Client, req *UploadRequest) error {
	req.EnsureSheetName()

	tx, err := yc.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.Errorf("unable to start upload transaction: %w", err)
	}
	defer tx.Abort()

	if req.create {
		if err := CreateTable(ctx, tx, req); err != nil {
			return xerrors.Errorf("unable to create table: %w", err)
		}
	}

	s, err := ReadSchema(ctx, tx, req.Path)
	if err != nil {
		if yterrors.ContainsErrorCode(err, yterrors.CodeResolveError) {
			return ErrBadRequest.Wrap(xerrors.Errorf("error reading schema for %q: %w", req.Path, err))
		}
		if yterrors.ContainsErrorCode(err, yterrors.CodeAuthorizationError) {
			return ErrUnauthorized.Wrap(xerrors.Errorf("authorization error when reading table schema for %q: %w", req.Path, err))
		}
		return xerrors.Errorf("error reading schema for %q: %w", req.Path, err)
	}

	if len(req.Columns) == 0 {
		if err := req.MakeColumnMapping(s); err != nil {
			return err
		}
	}

	if len(req.Columns) != len(s.Columns) {
		err := xerrors.Errorf("schema has %d column(s), request - %d", len(s.Columns), len(req.Columns))
		return ErrBadRequest.Wrap(err)
	}

	if len(req.Columns) > ExcelMaxColCount {
		return ErrBadRequest.Wrap(xerrors.Errorf("exceeding max number of excel columns %d", ExcelMaxColCount))
	}

	out, err := tx.WriteTable(ctx, ypath.Rich{Path: req.Path, Append: &req.append}, nil)
	if err != nil {
		if yterrors.ContainsErrorCode(err, yterrors.CodeAuthorizationError) {
			return ErrUnauthorized.Wrap(xerrors.Errorf("authorization error when creating table writer: %w", err))
		}
		return xerrors.Errorf("error creating writer: %w", err)
	}

	if err := upload(req, s, out); err != nil {
		_ = out.Rollback()
		return xerrors.Errorf("error uploading %s: %w", req, err)
	}

	err = tx.Commit()
	if err != nil && yterrors.ContainsErrorCode(err, yterrors.CodeAuthorizationError) {
		return ErrUnauthorized.Wrap(err)
	}
	return err
}

func upload(req *UploadRequest, s *schema.Schema, out yt.TableWriter) error {
	columnToIndex := make(map[string]int)
	for i, col := range s.Columns {
		columnToIndex[col.Name] = i
	}

	excelColToYTCols := make(map[string][]int)
	for ytCol, excelCol := range req.Columns {
		excelColToYTCols[excelCol] = append(excelColToYTCols[excelCol], columnToIndex[ytCol])
	}

	rows, err := req.Data.Rows(req.Sheet)
	if err != nil {
		return ErrBadRequest.Wrap(xerrors.Errorf("unable to read rows of sheet %q: %w", req.Sheet, err))
	}

	for i := 1; rows.Next(); i++ {
		row, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return ErrBadRequest.Wrap(xerrors.Errorf("unable to read row of sheet %q: %w", req.Sheet, err))
		}

		if len(row) == 0 {
			continue
		}

		if !req.allRows && int64(i) < req.StartRow {
			continue
		}

		if !req.allRows && int64(i) >= req.StartRow+req.RowCount {
			break
		}

		m := make(map[string]any)
		for j, excelValue := range row {
			name, _ := excelize.ColumnNumberToName(j + 1)
			ytColumns, ok := excelColToYTCols[name]
			if !ok {
				continue
			}
			for _, index := range ytColumns {
				col := s.Columns[index]
				v, err := convert(excelValue, col)
				if err != nil {
					if errors.Is(err, errOptionalField) {
						continue
					}
					return ErrBadRequest.Wrap(xerrors.Errorf("unable to convert %q (column %q) of %q to %s: %w",
						excelValue, name, row, col.Type, err))
				}
				m[col.Name] = v
			}
		}

		if err := out.Write(m); err != nil {
			return xerrors.Errorf("error writing row %+q: %w", m, err)
		}
	}

	return out.Commit()
}

// ReadSchema returns the value of @schema table attribute.
func ReadSchema(ctx context.Context, yc yt.CypressClient, path ypath.Path) (*schema.Schema, error) {
	var s *schema.Schema
	if err := yc.GetNode(ctx, path.Attr("schema"), &s, nil); err != nil {
		return nil, err
	}
	return s, nil
}

// CreateTable creates YT table for given request path with schema inferred from the excel data.
func CreateTable(ctx context.Context, yc yt.CypressClient, req *UploadRequest) error {
	s, err := MakeSchema(req)
	if err != nil {
		return xerrors.Errorf("error inferring schema from excel table: %w", err)
	}

	_, err = yt.CreateTable(ctx, yc, req.Path, yt.WithSchema(*s))
	if yterrors.ContainsErrorCode(err, yterrors.CodeAuthorizationError) {
		return ErrUnauthorized.Wrap(xerrors.Errorf("authorization error when creating table: %w", err))
	}
	return err
}

// MakeSchema creates YT table schema based on excel table.
//
// Column names are determined by one of three methods in the following order:
//  1. From request's column mapping
//  2. From the first row
//  3. Excel column names e.g. A, B, C...
//
// Column types are determined using the following logic:
//  1. Read column types from the first row if types is set to true and header is set to false.
//  2. Read column types from the second row if types is set to true and header is set to true.
//  3. Use Any if none of the above works.
func MakeSchema(req *UploadRequest) (*schema.Schema, error) {
	excelColToYTCols := make(map[string][]string)
	for ytCol, excelCol := range req.Columns {
		excelColToYTCols[excelCol] = append(excelColToYTCols[excelCol], ytCol)
	}

	var columns []*schema.Column
	if len(req.Columns) != 0 {
		ytColumnNames := make([]string, 0, len(req.Columns))
		excelColumnNumbers := make(map[string]int)
		for ytCol, excelCol := range req.Columns {
			ytColumnNames = append(ytColumnNames, ytCol)

			n, err := excelize.ColumnNameToNumber(excelCol)
			if err != nil {
				return nil, xerrors.Errorf("invalid column name %q: %w", excelCol, err)
			}
			excelColumnNumbers[ytCol] = n
		}

		sort.Slice(ytColumnNames, func(i, j int) bool {
			return excelColumnNumbers[ytColumnNames[i]] < excelColumnNumbers[ytColumnNames[j]]
		})

		for _, name := range ytColumnNames {
			col := &schema.Column{
				Name: name,
				Type: schema.TypeAny,
			}
			columns = append(columns, col)
		}
	} else {
		row, err := req.readFirstRow()
		if err != nil {
			return nil, err
		}

		for i, name := range row {
			excelCol, err := excelize.ColumnNumberToName(i + 1)
			if err != nil {
				return nil, xerrors.Errorf("unable to convert number %d to excel column: %w", i+1, err)
			}

			if name == "" {
				continue
			}

			if !req.Header {
				name = excelCol
			}

			col := &schema.Column{
				Name: name,
				Type: schema.TypeAny,
			}
			columns = append(columns, col)

			excelColToYTCols[excelCol] = append(excelColToYTCols[excelCol], name)
		}
	}

	colByName := make(map[string]*schema.Column)
	for _, col := range columns {
		colByName[col.Name] = col
	}

	var typeRow []string
	if req.Types {
		var err error
		if req.Header {
			typeRow, err = req.readSecondRow()
		} else {
			typeRow, err = req.readFirstRow()
		}
		if err != nil {
			return nil, err
		}
	}

	if len(typeRow) > 0 {
		for i, typeStr := range typeRow {
			excelCol, err := excelize.ColumnNumberToName(i + 1)
			if err != nil {
				return nil, xerrors.Errorf("unable to convert number %d to excel column: %w", i+1, err)
			}

			t, err := GetColumnType(typeStr)
			if err != nil {
				return nil, xerrors.Errorf("unable to read column type from %q", typeStr)
			}

			for _, name := range excelColToYTCols[excelCol] {
				colByName[name].Type = t
			}
		}
	}

	s := schema.Schema{}
	for _, col := range columns {
		s.Columns = append(s.Columns, *col)
	}

	return &s, nil
}

func GetColumnType(typeStr string) (schema.Type, error) {
	var t schema.Type
	normalized := strings.TrimSpace(typeStr)
	return t, t.UnmarshalText([]byte(normalized))
}

// errOptionalField is an error returned by convert function
// when converting empty cell values to optional columns.
var errOptionalField = xerrors.NewSentinel("optional field")

func convert(value string, c schema.Column) (any, error) {
	if value == "" && !c.Required {
		return "", errOptionalField
	}

	switch c.Type {
	case schema.TypeInt64:
		return strconv.ParseInt(value, 10, 64)
	case schema.TypeInt32:
		return strconv.ParseInt(value, 10, 32)
	case schema.TypeInt16:
		return strconv.ParseInt(value, 10, 16)
	case schema.TypeInt8:
		return strconv.ParseInt(value, 10, 8)
	case schema.TypeUint64:
		return strconv.ParseUint(value, 10, 64)
	case schema.TypeUint32:
		return strconv.ParseUint(value, 10, 32)
	case schema.TypeUint16:
		return strconv.ParseUint(value, 10, 16)
	case schema.TypeUint8:
		return strconv.ParseUint(value, 10, 8)
	case schema.TypeFloat32:
		return strconv.ParseFloat(value, 32)
	case schema.TypeFloat64:
		return strconv.ParseFloat(value, 64)
	case schema.TypeBoolean:
		return strconv.ParseBool(value)
	case schema.TypeBytes:
		return []byte(value), nil
	case schema.TypeString:
		return value, nil
	case schema.TypeAny:
		var i any
		if err := yson.Unmarshal([]byte(value), &i); err != nil {
			return []byte(value), nil
		}
		return i, nil
	case schema.TypeDate:
		return convertDate(value)
	case schema.TypeDatetime:
		return convertDatetime(value)
	case schema.TypeTimestamp:
		return convertTimestamp(value)
	case schema.TypeInterval:
		return strconv.ParseInt(value, 10, 64)
	default:
		return nil, xerrors.Errorf("unexpected type %s", c.Type)
	}
}

// convertDate converts Excel date to YT date.
//
// Excel date is a number of days since January 1, 1900.
// YT date is a number of days since January 1, 1970.
//
// Excel does not recognize dates before January 1, 1900.
// YT does not support dates before January 1, 1970.
func convertDate(value string) (schema.Date, error) {
	v, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, xerrors.Errorf("unable to convert %q to uint64: %w", value, err)
	}

	ytDate := schema.Date(v - uint64(unixEpoch.Add(day).Sub(excelEpoch).Hours()/24))
	return ytDate, nil
}

// convertDatetime converts Excel datetime to YT date.
//
// Excel datetime is a number of days since January 1, 1900.
// YT datetime is a number of seconds since January 1, 1970.
//
// Excel does not recognize dates before January 1, 1900.
// YT does not support dates before January 1, 1970.
func convertDatetime(value string) (schema.Datetime, error) {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, xerrors.Errorf("unable to convert %q to float64: %w", value, err)
	}

	if v < 0 {
		return 0, xerrors.Errorf("datetime value must be positive; got %v", v)
	}

	ytDatetime := schema.Datetime(uint64(v*86400) - uint64(unixEpoch.Add(day).Sub(excelEpoch).Seconds()))
	return ytDatetime, nil
}

// convertTimestamp converts Excel timestamp to YT date.
//
// Excel timestamp is a number of days since January 1, 1900.
// YT timestamp is a number of microseconds since January 1, 1970.
//
// Excel does not recognize dates before January 1, 1900.
// YT does not support dates before January 1, 1970.
func convertTimestamp(value string) (schema.Timestamp, error) {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, xerrors.Errorf("unable to convert %q to float64: %w", value, err)
	}

	if v < 0 {
		return 0, xerrors.Errorf("datetime value must be positive; got %v", v)
	}

	ytTimestamp := schema.Timestamp(uint64(v*86400*1e6) - uint64(unixEpoch.Add(day).Sub(excelEpoch).Microseconds()))
	return ytTimestamp, nil
}
