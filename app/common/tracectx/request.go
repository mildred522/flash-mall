package tracectx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const (
	HeaderRequestID  = "x-request-id"
	HeaderTraceID    = "x-trace-id"
	HeaderUserID     = "x-user-id"
	HeaderMerchantID = "x-merchant-id"
)

type traceKey struct{}

// Trace carries lightweight request metadata across HTTP and RPC boundaries.
type Trace struct {
	RequestID  string
	TraceID    string
	UserID     string
	MerchantID string
}

func NewRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-unknown"
	}
	return "req-" + hex.EncodeToString(b[:])
}

func WithTrace(ctx context.Context, trace Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

func FromContext(ctx context.Context) (Trace, bool) {
	if ctx == nil {
		return Trace{}, false
	}
	trace, ok := ctx.Value(traceKey{}).(Trace)
	return trace, ok
}

func RequestIDFrom(ctx context.Context) string {
	trace, ok := FromContext(ctx)
	if !ok || trace.RequestID == "" {
		return ""
	}
	return trace.RequestID
}
