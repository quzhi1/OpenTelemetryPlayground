package util

import "go.opentelemetry.io/otel/trace"

func GetTraceIdFromSpan(span trace.Span) string {
	return span.SpanContext().TraceID().String()
}

func GetSpanIdFromSpan(span trace.Span) string {
	return span.SpanContext().SpanID().String()
}
