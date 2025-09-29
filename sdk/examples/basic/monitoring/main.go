// Monitoring Example
// This example demonstrates how to expose metrics for Prometheus
// and implement custom monitoring for the Birb-Nest SDK.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/birbparty/birb-nest/sdk"
)

// MetricsCollector collects metrics about cache operations
type MetricsCollector struct {
	mu sync.RWMutex

	// Counters
	totalRequests    int64
	cacheHits        int64
	cacheMisses      int64
	setOperations    int64
	getOperations    int64
	deleteOperations int64
	errors           int64

	// Latency tracking
	latencies []time.Duration

	// Error tracking
	errorsByType map[string]int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		errorsByType: make(map[string]int64),
		latencies:    make([]time.Duration, 0, 1000),
	}
}

// RecordOperation records a cache operation
func (m *MetricsCollector) RecordOperation(op string, duration time.Duration, err error) {
	atomic.AddInt64(&m.totalRequests, 1)

	switch op {
	case "get":
		atomic.AddInt64(&m.getOperations, 1)
		if err == nil {
			atomic.AddInt64(&m.cacheHits, 1)
		} else if sdk.IsNotFound(err) {
			atomic.AddInt64(&m.cacheMisses, 1)
		}
	case "set":
		atomic.AddInt64(&m.setOperations, 1)
	case "delete":
		atomic.AddInt64(&m.deleteOperations, 1)
	}

	if err != nil && !sdk.IsNotFound(err) {
		atomic.AddInt64(&m.errors, 1)
		m.recordError(err)
	}

	m.recordLatency(duration)
}

func (m *MetricsCollector) recordError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	errType := "unknown"
	var sdkErr *sdk.Error
	if errors.As(err, &sdkErr) {
		errType = sdkErr.Type.String()
	}

	m.errorsByType[errType]++
}

func (m *MetricsCollector) recordLatency(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latencies = append(m.latencies, duration)
	// Keep only last 1000 latencies to prevent unbounded growth
	if len(m.latencies) > 1000 {
		m.latencies = m.latencies[len(m.latencies)-1000:]
	}
}

// GetMetrics returns current metrics
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate percentiles
	var p50, p95, p99 time.Duration
	if len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies)
		// Simple bubble sort for demo (use sort.Slice in production)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		p50 = sorted[len(sorted)*50/100]
		p95 = sorted[len(sorted)*95/100]
		p99 = sorted[len(sorted)*99/100]
	}

	hitRate := float64(0)
	totalCacheOps := atomic.LoadInt64(&m.cacheHits) + atomic.LoadInt64(&m.cacheMisses)
	if totalCacheOps > 0 {
		hitRate = float64(atomic.LoadInt64(&m.cacheHits)) / float64(totalCacheOps) * 100
	}

	return map[string]interface{}{
		"total_requests":    atomic.LoadInt64(&m.totalRequests),
		"cache_hits":        atomic.LoadInt64(&m.cacheHits),
		"cache_misses":      atomic.LoadInt64(&m.cacheMisses),
		"cache_hit_rate":    fmt.Sprintf("%.2f%%", hitRate),
		"set_operations":    atomic.LoadInt64(&m.setOperations),
		"get_operations":    atomic.LoadInt64(&m.getOperations),
		"delete_operations": atomic.LoadInt64(&m.deleteOperations),
		"errors":            atomic.LoadInt64(&m.errors),
		"errors_by_type":    m.errorsByType,
		"latency_p50_ms":    p50.Milliseconds(),
		"latency_p95_ms":    p95.Milliseconds(),
		"latency_p99_ms":    p99.Milliseconds(),
	}
}

// MonitoredClient wraps the SDK client with monitoring
type MonitoredClient struct {
	client  sdk.Client
	metrics *MetricsCollector
}

// NewMonitoredClient creates a client with monitoring
func NewMonitoredClient(client sdk.Client, metrics *MetricsCollector) *MonitoredClient {
	return &MonitoredClient{
		client:  client,
		metrics: metrics,
	}
}

// Get performs a monitored get operation
func (mc *MonitoredClient) Get(ctx context.Context, key string, value interface{}) error {
	start := time.Now()
	err := mc.client.Get(ctx, key, value)
	mc.metrics.RecordOperation("get", time.Since(start), err)
	return err
}

// Set performs a monitored set operation
func (mc *MonitoredClient) Set(ctx context.Context, key string, value interface{}) error {
	start := time.Now()
	err := mc.client.Set(ctx, key, value)
	mc.metrics.RecordOperation("set", time.Since(start), err)
	return err
}

// Delete performs a monitored delete operation
func (mc *MonitoredClient) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := mc.client.Delete(ctx, key)
	mc.metrics.RecordOperation("delete", time.Since(start), err)
	return err
}

