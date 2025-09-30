package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

// SetupMiddleware configures all middleware for the application
func SetupMiddleware(app *fiber.App) {
	// Request ID middleware
	app.Use(requestid.New())

	// Logger middleware
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path} ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "UTC",
	}))

	// Recover middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Custom error handler
	app.Use(errorHandler())

	// Timing middleware
	app.Use(timingMiddleware())
}

// errorHandler creates a custom error handling middleware
func errorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		if err != nil {
			// Default to 500 Internal Server Error
			code := fiber.StatusInternalServerError
			message := "Internal Server Error"
			errCode := ErrCodeInternalError

			// Check if it's a Fiber error
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				message = e.Message
			}

			// Map common errors to appropriate codes
			switch code {
			case fiber.StatusNotFound:
				errCode = ErrCodeNotFound
			case fiber.StatusBadRequest:
				errCode = ErrCodeInvalidRequest
			case fiber.StatusRequestTimeout:
				errCode = ErrCodeTimeout
			case fiber.StatusTooManyRequests:
				errCode = ErrCodeRateLimited
			}

			// Log the error
			fmt.Printf("Error: %v, Path: %s, Method: %s\n", err, c.Path(), c.Method())

			// Return JSON error response
			return c.Status(code).JSON(NewErrorResponse(message, errCode))
		}
		return nil
	}
}

// timingMiddleware adds request timing headers
func timingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Add timing headers
		c.Set("X-Response-Time", fmt.Sprintf("%d ms", duration.Milliseconds()))

		return err
	}
}

// ValidateAPIKey creates a middleware for API key validation
func ValidateAPIKey(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if apiKey != "" {
			// Get API key from header
			key := c.Get("X-API-Key")
			if key == "" {
				// Try Authorization header
				auth := c.Get("Authorization")
				if auth != "" && len(auth) > 7 && auth[:7] == "Bearer " {
					key = auth[7:]
				}
			}

			if key != apiKey {
				return c.Status(fiber.StatusUnauthorized).JSON(
					NewErrorResponse("Invalid or missing API key", "UNAUTHORIZED"),
				)
			}
		}
		return c.Next()
	}
}

// RateLimiter creates a simple in-memory rate limiter
func RateLimiter(requestsPerMinute int) fiber.Handler {
	type client struct {
		count     int
		lastReset time.Time
	}

	clients := make(map[string]*client)

	return func(c *fiber.Ctx) error {
		// Get client IP
		ip := c.IP()

		now := time.Now()

		// Get or create client entry
		cl, exists := clients[ip]
		if !exists {
			cl = &client{
				count:     0,
				lastReset: now,
			}
			clients[ip] = cl
		}

		// Reset counter if a minute has passed
		if now.Sub(cl.lastReset) > time.Minute {
			cl.count = 0
			cl.lastReset = now
		}

		// Check rate limit
		if cl.count >= requestsPerMinute {
			return c.Status(fiber.StatusTooManyRequests).JSON(
				NewErrorResponse("Rate limit exceeded", ErrCodeRateLimited),
			)
		}

		// Increment counter
		cl.count++

		// Add rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", requestsPerMinute-cl.count))
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", cl.lastReset.Add(time.Minute).Unix()))

		return c.Next()
	}
}

// MetricsMiddleware tracks request metrics
func MetricsMiddleware(metrics *Metrics) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Track metrics
		duration := time.Since(start)
		metrics.RecordRequest(c.Method(), c.Path(), c.Response().StatusCode(), duration)

		return err
	}
}

// Metrics holds application metrics
type Metrics struct {
	totalRequests    int64
	totalErrors      int64
	cacheHits        int64
	cacheMisses      int64
	requestDurations []time.Duration
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		requestDurations: make([]time.Duration, 0, 1000),
	}
}

// RecordRequest records a request metric
func (m *Metrics) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	m.totalRequests++
	if statusCode >= 400 {
		m.totalErrors++
	}
	m.requestDurations = append(m.requestDurations, duration)

	// Keep only last 1000 durations
	if len(m.requestDurations) > 1000 {
		m.requestDurations = m.requestDurations[1:]
	}
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.cacheHits++
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses++
}

// GetStats returns current metrics
func (m *Metrics) GetStats() *MetricsResponse {
	hitRate := float64(0)
	if total := m.cacheHits + m.cacheMisses; total > 0 {
		hitRate = float64(m.cacheHits) / float64(total) * 100
	}

	avgLatency := float64(0)
	if len(m.requestDurations) > 0 {
		var total time.Duration
		for _, d := range m.requestDurations {
			total += d
		}
		avgLatency = float64(total.Milliseconds()) / float64(len(m.requestDurations))
	}

	return &MetricsResponse{
		CacheHits:        m.cacheHits,
		CacheMisses:      m.cacheMisses,
		CacheHitRate:     hitRate,
		TotalRequests:    m.totalRequests,
		TotalErrors:      m.totalErrors,
		AverageLatencyMs: avgLatency,
	}
}
