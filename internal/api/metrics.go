package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Request metrics
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "birbnest_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
	}, []string{"method", "endpoint", "status", "instance_id", "mode"})

	// Cache metrics
	cacheOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "birbnest_cache_operations_total",
		Help: "Total number of cache operations",
	}, []string{"operation", "result", "instance_id", "mode"})

	// Async writer metrics (primary only)
	asyncQueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "birbnest_async_queue_depth",
		Help: "Current depth of async write queue",
	}, []string{"instance_id"})

	asyncQueueCapacity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "birbnest_async_queue_capacity",
		Help: "Total capacity of async write queue",
	}, []string{"instance_id"})

	asyncWriteErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "birbnest_async_write_errors_total",
		Help: "Total number of async write errors",
	}, []string{"instance_id", "error_type"})

	// Write forwarding metrics (replica only)
	writeForwards = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "birbnest_write_forwards_total",
		Help: "Total number of write forwards to primary",
	}, []string{"instance_id", "result"})

	// Primary query metrics (replica only)
	primaryQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "birbnest_primary_queries_total",
		Help: "Total number of queries to primary on cache miss",
	}, []string{"instance_id", "result"})

	// System health
	healthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "birbnest_health_status",
		Help: "Health status (1=healthy, 0.5=degraded, 0=unhealthy)",
	}, []string{"instance_id", "mode"})

	// Default instance metrics
	defaultInstanceRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "birbnest_default_instance_requests_total",
		Help: "Total number of requests using the default instance",
	})

	defaultInstanceCacheOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "birbnest_default_instance_cache_operations_total",
		Help: "Total number of cache operations on the default instance",
	}, []string{"operation", "result"})

	defaultInstanceCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "birbnest_default_instance_created_total",
		Help: "Total number of times the default instance was created",
	})
)

// PrometheusMetricsMiddleware tracks request metrics for Prometheus
func PrometheusMetricsMiddleware(instanceID string, mode string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())

		requestDuration.WithLabelValues(
			c.Method(),
			c.Path(),
			status,
			instanceID,
			mode,
		).Observe(duration)

		// Check if default instance was used
		if usingDefault, ok := c.Locals("using_default_instance").(bool); ok && usingDefault {
			RecordDefaultInstanceRequest()
		}

		// Check if default instance was created
		if created, ok := c.Locals("default_instance_created").(bool); ok && created {
			RecordDefaultInstanceCreated()
		}

		return err
	}
}

// UpdateHealthMetric updates the health status metric
func UpdateHealthMetric(instanceID, mode string, status float64) {
	healthStatus.WithLabelValues(instanceID, mode).Set(status)
}

// RecordCacheOperation records a cache operation metric
func RecordCacheOperation(operation, result, instanceID, mode string) {
	cacheOperations.WithLabelValues(operation, result, instanceID, mode).Inc()
}

// RecordWriteForward records a write forward metric
func RecordWriteForward(instanceID, result string) {
	writeForwards.WithLabelValues(instanceID, result).Inc()
}

// RecordPrimaryQuery records a primary query metric
func RecordPrimaryQuery(instanceID, result string) {
	primaryQueries.WithLabelValues(instanceID, result).Inc()
}

// InitializeAsyncMetrics initializes async writer metrics
func InitializeAsyncMetrics(instanceID string, queueCapacity int) {
	asyncQueueCapacity.WithLabelValues(instanceID).Set(float64(queueCapacity))
	asyncQueueDepth.WithLabelValues(instanceID).Set(0)
}

// RecordDefaultInstanceRequest records a request using the default instance
func RecordDefaultInstanceRequest() {
	defaultInstanceRequests.Inc()
}

// RecordDefaultInstanceCacheOperation records a cache operation on the default instance
func RecordDefaultInstanceCacheOperation(operation, result string) {
	defaultInstanceCacheOperations.WithLabelValues(operation, result).Inc()
}

// RecordDefaultInstanceCreated records the creation of the default instance
func RecordDefaultInstanceCreated() {
	defaultInstanceCreated.Inc()
}