func main() {
	// Create base client
	baseClient, err := sdk.NewClient(sdk.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer baseClient.Close()

	// Create metrics collector
	metrics := NewMetricsCollector()

	// Wrap client with monitoring
	client := NewMonitoredClient(baseClient, metrics)

	// Start metrics HTTP server
	go startMetricsServer(metrics)

	// Run example workload
	fmt.Println("Starting monitored cache operations...")
	fmt.Println("Metrics available at http://localhost:9090/metrics")
	fmt.Println("")

	ctx := context.Background()

	// Simulate realistic workload
	runWorkload(ctx, client)

	// Display final metrics
	fmt.Println("\n=== Final Metrics ===")
	displayMetrics(metrics.GetMetrics())
}

func startMetricsServer(metrics *MetricsCollector) {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// Prometheus format (simplified)
		m := metrics.GetMetrics()

		fmt.Fprintf(w, "# HELP birb_nest_requests_total Total number of requests\n")
		fmt.Fprintf(w, "# TYPE birb_nest_requests_total counter\n")
		fmt.Fprintf(w, "birb_nest_requests_total %d\n\n", m["total_requests"])

		fmt.Fprintf(w, "# HELP birb_nest_cache_hits_total Total number of cache hits\n")
		fmt.Fprintf(w, "# TYPE birb_nest_cache_hits_total counter\n")
		fmt.Fprintf(w, "birb_nest_cache_hits_total %d\n\n", m["cache_hits"])

		fmt.Fprintf(w, "# HELP birb_nest_cache_misses_total Total number of cache misses\n")
		fmt.Fprintf(w, "# TYPE birb_nest_cache_misses_total counter\n")
		fmt.Fprintf(w, "birb_nest_cache_misses_total %d\n\n", m["cache_misses"])

		fmt.Fprintf(w, "# HELP birb_nest_errors_total Total number of errors\n")
		fmt.Fprintf(w, "# TYPE birb_nest_errors_total counter\n")
		fmt.Fprintf(w, "birb_nest_errors_total %d\n\n", m["errors"])

		fmt.Fprintf(w, "# HELP birb_nest_latency_milliseconds Request latency in milliseconds\n")
		fmt.Fprintf(w, "# TYPE birb_nest_latency_milliseconds summary\n")
		fmt.Fprintf(w, "birb_nest_latency_milliseconds{quantile=\"0.5\"} %d\n", m["latency_p50_ms"])
		fmt.Fprintf(w, "birb_nest_latency_milliseconds{quantile=\"0.95\"} %d\n", m["latency_p95_ms"])
		fmt.Fprintf(w, "birb_nest_latency_milliseconds{quantile=\"0.99\"} %d\n", m["latency_p99_ms"])
	})

	http.HandleFunc("/metrics/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics.GetMetrics())
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("Metrics server listening on :9090")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

func runWorkload(ctx context.Context, client *MonitoredClient) {
	// Simulate a realistic workload mix
	workloadMix := []struct {
		name   string
		weight int
		fn     func(ctx context.Context, client *MonitoredClient, i int)
	}{
		{
			name:   "cache_hit",
			weight: 60, // 60% cache hits
			fn: func(ctx context.Context, client *MonitoredClient, i int) {
				// Set once, get many times
				key := fmt.Sprintf("popular:key:%d", i%10)
				if i < 10 {
					client.Set(ctx, key, fmt.Sprintf("value-%d", i))
				}
				var value string
				client.Get(ctx, key, &value)
			},
		},
		{
			name:   "cache_miss",
			weight: 20, // 20% cache misses
			fn: func(ctx context.Context, client *MonitoredClient, i int) {
				key := fmt.Sprintf("missing:key:%d", i)
				var value string
				client.Get(ctx, key, &value)
			},
		},
		{
			name:   "write_operation",
			weight: 15, // 15% writes
			fn: func(ctx context.Context, client *MonitoredClient, i int) {
				key := fmt.Sprintf("data:key:%d", i)
				value := map[string]interface{}{
					"id":        i,
					"timestamp": time.Now().Unix(),
					"data":      fmt.Sprintf("payload-%d", i),
				}
				client.Set(ctx, key, value)
			},
		},
		{
			name:   "delete_operation",
			weight: 5, // 5% deletes
			fn: func(ctx context.Context, client *MonitoredClient, i int) {
				key := fmt.Sprintf("data:key:%d", i-10)
				client.Delete(ctx, key)
			},
		},
	}

	// Run workload for 10 seconds
	fmt.Println("Running workload for 10 seconds...")
	start := time.Now()
	i := 0

	for time.Since(start) < 10*time.Second {
		// Select operation based on weights
		r := i % 100
		cumWeight := 0

		for _, work := range workloadMix {
			cumWeight += work.weight
			if r < cumWeight {
				work.fn(ctx, client, i)
				break
			}
		}

		i++

		// Add some variability to request rate
		if i%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	fmt.Printf("Completed %d operations in %v\n", i, time.Since(start))
}

func displayMetrics(metrics map[string]interface{}) {
	fmt.Printf("Total Requests: %d\n", metrics["total_requests"])
	fmt.Printf("Cache Hits: %d\n", metrics["cache_hits"])
	fmt.Printf("Cache Misses: %d\n", metrics["cache_misses"])
	fmt.Printf("Cache Hit Rate: %s\n", metrics["cache_hit_rate"])
	fmt.Printf("Errors: %d\n", metrics["errors"])
	fmt.Printf("\nOperations Breakdown:\n")
	fmt.Printf("  GET: %d\n", metrics["get_operations"])
	fmt.Printf("  SET: %d\n", metrics["set_operations"])
	fmt.Printf("  DELETE: %d\n", metrics["delete_operations"])
	fmt.Printf("\nLatency Percentiles:\n")
	fmt.Printf("  P50: %dms\n", metrics["latency_p50_ms"])
	fmt.Printf("  P95: %dms\n", metrics["latency_p95_ms"])
	fmt.Printf("  P99: %dms\n", metrics["latency_p99_ms"])

	if errorsByType, ok := metrics["errors_by_type"].(map[string]int64); ok && len(errorsByType) > 0 {
		fmt.Printf("\nErrors by Type:\n")
		for errType, count := range errorsByType {
			fmt.Printf("  %s: %d\n", errType, count)
		}
	}
}
