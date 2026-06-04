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

// YTTraceFn produces the trace parent expected by the YT client. Assign it to
// yt.Config.TraceFn so that outgoing requests to the YT proxy share a trace
// with the excel request that triggered them.
//
// It prefers a real W3C traceparent propagated into ctx. When none is present,
// it falls back to the internal request id — the same value used as the event
// log trace_id — so the event log and the YT proxy logs can be correlated even
// for requests that arrive without a traceparent (e.g. from the UI).
//
// It returns ok=false only when there is no trace at all, which leaves the YT
// client's tracing untouched (no traceparent header is added).
func YTTraceFn(ctx context.Context) (traceID guid.GUID, spanID uint64, flags byte, ok bool) {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		tid := sc.TraceID()
		high := binary.BigEndian.Uint64(tid[:8])
		low := binary.BigEndian.Uint64(tid[8:])
		traceID = guid.FromHalves(low, high)

		sid := sc.SpanID()
		spanID = binary.BigEndian.Uint64(sid[:])

		flags = byte(sc.TraceFlags())

		return traceID, spanID, flags, true
	}

	// Fallback: no upstream traceparent. Reuse the request id stored for the
	// event log (see WithTraceContext) as the trace id sent to YT, so all YT
	// calls made for this excel request carry the same trace_id that appears in
	// the event log.
	tc := traceContextFromContext(ctx)
	if tc.fallbackTraceID == "" {
		return
	}
	g, err := guid.ParseString(tc.fallbackTraceID)
	if err != nil {
		return
	}
	traceID = g
	// Derive a stable, non-zero parent span id from the trace id.
	a, b := g.Halves()
	switch {
	case a != 0:
		spanID = a
	case b != 0:
		spanID = b
	default:
		spanID = 1
	}
	flags = 1 // sampled

	return traceID, spanID, flags, true
}
