package api

import (
	"net/http"
	"net/url"

	"github.com/birbparty/birb-nest/internal/api/middleware"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupInstanceRoutes configures all API routes for instance mode
func SetupInstanceRoutes(app *fiber.App, handlers *Handlers, cfg *Config, registry *instance.Registry) {
	// Apply Prometheus metrics middleware globally
	app.Use(PrometheusMetricsMiddleware(cfg.InstanceID, cfg.Mode))

	// API v1 group
	v1 := app.Group("/v1")

	// Cache endpoints with required instance middleware
	reqMiddleware := middleware.NewInstanceMiddleware(registry, true)
	reqMiddleware.SetDefaultInstanceID(cfg.DefaultInstanceID)
	cache := v1.Group("/cache", reqMiddleware.Handle())

	// Single key operations
	cache.Get("/:key", handlers.Get)
	cache.Post("/:key", handlers.Set)
	cache.Put("/:key", handlers.Set)
	cache.Delete("/:key", handlers.Delete)

	// Batch operations with optional instance middleware
	optMiddleware := middleware.NewInstanceMiddleware(registry, false)
	optMiddleware.SetDefaultInstanceID(cfg.DefaultInstanceID)
	v1.Post("/cache/batch/get", optMiddleware.Handle(), handlers.BatchGet)

	// Health endpoint (no auth required)
	app.Get("/health", handlers.Health)

	// Metrics endpoints
	app.Get("/metrics", handlers.Metrics)

	// Prometheus metrics endpoint
	if cfg.TelemetryEnabled {
		app.Get(cfg.MetricsPath, adaptor.HTTPHandler(promhttp.Handler()))
	}

	// Root endpoint
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service":     "birb-nest-api",
			"version":     "1.0.0",
			"mode":        cfg.Mode,
			"instance_id": cfg.InstanceID,
			"status":      "running",
			"endpoints": fiber.Map{
				"cache": fiber.Map{
					"get":    "GET /v1/cache/:key",
					"create": "POST /v1/cache/:key",
					"update": "PUT /v1/cache/:key",
					"delete": "DELETE /v1/cache/:key",
					"batch":  "POST /v1/cache/batch/get",
				},
				"health":  "GET /health",
				"metrics": "GET /metrics",
			},
		})
	})

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Endpoint not found",
		})
	})
}

// adaptor wraps http.Handler for Fiber
var adaptor = &httpHandlerAdaptor{}

type httpHandlerAdaptor struct{}

func (h *httpHandlerAdaptor) HTTPHandler(handler http.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Fiber v2 provides a built-in adaptor through fasthttp
		c.Context().Response.Header.Set("Content-Type", "text/plain; charset=utf-8")

		// Create a simple wrapper that calls the Prometheus handler
		// We'll write directly to the response
		fasthttpHandler := func(ctx *fiber.Ctx) error {
			// Prometheus handler writes directly to response writer
			promhttp.Handler().ServeHTTP(
				&responseWriter{ctx: ctx},
				&http.Request{
					Method: ctx.Method(),
					URL: &url.URL{
						Path: ctx.Path(),
					},
					Header: convertHeaders(ctx),
				},
			)
			return nil
		}

		return fasthttpHandler(c)
	}
}

type responseWriter struct {
	ctx *fiber.Ctx
}

func (w *responseWriter) Header() http.Header {
	headers := make(http.Header)
	w.ctx.Context().Response.Header.VisitAll(func(key, value []byte) {
		headers.Add(string(key), string(value))
	})
	return headers
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.ctx.Context().Response.AppendBody(data)
	return len(data), nil
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.ctx.Context().Response.SetStatusCode(statusCode)
}

func convertHeaders(c *fiber.Ctx) http.Header {
	headers := make(http.Header)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers.Add(string(key), string(value))
	})
	return headers
}
