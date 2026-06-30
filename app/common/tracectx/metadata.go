package tracectx

// Metadata is a framework-neutral key-value carrier for HTTP headers or RPC metadata.
type Metadata map[string]string

func (t Trace) ToMetadata() Metadata {
	metadata := Metadata{}
	if t.RequestID != "" {
		metadata[HeaderRequestID] = t.RequestID
	}
	if t.TraceID != "" {
		metadata[HeaderTraceID] = t.TraceID
	}
	if t.UserID != "" {
		metadata[HeaderUserID] = t.UserID
	}
	if t.MerchantID != "" {
		metadata[HeaderMerchantID] = t.MerchantID
	}
	return metadata
}

func FromMetadata(metadata Metadata) Trace {
	if metadata == nil {
		return Trace{}
	}
	return Trace{
		RequestID:  metadata[HeaderRequestID],
		TraceID:    metadata[HeaderTraceID],
		UserID:     metadata[HeaderUserID],
		MerchantID: metadata[HeaderMerchantID],
	}
}
