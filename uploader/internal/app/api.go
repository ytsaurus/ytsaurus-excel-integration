package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"
	"go.uber.org/atomic"

	"go.ytsaurus.tech/library/go/core/log"
	"go.ytsaurus.tech/library/go/core/metrics"
	"go.ytsaurus.tech/library/go/core/xerrors"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/microservices/excel/uploader/internal/uploader"
)

const (
	uploadFormName = "uploadfile"

	// maxMemory is a max number of bytes of the upload file parts that could be stored in memory.
	maxMemory = 32 << 20 // 32 mb
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

	r.Route("/upload", func(r chi.Router) {
		r.Use(waitReady(&a.ready))
		r.Post("/", a.uploadFile)
	})

	return r
}

func (a *API) RegisterMetrics(r metrics.Registry) {}

func (a *API) SetReady() {
	a.ready.Store(true)
	a.l.Info("api is ready to serve!")
}

// uploadFile uploads excel file to static yt table with strict schema.
func (a *API) uploadFile(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	paths, ok := q["path"]
	if !ok || len(paths) != 1 {
		err := xerrors.Errorf("single path is required, got %d", len(paths))
		replyError(w, r, err, http.StatusBadRequest)
		return
	}
	path := paths[0]

	var startRow int64
	if s := q.Get("start_row"); s != "" {
		var err error
		startRow, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			replyError(w, r, err, http.StatusBadRequest)
			return
		}
	}

	var rowCount int64
	if s := q.Get("row_count"); s != "" {
		var err error
		rowCount, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			replyError(w, r, err, http.StatusBadRequest)
			return
		}
	}

	var sheet string
	if sheets, ok := q["sheet"]; ok {
		sheet = sheets[0]
	}

	var header bool
	if headers, ok := q["header"]; ok {
		header = headers[0] == "true"
	}

	var types bool
	if ts, ok := q["types"]; ok {
		types = ts[0] == "true"
	}

	columnMapping := make(map[string]string)
	if columns, ok := q["columns"]; ok {
		if header {
			err := xerrors.Errorf("unable to use header=true together with column mapping")
			replyError(w, r, err, http.StatusBadRequest)
		}

		if err := json.Unmarshal([]byte(columns[0]), &columnMapping); err != nil {
			err := xerrors.Errorf("unable to parse column mapping: %w", err)
			replyError(w, r, err, http.StatusBadRequest)
			return
		}
	}

	appendRows := false
	if v, ok := q["append"]; ok {
		if v[0] == "true" {
			appendRows = true
		}
	}

	create := false
	if v, ok := q["create"]; ok {
		if v[0] == "true" {
			create = true
		}
	}

	req, err := uploader.MakeUploadRequest(path, startRow, rowCount, sheet, header, types, columnMapping, appendRows, create)
	if err != nil {
		err = xerrors.Errorf("error parsing request: %w", err)
		replyError(w, r, err, http.StatusBadRequest)
		return
	}
	a.l.Info("parsed url params", log.Any("upload_request", req))

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		err := xerrors.Errorf("unable to read request: %w", err)
		replyError(w, r, err, http.StatusBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, _, err := r.FormFile(uploadFormName)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = file.Close() }()

	xlsx, err := excelize.OpenReader(file)
	if err != nil {
		err := xerrors.Errorf("unable to read excel file: %w", err)
		replyError(w, r, err, http.StatusBadRequest)
		return
	}
	defer func() { _ = xlsx.Close() }()
	req.Data = xlsx

	if err := uploader.Upload(r.Context(), a.yc, req); err != nil {
		if errors.Is(err, uploader.ErrUnauthorized) {
			replyError(w, r, err, http.StatusUnauthorized)
			return
		}
		if errors.Is(err, uploader.ErrBadRequest) {
			replyError(w, r, err, http.StatusBadRequest)
			return
		}
		replyError(w, r, err, http.StatusInternalServerError)
		return
	}
}
