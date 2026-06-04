package events

import (
	"context"
	"encoding/binary"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"go.ytsaurus.tech/yt/go/guid"
)

// propagator extracts/injects W3C trace context (the traceparent header).
var propagator = propagation.TraceContext{}

// ExtractTraceContext parses a W3C traceparent header from h into ctx.
//
// When a valid traceparent is present, the propagated span context is stored in
// ctx, so that FileWriter.Write picks it up as the event trace_id and the YT
// client (via YTTraceFn) forwards the same trace to the proxy.
//
// When no valid traceparent is present, ctx is returned effectively unchanged
// and event logging falls back to the internal request id (see
// WithTraceContext) — i.e. the previous behaviour.
func ExtractTraceContext(ctx context.Context, h http.Header) context.Context {
	return propagator.Extract(ctx, propagation.HeaderCarrier(h))
}

// YTTraceFn adapts the OTel span context propagated into ctx to the trace
// parent expected by the YT client. Assign it to yt.Config.TraceFn so that
// outgoing requests to the YT proxy carry the incoming trace.
//
// It returns ok=false when no valid trace is present, which leaves the YT
// client's tracing untouched (no traceparent header is added).
func YTTraceFn(ctx context.Context) (traceID guid.GUID, spanID uint64, flags byte, ok bool) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	tid := sc.TraceID()
	high := binary.BigEndian.Uint64(tid[:8])
	low := binary.BigEndian.Uint64(tid[8:])
	traceID = guid.FromHalves(low, high)

	sid := sc.SpanID()
	spanID = binary.BigEndian.Uint64(sid[:])

	flags = byte(sc.TraceFlags())

	return traceID, spanID, flags, true
}
