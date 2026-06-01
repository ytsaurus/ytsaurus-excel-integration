package events

import "context"

type ctxKey struct{}

type traceCtx struct {
	fallbackTraceID string
	parentSpanID    string
}

// WithTraceContext stores fallback trace ID and parent span ID in ctx.
// Called from request middleware with the internal request GUID and the X-Req-Id balancer header.
func WithTraceContext(ctx context.Context, fallbackTraceID, parentSpanID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, traceCtx{
		fallbackTraceID: fallbackTraceID,
		parentSpanID:    parentSpanID,
	})
}

func traceContextFromContext(ctx context.Context) traceCtx {
	tc, _ := ctx.Value(ctxKey{}).(traceCtx)
	return tc
}
