package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracerOnce sync.Once
	tracer     trace.Tracer
	fileTracer *FileTracerExporter
)

// FileTracerExporter exports traces to a file for local-otel integration
type FileTracerExporter struct {
	mu       sync.Mutex
	file     *os.File
	encoder  *json.Encoder
	filePath string
}

// Span represents a trace span for file export
type FileSpan struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Name       string                 `json:"name"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Attributes map[string]interface{} `json:"attributes"`
	Status     string                 `json:"status"`
	Events     []SpanEvent            `json:"events,omitempty"`
}

// SpanEvent represents an event in a span
type SpanEvent struct {
	Name       string                 `json:"name"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// InitTracing initializes the tracer
func InitTracing(cfg *Config) error {
	var err error
	tracerOnce.Do(func() {
		if !cfg.EnableTracing {
			// Set noop tracer if tracing is disabled
			otel.SetTracerProvider(trace.NewNoopTracerProvider())
			tracer = otel.Tracer(cfg.ServiceName)
			return
		}

		ctx := context.Background()

		// Create resource
		res, resErr := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceNameKey.String(cfg.ServiceName),
				semconv.ServiceVersionKey.String(cfg.ServiceVersion),
				semconv.DeploymentEnvironmentKey.String(cfg.Environment),
			),
		)
		if resErr != nil {
			err = fmt.Errorf("failed to create resource: %w", resErr)
			return
		}

		var tp *sdktrace.TracerProvider

		if cfg.ExportToFile && cfg.TracesFilePath != "" {
			// File export mode
			fileTracer, err = NewFileTracerExporter(cfg.TracesFilePath)
			if err != nil {
				L().WithError(err).Error("Failed to create file tracer")
				return
			}

			tp = sdktrace.NewTracerProvider(
				sdktrace.WithBatcher(fileTracer),
				sdktrace.WithResource(res),
				sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SamplingRate)),
			)
		} else {
			// OTLP export mode
			client := otlptracegrpc.NewClient(
				otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
				otlptracegrpc.WithInsecure(),
			)

			exporter, exportErr := otlptrace.New(ctx, client)
			if exportErr != nil {
				err = fmt.Errorf("failed to create trace exporter: %w", exportErr)
				return
			}

			tp = sdktrace.NewTracerProvider(
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(res),
				sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SamplingRate)),
			)
		}

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		tracer = otel.Tracer(cfg.ServiceName)
	})

	return err
}

// NewFileTracerExporter creates a new file tracer exporter
func NewFileTracerExporter(filePath string) (*FileTracerExporter, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &FileTracerExporter{
		file:     file,
		encoder:  json.NewEncoder(file),
		filePath: filePath,
	}, nil
}

// ExportSpans implements the SpanExporter interface
func (f *FileTracerExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, span := range spans {
		fileSpan := FileSpan{
			TraceID:   span.SpanContext().TraceID().String(),
			SpanID:    span.SpanContext().SpanID().String(),
			Name:      span.Name(),
			StartTime: span.StartTime(),
			EndTime:   span.EndTime(),
			Status:    span.Status().Code.String(),
		}

		if span.Parent().IsValid() {
			fileSpan.ParentID = span.Parent().SpanID().String()
		}

		// Convert attributes
		fileSpan.Attributes = make(map[string]interface{})
		for _, attr := range span.Attributes() {
			fileSpan.Attributes[string(attr.Key)] = attr.Value.AsInterface()
		}

		// Convert events
		for _, event := range span.Events() {
			spanEvent := SpanEvent{
				Name:      event.Name,
				Timestamp: event.Time,
			}
			if len(event.Attributes) > 0 {
				spanEvent.Attributes = make(map[string]interface{})
				for _, attr := range event.Attributes {
					spanEvent.Attributes[string(attr.Key)] = attr.Value.AsInterface()
				}
			}
			fileSpan.Events = append(fileSpan.Events, spanEvent)
		}

		if err := f.encoder.Encode(fileSpan); err != nil {
			return err
		}
	}

	return nil
}

// Shutdown implements the SpanExporter interface
func (f *FileTracerExporter) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

// Tracer returns the global tracer instance
func Tracer() trace.Tracer {
	if tracer == nil {
		return otel.Tracer("birb-nest")
	}
	return tracer
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// StartSpanWithAttributes starts a new span with attributes
func StartSpanWithAttributes(ctx context.Context, name string, attrs map[string]interface{}) (context.Context, trace.Span) {
	var kvs []trace.SpanStartOption
	if len(attrs) > 0 {
		var attributes []trace.SpanStartOption
		for _, v := range attrs {
			switch val := v.(type) {
			case string:
				attributes = append(attributes, trace.WithAttributes(semconv.ServiceNameKey.String(val)))
			case int:
				attributes = append(attributes, trace.WithAttributes(semconv.HTTPStatusCodeKey.Int(val)))
			case int64:
				attributes = append(attributes, trace.WithAttributes(semconv.HTTPResponseContentLengthKey.Int64(val)))
			case float64:
				attributes = append(attributes, trace.WithAttributes(semconv.HTTPRequestContentLengthKey.Int64(int64(val))))
			case bool:
				if val {
					attributes = append(attributes, trace.WithAttributes(semconv.HTTPTargetKey.String("true")))
				} else {
					attributes = append(attributes, trace.WithAttributes(semconv.HTTPTargetKey.String("false")))
				}
			}
		}
		kvs = append(kvs, attributes...)
	}
	return Tracer().Start(ctx, name, kvs...)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, attrs...)
}

// SetStatus sets the status of the current span
func SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// SetErrorStatus sets the status of the current span to Error
func SetErrorStatus(ctx context.Context, description string) {
	SetStatus(ctx, codes.Error, description)
}

// SetOKStatus sets the status of the current span to OK
func SetOKStatus(ctx context.Context) {
	SetStatus(ctx, codes.Ok, "")
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, opts...)
}

// CloseTracing shuts down the tracer provider
func CloseTracing(ctx context.Context) error {
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}
	return nil
}
