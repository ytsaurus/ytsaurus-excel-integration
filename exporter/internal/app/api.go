package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/atomic"

	"go.ytsaurus.tech/library/go/core/log"
	"go.ytsaurus.tech/library/go/core/metrics"
	"go.ytsaurus.tech/library/go/core/xerrors"
	"go.ytsaurus.tech/yt/go/guid"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/microservices/excel/exporter/internal/exporter"
)

// API provides http endpoints to interact with the service.
type API struct {
	conf *ClusterConfig
	yc   yt.Client

	l log.Structured

	ready atomic.Bool
}

// NewAPI creates new API.
func NewAPI(c *ClusterConfig, yc yt.Client, l log.Structured) *API {
	return &API{conf: c, yc: yc, l: l}
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/ready", func(r chi.Router) {
		r.Use(waitReady(&a.ready))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			// Explicitly set status code, so that log middleware could properly log it.
			w.WriteHeader(http.StatusOK)
		})
	})

	r.Route("/export", func(r chi.Router) {
		r.Use(waitReady(&a.ready))
		r.Get("/", a.exportTable)
	})

	r.Route("/export-query-result", func(r chi.Router) {
		r.Use(waitReady(&a.ready))
		r.Get("/", a.exportQueryResult)
	})

	return r
}

func (a *API) RegisterMetrics(r metrics.Registry) {}

func (a *API) SetReady() {
	a.ready.Store(true)
	a.l.Info("api is ready to serve!")
}

// exportTable exports data from static yt table to excel.
func (a *API) exportTable(w http.ResponseWriter, r *http.Request) {
	paths, ok := r.URL.Query()["path"]
	if !ok || len(paths) != 1 {
		err := xerrors.Errorf("single path is required, got %d", len(paths))
		replyError(w, r, err, http.StatusBadRequest)
		return
	}

	numberPrecisionMode := exporter.NumberPrecisionMode(r.URL.Query().Get("number_precision_mode"))

	req, err := exporter.MakeExportRequest(paths[0], numberPrecisionMode)
	if err != nil {
		err = xerrors.Errorf("error parsing request: %w", err)
		replyError(w, r, err, http.StatusBadRequest)
		return
	}

	a.l.Info("parsed url params", log.Any("export_request", req))

	if err := a.validateExportRequest(r.Context(), req); err != nil {
		replyError(w, r, err, http.StatusBadRequest)
		return
	}

	opts := &exporter.ExportOptions{MaxExcelFileSize: a.conf.maxExcelFileSize}
	rsp, err := exporter.Export(r.Context(), a.yc, req, opts)
	if err != nil {
		if errors.Is(err, exporter.ErrBadRequest) {
			replyError(w, r, err, http.StatusBadRequest)
			return
		}
		replyError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.ms-excel")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", rsp.Filename))
	_ = rsp.File.Write(w)
}

func validateNumberPrecisionMode(mode *exporter.NumberPrecisionMode) error {
	if mode == nil {
		return xerrors.Errorf("missing number precision mode")
	}
	if *mode == "" {
		*mode = exporter.NumberPrecisionModeString
	}

	if !slices.Contains([]exporter.NumberPrecisionMode{
		exporter.NumberPrecisionModeError,
		exporter.NumberPrecisionModeString,
		exporter.NumberPrecisionModeLose,
	}, *mode) {
		return xerrors.Errorf("unexpected handle long numbers: %q; expected one of %q, %q, %q",
			*mode, exporter.NumberPrecisionModeError, exporter.NumberPrecisionModeString,
			exporter.NumberPrecisionModeLose)
	}

	return nil
}

func (a *API) validateExportRequest(ctx context.Context, req *exporter.ExportRequest) error {
	if req.StartRow < 0 {
		return xerrors.Errorf("start row cannot be negative; got %d", req.StartRow)
	}

	if req.RowCount > exporter.MaxRowCount {
		return xerrors.Errorf("too many rows to export; max is %d", exporter.MaxRowCount)
	}

	if req.RowCount == 0 {
		tableRowCount, err := a.readTableRowCount(ctx, req.Path)
		if err != nil {
			return err
		}

		if tableRowCount > exporter.MaxRowCount {
			return xerrors.Errorf("too many rows to export; max is %d", exporter.MaxRowCount)
		}

		req.RowCount = exporter.MaxRowCount
	}

	return validateNumberPrecisionMode(&req.NumberPrecisionMode)
}

