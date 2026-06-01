package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.opentelemetry.io/otel/trace"
	"go.ytsaurus.tech/library/go/core/log"
	"go.ytsaurus.tech/yt/go/guid"
)

// Writer writes audit events to some sink.
type Writer interface {
	Write(ctx context.Context, eventType EventType, info any)
}

// NoOpWriter discards all events. Used when event logging is not configured.
type NoOpWriter struct{}

func (NoOpWriter) Write(_ context.Context, _ EventType, _ any) {}

// FileWriter writes events as JSON lines to a rotating log file.
type FileWriter struct {
	w io.Writer
	l log.Structured
}

// NewFileWriter creates a FileWriter with time-based log rotation.
// pattern is a strftime pattern for rotated files, e.g. "/var/log/events.%Y%m%d%H%M".
// linkName is the symlink pointing to the current log file.
func NewFileWriter(pattern, linkName string, rotationTime, maxAge time.Duration, l log.Structured) (*FileWriter, error) {
	rl, err := rotatelogs.New(
		pattern,
		rotatelogs.WithLinkName(linkName),
		rotatelogs.WithRotationTime(rotationTime),
		rotatelogs.WithMaxAge(maxAge),
	)
	if err != nil {
		return nil, err
	}
	return &FileWriter{w: rl, l: l}, nil
}

func (fw *FileWriter) Write(ctx context.Context, eventType EventType, info any) {
	tc := traceContextFromContext(ctx)

	traceID := tc.fallbackTraceID
	spanID := guid.New().String()

	// Prefer OTel trace/span IDs when traceparent is propagated.
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		traceID = sc.TraceID().String()
		spanID = sc.SpanID().String()
	}

	e := Event{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: tc.parentSpanID,
		EventType:    eventType,
		EventInfo:    info,
	}

	data, err := json.Marshal(e)
	if err != nil {
		fw.l.Warn("failed to marshal event", log.Error(err))
		return
	}

	if _, err := fmt.Fprintf(fw.w, "%s\n", data); err != nil {
		fw.l.Warn("failed to write event", log.Error(err))
	}
}
