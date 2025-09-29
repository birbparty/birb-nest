package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Init initializes all telemetry components
func Init(cfg *Config) error {
	// Initialize logger
	if err := InitLogger(cfg); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize metrics
	if err := InitMetrics(cfg); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	// Initialize tracing
	if err := InitTracing(cfg); err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	L().WithFields(map[string]interface{}{
		"service":      cfg.ServiceName,
		"version":      cfg.ServiceVersion,
		"environment":  cfg.Environment,
		"exportToFile": cfg.ExportToFile,
	}).Info("Telemetry initialized")

	return nil
}

// Shutdown gracefully shuts down all telemetry components
func Shutdown(ctx context.Context) error {
	// Close tracing
	if err := CloseTracing(ctx); err != nil {
		L().WithError(err).Error("Failed to close tracing")
	}

	// Close logger
	if err := CloseLogger(); err != nil {
		L().WithError(err).Error("Failed to close logger")
	}

	return nil
}

// PrometheusHandler returns an HTTP handler for Prometheus metrics
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// FiberMetricsMiddleware returns a Fiber middleware for recording HTTP metrics
func FiberMetricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Start a span for the request
		ctx, span := StartSpan(c.UserContext(), fmt.Sprintf("%s %s", c.Method(), c.Path()))
		defer span.End()

		// Set request context with trace
		c.SetUserContext(ctx)

		// Process request
		err := c.Next()

		// Record metrics
		duration := time.Since(start)
		status := fmt.Sprintf("%d", c.Response().StatusCode())

		RecordHTTPRequest(c.Method(), c.Path(), status, duration)

		// Set span attributes
		span.SetAttributes(
			semconv.HTTPMethodKey.String(c.Method()),
			semconv.HTTPTargetKey.String(c.Path()),
			semconv.HTTPStatusCodeKey.Int(c.Response().StatusCode()),
		)

		if err != nil {
			RecordError(ctx, err)
			SetErrorStatus(ctx, err.Error())
		} else if c.Response().StatusCode() >= 400 {
			SetErrorStatus(ctx, fmt.Sprintf("HTTP %d", c.Response().StatusCode()))
		} else {
			SetOKStatus(ctx)
		}

		return err
	}
}

// FiberLoggingMiddleware returns a Fiber middleware for structured logging
func FiberLoggingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Log request
		entry := WithContext(c.UserContext()).WithFields(map[string]interface{}{
			"method":     c.Method(),
			"path":       c.Path(),
			"status":     c.Response().StatusCode(),
			"duration":   time.Since(start).Milliseconds(),
			"ip":         c.IP(),
			"user_agent": c.Get("User-Agent"),
		})

		if err != nil {
			entry.WithError(err).Error("Request failed")
		} else if c.Response().StatusCode() >= 400 {
			entry.Warn("Request completed with error status")
		} else {
			entry.Info("Request completed")
		}

		return err
	}
}

// TimeOperation is a helper to time and record metrics for an operation
func TimeOperation(ctx context.Context, operation string) func(status string) {
	start := time.Now()
	_, span := StartSpan(ctx, operation)

	return func(status string) {
		duration := time.Since(start)
		RecordCacheOperation(operation, status, duration)

		if status == "error" {
			SetErrorStatus(ctx, "Operation failed")
		} else {
			SetOKStatus(ctx)
		}

		span.End()

		WithContext(ctx).WithFields(map[string]interface{}{
			"operation": operation,
			"status":    status,
			"duration":  duration.Milliseconds(),
		}).Debug("Operation completed")
	}
}
