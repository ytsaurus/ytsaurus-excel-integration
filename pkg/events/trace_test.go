package events

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

const (
	sampleTraceID = "0af7651916cd43dd8448eb211c80319c"
	sampleSpanID  = "b7ad6b7169203331"
)

func TestExtractTraceContext_PropagatesIncomingTraceparent(t *testing.T) {
	h := http.Header{}
	h.Set("traceparent", "00-"+sampleTraceID+"-"+sampleSpanID+"-01")

	ctx := ExtractTraceContext(context.Background(), h)

	// This is what FileWriter.Write reads for the event trace_id.
	sc := trace.SpanFromContext(ctx).SpanContext()
	require.True(t, sc.IsValid())
	require.Equal(t, sampleTraceID, sc.TraceID().String())
	require.Equal(t, sampleSpanID, sc.SpanID().String())
	require.True(t, sc.TraceFlags().IsSampled())
}

func TestYTTraceFn_RoundTripsIncomingTrace(t *testing.T) {
	h := http.Header{}
	h.Set("traceparent", "00-"+sampleTraceID+"-"+sampleSpanID+"-01")
	ctx := ExtractTraceContext(context.Background(), h)

	traceID, spanID, flags, ok := YTTraceFn(ctx)
	require.True(t, ok)
	// HexString feeds the traceparent the YT client sends to the proxy; it must
	// match the trace id we received so the trace stays continuous.
	require.Equal(t, sampleTraceID, traceID.HexString())
	require.Equal(t, uint64(0xb7ad6b7169203331), spanID)
	require.Equal(t, byte(1), flags)
}

func TestYTTraceFn_NoTraceIsNoOp(t *testing.T) {
	_, _, _, ok := YTTraceFn(context.Background())
	require.False(t, ok)

	// An empty header set must not produce a trace either (fallback behaviour).
	ctx := ExtractTraceContext(context.Background(), http.Header{})
	_, _, _, ok = YTTraceFn(ctx)
	require.False(t, ok)
}
