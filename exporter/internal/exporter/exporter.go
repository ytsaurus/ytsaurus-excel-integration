package exporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
	"go.ytsaurus.tech/library/go/core/xerrors"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
)

const (
	excelMaxRowCount = 1048576
	excelMaxColCount = 16384
	// MaxRowCount stores the maximum number of static table rows that service handles.
	// -2 is for headers and types.
	MaxRowCount = excelMaxRowCount - 2
	// excelMaxFilepathLength stores the maximum length of excel filepath that excel can open.
	excelMaxFilepathLength = 218
	// maxFilenameLength stores max length of generated filename.
	maxFilenameLength = excelMaxFilepathLength - 60
)

type NumberPrecisionMode string

const (
	NumberPrecisionModeError  NumberPrecisionMode = "error"
	NumberPrecisionModeString NumberPrecisionMode = "string"
	NumberPrecisionModeLose   NumberPrecisionMode = "lose"
)

// ExportRequest represents a request to export static yt table to excel.
type ExportRequest struct {
	Filename            string     `json:"filename"`
	Path                ypath.Path `json:"path"`
	allColumns          bool
	Columns             []string `json:"columns"`
	allRows             bool
	StartRow            int64 `json:"start_row"`
	RowCount            int64 `json:"row_count"`
	NumberPrecisionMode NumberPrecisionMode
}

func (r *ExportRequest) String() string {
	s := r.Path.String()
	if !r.allColumns && r.Columns != nil {
		s += fmt.Sprintf("{%s}", strings.Join(r.Columns, ","))
	}
	if !r.allRows {
		s += fmt.Sprintf("[#%d:#%d]", r.StartRow, r.StartRow+r.RowCount)
	}
	return s
}

// MakeExportRequest creates request object from ypath string and mode of handling numbers with high precision.
//
// Example inputs:
//
//	//home/example{"col1","col2"}[#10:#999]
//	//home/example{"col1","col2"}
//	<file_name=data.xlsx>//home/example
//	//home/example
//
// numberPrecisionMode="error"/"string"/"lose".
func MakeExportRequest(s string, numberPrecisionMode NumberPrecisionMode) (*ExportRequest, error) {
	p, err := ypath.Parse(s)
	if err != nil {
		return nil, err
	}

	r := &ExportRequest{
		Filename:            p.FileName,
		Path:                p.Path,
		allColumns:          len(p.Columns) == 0,
		Columns:             p.Columns,
		allRows:             true,
		NumberPrecisionMode: numberPrecisionMode,
	}

	if len(p.Ranges) > 1 {
		return nil, xerrors.Errorf("multiple ranges are not supported")
	}

	if len(p.Ranges) == 1 {
		r.allRows = false
		r.StartRow = *p.Ranges[0].Lower.RowIndex
		r.RowCount = *p.Ranges[0].Upper.RowIndex - r.StartRow
	}

	return r, nil
}

// MakePath creates ypath for the read request.
//
// Example: //home/example{col1,col2}[#10:#999].
func (r *ExportRequest) MakePath() *ypath.Rich {
	endRow := r.StartRow + r.RowCount
	return ypath.NewRich(string(r.Path)).
		AddRange(ypath.Range{
			Lower: &ypath.ReadLimit{RowIndex: &r.StartRow},
			Upper: &ypath.ReadLimit{RowIndex: &endRow},
		}).
		SetColumns(r.Columns)
}

func (r *ExportRequest) EnsureFileName(ctx context.Context, yc yt.Client) {
	defer func() {
		if !strings.HasSuffix(r.Filename, ".xlsx") {
			r.Filename += ".xlsx"
		}
	}()

	if r.Filename != "" {
		return
	}

	filename, err := ReadFileName(ctx, yc, r.Path)
	if err == nil && filename != "" {
		r.Filename = filename
		return
	}

	r.Filename = r.MakeFileName(randomName())
}

func (r *ExportRequest) MakeFileName(suffix string) string {
	name := "yt"
	name += replaceNonAlphanumeric(string(r.Path))

	if r.Columns != nil && !r.allColumns {
		colStr := strings.Join(r.Columns, "_")
		name += "__" + replaceNonAlphanumeric(colStr)
	}

	if !r.allRows {
		name += fmt.Sprintf("__%d_%d__", r.StartRow, r.StartRow+r.RowCount)
	}

	name += suffix

	if len(name) > maxFilenameLength {
		name = name[:maxFilenameLength]
	}

	name += ".xlsx"
	return name
}

type ExportOptions struct {
	MaxExcelFileSize int
}

type ExportResponse struct {
	// Filename is name of a converted file.
	Filename string
	File     *excelize.File
}

// ErrBadRequest is an error that signals that conversion is failed due to bad request.
var ErrBadRequest = xerrors.NewSentinel("bad request")

