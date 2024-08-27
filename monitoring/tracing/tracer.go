// Package tracing sets up jaeger as an opentracing tool
// for services in Prysm.
package tracing

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var log = logrus.WithField("prefix", "tracing")

// Setup creates and initializes a new tracing configuration using OpenTelemetry.
func Setup(serviceName, processName, endpoint string, sampleFraction float64, enable bool) error {
	if !enable {
		// If tracing is disabled, return immediately
		return nil
	}

	if serviceName == "" {
		return errors.New("tracing service name cannot be empty")
	}

	log.Infof("Starting Jaeger exporter endpoint at address = %s", endpoint)
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithHeaders(map[string]string{
				"content-type": "application/json",
			}),
			otlptracehttp.WithInsecure(), //Remove this for public env
		),
	)
	if err != nil {
		return err
	}

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.TraceIDRatioBased(sampleFraction)),
		trace.WithBatcher(
			exporter,
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
			trace.WithBatchTimeout(trace.DefaultScheduleDelay*time.Millisecond),
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
		),
		trace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
				attribute.String("process_name", processName),
			),
		),
	)

	otel.SetTracerProvider(tp)
	log.Printf("Tracing enabled with endpoint: %s", endpoint)
	return nil
}
