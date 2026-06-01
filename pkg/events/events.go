package events

// EventType identifies the kind of event.
type EventType string

const (
	EventUploadStarted   EventType = "upload_started"
	EventUploadSucceeded EventType = "upload_succeeded"
	EventUploadFailed    EventType = "upload_failed"

	EventExportTableStarted   EventType = "export_table_started"
	EventExportTableSucceeded EventType = "export_table_succeeded"
	EventExportTableFailed    EventType = "export_table_failed"

	EventExportQueryResultStarted   EventType = "export_query_result_started"
	EventExportQueryResultSucceeded EventType = "export_query_result_succeeded"
	EventExportQueryResultFailed    EventType = "export_query_result_failed"
)

// Event is a single record written to the event log.
type Event struct {
	TraceID      string    `json:"trace_id"`
	SpanID       string    `json:"span_id"`
	ParentSpanID string    `json:"parent_span_id"`
	EventType    EventType `json:"event_type"`
	EventInfo    any       `json:"event_info"`
}

// UploadInfo is event_info for upload_* events.
type UploadInfo struct {
	Cluster  string `json:"cluster"`
	YTPath   string `json:"yt_path"`
	Filename string `json:"filename,omitempty"`
	Sheet    string `json:"sheet,omitempty"`
	Append   bool   `json:"append"`
	Create   bool   `json:"create"`
	Error    string `json:"error,omitempty"`
}

// ExportTableInfo is event_info for export_table_* events.
type ExportTableInfo struct {
	Cluster  string   `json:"cluster"`
	YTPath   string   `json:"yt_path"`
	Filename string   `json:"filename,omitempty"`
	Columns  []string `json:"columns,omitempty"`
	StartRow int64    `json:"start_row"`
	RowCount int64    `json:"row_count"`
	Error    string   `json:"error,omitempty"`
}

// ExportQueryResultInfo is event_info for export_query_result_* events.
type ExportQueryResultInfo struct {
	Cluster     string   `json:"cluster"`
	QueryID     string   `json:"query_id"`
	ResultIndex int64    `json:"result_index"`
	Filename    string   `json:"filename,omitempty"`
	Columns     []string `json:"columns,omitempty"`
	Error       string   `json:"error,omitempty"`
}