func (a *API) readTableRowCount(ctx context.Context, path ypath.Path) (int64, error) {
	var tableRowCount int64
	if err := a.yc.GetNode(ctx, path.Attr("row_count"), &tableRowCount, nil); err != nil {
		return 0, xerrors.Errorf("error reading table row count: %w", err)
	}
	return tableRowCount, nil
}

func (a *API) validateQueryResultExportRequest(ctx context.Context, req *exporter.ExportQueryResultRequest) error {
	if req.LowerRowIndex != nil && *req.LowerRowIndex < 0 {
		return xerrors.Errorf("start row cannot be negative; got %d", req.LowerRowIndex)
	}

	return validateNumberPrecisionMode(&req.NumberPrecisionMode)
}

func makeQueryResultExportRequestFromQuery(r *http.Request) (*exporter.ExportQueryResultRequest, error) {
	var exportRequest exporter.ExportQueryResultRequest
	id := r.URL.Query().Get("query_id")
	if id == "" {
		return nil, xerrors.Errorf("query id is required")
	}
	guid, err := guid.ParseString(id)
	if err != nil {
		return nil, xerrors.Errorf("error parsing id: %w", err)
	}
	exportRequest.ID = yt.QueryID(guid)

	resultIndex := r.URL.Query().Get("result_index")
	if resultIndex == "" {
		return nil, xerrors.Errorf("result index is required")
	}
	exportRequest.Index, err = strconv.ParseInt(resultIndex, 10, 64)
	if err != nil {
		return nil, xerrors.Errorf("error parsing result index: %w", err)
	}

	lowerRowIndexQuery := r.URL.Query().Get("lower_row_index")
	if lowerRowIndexQuery != "" {
		lowerRowIndex, err := strconv.ParseInt(lowerRowIndexQuery, 10, 64)
		if err != nil {
			return nil, xerrors.Errorf("error parsing lower row index: %w", err)
		}
		exportRequest.LowerRowIndex = &lowerRowIndex
	}

	upperRowIndexQuery := r.URL.Query().Get("upper_row_index")
	if upperRowIndexQuery != "" {
		upperRowIndex, err := strconv.ParseInt(upperRowIndexQuery, 10, 64)
		if err != nil {
			return nil, xerrors.Errorf("error parsing upper row index: %w", err)
		}
		exportRequest.UpperRowIndex = &upperRowIndex
	}

	exportRequest.Columns = r.URL.Query()["columns"]

	exportRequest.Filename = r.URL.Query().Get("filename")

	exportRequest.NumberPrecisionMode = exporter.NumberPrecisionMode(r.URL.Query().Get("number_precision_mode"))

	return &exportRequest, nil
}

// exportQueryResult exports data from query tracker result to excel.
func (a *API) exportQueryResult(w http.ResponseWriter, r *http.Request) {
	req, err := makeQueryResultExportRequestFromQuery(r)
	if err != nil {
		err = xerrors.Errorf("error parsing request: %w", err)
		replyError(w, r, err, http.StatusBadRequest)
		return
	}

	if err = a.validateQueryResultExportRequest(r.Context(), req); err != nil {
		replyError(w, r, err, http.StatusBadRequest)
		return
	}

	opts := &exporter.ExportOptions{MaxExcelFileSize: a.conf.maxExcelFileSize}
	rsp, err := exporter.ExportQueryResult(r.Context(), a.yc, req, opts)
	if err != nil {
		if errors.Is(err, exporter.ErrBadRequest) {
			replyError(w, r, err, http.StatusBadRequest)
			return
		}
		replyError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.ms-excel")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", rsp.Filename))
	_ = rsp.File.Write(w)
}
