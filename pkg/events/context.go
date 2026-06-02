package events

import "context"

type ctxKey struct{}

type traceCtx struct {
	fallbackTraceID string
}

// WithTraceContext stores a fallback trace ID in ctx.
// Used when no OTel traceparent is propagated — typically set from the internal request GUID.
func WithTraceContext(ctx context.Context, fallbackTraceID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, traceCtx{
		fallbackTraceID: fallbackTraceID,
	})
}

func traceContextFromContext(ctx context.Context) traceCtx {
	tc, _ := ctx.Value(ctxKey{}).(traceCtx)
	return tc
}