// Export executes given conversion request.
func Export(ctx context.Context, yc yt.Client, req *ExportRequest, opts *ExportOptions) (*ExportResponse, error) {
	s, err := ReadSchema(ctx, yc, req.Path)
	if err != nil {
		if yterrors.ContainsResolveError(err) {
			return nil, ErrBadRequest.Wrap(xerrors.Errorf("error reading schema for %q: %w", req.Path, err))
		}
		return nil, xerrors.Errorf("error reading schema for %q: %w", req.Path, err)
	}

	req.EnsureFileName(ctx, yc)

	if len(req.Columns) > excelMaxColCount || len(req.Columns) == 0 && len(s.Columns) > excelMaxColCount {
		return nil, ErrBadRequest.Wrap(xerrors.Errorf("exceeding max number of excel columns %d", excelMaxColCount))
	}

	in, err := yc.ReadTable(ctx, req.MakePath(), nil)
	if err != nil {
		return nil, xerrors.Errorf("error creating reader: %w", err)
	}
	defer func() { _ = in.Close() }()

	if len(req.Columns) == 0 {
		req.Columns = getColumnNames(s.Columns)
	}

	convertOpts := &ConvertOptions{
		Columns:             req.Columns,
		Schema:              s,
		ExportOptions:       opts,
		NumberPrecisionMode: req.NumberPrecisionMode,
	}
	out, err := Convert(in, convertOpts)
	if err != nil {
		return nil, xerrors.Errorf("error converting %s: %w", req, err)
	}

	return &ExportResponse{Filename: req.Filename, File: out}, nil
}

// ReadSchema returns the value of @schema table attribute.
func ReadSchema(ctx context.Context, yc yt.Client, path ypath.Path) (*schema.Schema, error) {
	var s *schema.Schema
	if err := yc.GetNode(ctx, path.Attr("schema"), &s, nil); err != nil {
		return nil, err
	}
	return s, nil
}

// ReadFileName returns the value of @file_name table attribute.
func ReadFileName(ctx context.Context, yc yt.Client, path ypath.Path) (string, error) {
	var filename string
	if err := yc.GetNode(ctx, path.Attr("file_name"), &filename, nil); err != nil {
		return "", err
	}
	return filename, nil
}

func getColumnNames(columns []schema.Column) []string {
	names := make([]string, len(columns))
	for i, col := range columns {
		names[i] = col.Name
	}
	return names
}

// ExportQueryResultRequest represents a request to export query tracker result to excel.
type ExportQueryResultRequest struct {
	Filename            string
	ID                  yt.QueryID
	Index               int64
	LowerRowIndex       *int64
	UpperRowIndex       *int64
	Columns             []string
	NumberPrecisionMode NumberPrecisionMode
}

func (r *ExportQueryResultRequest) EnsureFileName() {
	defer func() {
		if !strings.HasSuffix(r.Filename, ".xlsx") {
			r.Filename += ".xlsx"
		}
	}()

	if r.Filename != "" {
		return
	}

	r.Filename = r.MakeFileName()
}

func (r *ExportQueryResultRequest) MakeFileName() string {
	return fmt.Sprintf("yt_query_result__%s__%d.xlsx", replaceNonAlphanumeric(string(r.ID.String())), r.Index)
}

// Export executes given conversion request.
func ExportQueryResult(
	ctx context.Context,
	yc yt.Client,
	req *ExportQueryResultRequest,
	opts *ExportOptions,
) (*ExportResponse, error) {
	qr, err := yc.GetQueryResult(ctx, req.ID, req.Index, nil)
	if err != nil {
		return nil, ErrBadRequest.Wrap(xerrors.Errorf("error getting query result by id %q: %w", req.ID, err))
	}

	s := &qr.Schema

	req.EnsureFileName()

	if len(req.Columns) > excelMaxColCount || len(req.Columns) == 0 && len(s.Columns) > excelMaxColCount {
		return nil, ErrBadRequest.Wrap(xerrors.Errorf("exceeding max number of excel columns %d", excelMaxColCount))
	}

	in, err := yc.ReadQueryResult(ctx, req.ID, req.Index, &yt.ReadQueryResultOptions{
		Columns:       req.Columns,
		LowerRowIndex: req.LowerRowIndex,
		UpperRowIndex: req.UpperRowIndex,
	})
	if err != nil {
		return nil, ErrBadRequest.Wrap(err)
	}
	defer func() { _ = in.Close() }()

	if len(req.Columns) == 0 {
		req.Columns = getColumnNames(s.Columns)
	}

	convertOpts := &ConvertOptions{
		Columns:             req.Columns,
		Schema:              s,
		ExportOptions:       opts,
		NumberPrecisionMode: req.NumberPrecisionMode,
	}
	out, err := Convert(in, convertOpts)
	if err != nil {
		return nil, xerrors.Errorf("error converting %q: %w", req.ID, err)
	}

	return &ExportResponse{Filename: req.Filename, File: out}, nil
}
