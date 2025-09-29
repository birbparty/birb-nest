package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var (
	// Metrics instances
	metricsOnce sync.Once

	// Cache metrics
	cacheHits              prometheus.Counter
	cacheMisses            prometheus.Counter
	cacheOperationDuration *prometheus.HistogramVec
	cacheSize              prometheus.Gauge

	// API metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	activeConnections   prometheus.Gauge

	// Queue metrics
	messagesProcessedTotal    *prometheus.CounterVec
	messageProcessingDuration *prometheus.HistogramVec
	queueDepth                *prometheus.GaugeVec
	batchSize                 *prometheus.HistogramVec
	dlqMessagesTotal          *prometheus.CounterVec

	// System metrics
	serviceUp                 prometheus.Gauge
	databaseConnectionsActive prometheus.Gauge
	redisConnectionsActive    prometheus.Gauge

	// File exporter for local-otel
	fileExporter *FileMetricsExporter
)

// FileMetricsExporter exports metrics to a file for local-otel integration
type FileMetricsExporter struct {
	mu       sync.Mutex
	filePath string
	metrics  map[string]interface{}
}

// InitMetrics initializes all metrics
func InitMetrics(cfg *Config) error {
	var err error
	metricsOnce.Do(func() {
		// Initialize Prometheus metrics
		initPrometheusMetrics()

		// Initialize OpenTelemetry metrics if enabled
		if cfg.EnableMetrics {
			err = initOTELMetrics(cfg)
		}

		// Initialize file exporter if enabled
		if cfg.ExportToFile && cfg.MetricsFilePath != "" {
			fileExporter = &FileMetricsExporter{
				filePath: cfg.MetricsFilePath,
				metrics:  make(map[string]interface{}),
			}
			// Start periodic file export
			go fileExporter.startPeriodicExport(time.Duration(cfg.MetricsInterval) * time.Second)
		}

		// Set service as up
		serviceUp.Set(1)
	})
	return err
}

func initPrometheusMetrics() {
	// Cache metrics
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total number of cache hits",
	})

	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total number of cache misses",
	})

	cacheOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_operation_duration_seconds",
		Help:    "Duration of cache operations in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "status"})

	cacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cache_size_bytes",
		Help: "Current size of the cache in bytes",
	})

	// API metrics
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "endpoint", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})

	activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_connections",
		Help: "Number of active HTTP connections",
	})

	// Queue metrics
	messagesProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "messages_processed_total",
		Help: "Total number of messages processed",
	}, []string{"type", "status"})

	messageProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "message_processing_duration_seconds",
		Help:    "Duration of message processing in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})

	queueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "queue_depth",
		Help: "Current depth of the queue",
	}, []string{"queue"})

	batchSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "batch_size",
		Help:    "Size of processing batches",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{"type"})

	dlqMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dlq_messages_total",
		Help: "Total number of messages sent to DLQ",
	}, []string{"reason"})

	// System metrics
	serviceUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "service_up",
		Help: "Whether the service is up (1) or down (0)",
	})

	databaseConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "database_connections_active",
		Help: "Number of active database connections",
	})

	redisConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "redis_connections_active",
		Help: "Number of active Redis connections",
	})
}

func initOTELMetrics(cfg *Config) error {
	ctx := context.Background()

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP exporter if not in file export mode
	if !cfg.ExportToFile {
		exporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return fmt.Errorf("failed to create metrics exporter: %w", err)
		}

		// Create meter provider
		provider := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(
				sdkmetric.NewPeriodicReader(
					exporter,
					sdkmetric.WithInterval(time.Duration(cfg.MetricsInterval)*time.Second),
				),
			),
		)

		otel.SetMeterProvider(provider)
	}

	return nil
}

// FileMetricsExporter methods

func (f *FileMetricsExporter) startPeriodicExport(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := f.export(); err != nil {
			L().WithError(err).Error("Failed to export metrics to file")
		}
	}
}

func (f *FileMetricsExporter) export() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Gather current metric values
	f.updateMetrics()

	// Ensure directory exists
	dir := filepath.Dir(f.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to file
	file, err := os.Create(f.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(f.metrics)
}

func (f *FileMetricsExporter) updateMetrics() {
	// This is a simplified version - in a real implementation,
	// you'd gather actual metric values from Prometheus
	f.metrics["timestamp"] = time.Now().Unix()
	f.metrics["cache_hits_total"] = getCounterValue(cacheHits)
	f.metrics["cache_misses_total"] = getCounterValue(cacheMisses)
	f.metrics["service_up"] = getGaugeValue(serviceUp)
	f.metrics["database_connections_active"] = getGaugeValue(databaseConnectionsActive)
	f.metrics["redis_connections_active"] = getGaugeValue(redisConnectionsActive)
}

// Helper functions to get metric values (simplified)
func getCounterValue(counter prometheus.Counter) float64 {
	// In a real implementation, you'd use prometheus.Gatherer
	return 0
}

func getGaugeValue(gauge prometheus.Gauge) float64 {
	// In a real implementation, you'd use prometheus.Gatherer
	return 0
}

// Metric recording functions

// RecordCacheHit records a cache hit
func RecordCacheHit() {
	cacheHits.Inc()
	if fileExporter != nil {
		fileExporter.mu.Lock()
		fileExporter.metrics["cache_hits_total"] = fileExporter.metrics["cache_hits_total"].(float64) + 1
		fileExporter.mu.Unlock()
	}
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss() {
	cacheMisses.Inc()
	if fileExporter != nil {
		fileExporter.mu.Lock()
		fileExporter.metrics["cache_misses_total"] = fileExporter.metrics["cache_misses_total"].(float64) + 1
		fileExporter.mu.Unlock()
	}
}

// RecordCacheOperation records a cache operation duration
func RecordCacheOperation(operation string, status string, duration time.Duration) {
	cacheOperationDuration.WithLabelValues(operation, status).Observe(duration.Seconds())
}

// RecordHTTPRequest records an HTTP request
func RecordHTTPRequest(method, endpoint, status string, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordMessageProcessed records a processed message
func RecordMessageProcessed(msgType, status string, duration time.Duration) {
	messagesProcessedTotal.WithLabelValues(msgType, status).Inc()
	messageProcessingDuration.WithLabelValues(msgType).Observe(duration.Seconds())
}

// RecordBatchSize records the size of a processing batch
func RecordBatchSize(batchType string, size int) {
	batchSize.WithLabelValues(batchType).Observe(float64(size))
}

// RecordDLQMessage records a message sent to DLQ
func RecordDLQMessage(reason string) {
	dlqMessagesTotal.WithLabelValues(reason).Inc()
}

// UpdateQueueDepth updates the queue depth metric
func UpdateQueueDepth(queue string, depth int) {
	queueDepth.WithLabelValues(queue).Set(float64(depth))
}

// UpdateCacheSize updates the cache size metric
func UpdateCacheSize(bytes int64) {
	cacheSize.Set(float64(bytes))
}

// UpdateActiveConnections updates the active connections metric
func UpdateActiveConnections(count int) {
	activeConnections.Set(float64(count))
}

// UpdateDatabaseConnections updates the database connections metric
func UpdateDatabaseConnections(count int) {
	databaseConnectionsActive.Set(float64(count))
}

// UpdateRedisConnections updates the Redis connections metric
func UpdateRedisConnections(count int) {
	redisConnectionsActive.Set(float64(count))
}
