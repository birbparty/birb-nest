package api

import (
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, handler *Handler, metrics *Metrics, apiKey string) {
	// API v1 group
	v1 := app.Group("/v1")

	// Apply metrics middleware to all v1 routes
	v1.Use(MetricsMiddleware(metrics))

	// Apply rate limiting (100 requests per minute)
	v1.Use(RateLimiter(100))

	// Apply API key validation if configured
	if apiKey != "" {
		v1.Use(ValidateAPIKey(apiKey))
	}

	// Cache endpoints
	cache := v1.Group("/cache")

	// Single key operations
	cache.Get("/:key", handler.GetCache)
	cache.Post("/:key", handler.SetCache)
	cache.Put("/:key", handler.UpdateCache)
	cache.Delete("/:key", handler.DeleteCache)

	// Batch operations
	batch := cache.Group("/batch")
	batch.Post("/get", handler.BatchGet)

	// Health and metrics endpoints (no auth required)
	app.Get("/health", handler.Health)
	app.Get("/metrics", handler.GetMetrics)

	// Root endpoint
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": "birb-nest-api",
			"version": "1.0.0",
			"status":  "running",
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
		return c.Status(fiber.StatusNotFound).JSON(
			NewErrorResponse("Endpoint not found", ErrCodeNotFound),
		)
	})
}
