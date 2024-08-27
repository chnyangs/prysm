// Package tracing includes useful functions for opentelemetry annotations.
package tracing

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// AnnotateError on span. This should be used any time a particular span experiences an error.
func AnnotateError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.SetAttributes(attribute.Bool("error", true))
	span.SetStatus(codes.Error, err.Error())
}
