package sdk

import (
	"sync"
	"time"
)

// Observer provides hooks for monitoring SDK operations.
// Implement this interface to track performance metrics, debug issues,
// or integrate with your observability stack.
//
// The SDK calls observer methods at key points during operation execution.
// Observer methods should be fast and non-blocking to avoid impacting performance.
//
// Example implementation:
//
//	type LogObserver struct {
//	    logger *log.Logger
//	}
//
//	func (o *LogObserver) OnRequestStart(method, path string) {
//	    o.logger.Printf("[START] %s %s", method, path)
//	}
//
//	func (o *LogObserver) OnRequestEnd(method, path string, duration time.Duration, err error) {
//	    if err != nil {
//	        o.logger.Printf("[ERROR] %s %s - %v (took %v)", method, path, err, duration)
//	    } else {
//	        o.logger.Printf("[SUCCESS] %s %s (took %v)", method, path, duration)
//	    }
//	}
//
//	config := sdk.DefaultConfig().
//	    WithObserver(&LogObserver{logger: log.Default()})
type Observer interface {
	// OnRequestStart is called when an HTTP request starts.
	// Use this to track request rates or log request initiation.
	//
	// Parameters:
	//   - method: HTTP method (GET, POST, DELETE)
	//   - path: Request path (e.g., "/v1/cache/key123")
	OnRequestStart(method, path string)

	// OnRequestEnd is called when an HTTP request completes.
	// Use this to track latencies, error rates, or log completions.
	//
	// Parameters:
	//   - method: HTTP method
	//   - path: Request path
	//   - duration: Time taken for the request
	//   - err: Error if request failed, nil on success
	OnRequestEnd(method, path string, duration time.Duration, err error)

	// OnRetryAttempt is called for each retry attempt.
	// Use this to track retry rates or debug retry behavior.
	//
	// Parameters:
	//   - method: HTTP method
	//   - path: Request path
	//   - attempt: Retry attempt number (1, 2, 3...)
	//   - delay: Delay before this retry attempt
	//   - err: The error that triggered the retry
	OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error)

	// OnCircuitBreakerStateChange is called when a circuit breaker changes state.
	// Use this to monitor service health or alert on circuit opens.
	//
	// Parameters:
	//   - endpoint: The endpoint whose circuit changed
	//   - oldState: Previous circuit state
	//   - newState: New circuit state
	OnCircuitBreakerStateChange(endpoint string, oldState, newState CircuitState)

	// OnCacheHit is called when a cache key is found.
	// Use this to track cache hit rates.
	//
	// Parameters:
	//   - key: The cache key that was hit
	OnCacheHit(key string)

	// OnCacheMiss is called when a cache key is not found.
	// Use this to track cache miss rates.
	//
	// Parameters:
	//   - key: The cache key that was missed
	OnCacheMiss(key string)
}

// NoopObserver is a no-op implementation of Observer that does nothing.
// This is the default observer used when none is configured.
// It has zero overhead and is safe for production use.
//
// Example:
//
//	// Explicitly use no-op observer (same as default)
//	config := sdk.DefaultConfig().
//	    WithObserver(&sdk.NoopObserver{})
type NoopObserver struct{}

// OnRequestStart does nothing
func (n *NoopObserver) OnRequestStart(method, path string) {}

// OnRequestEnd does nothing
func (n *NoopObserver) OnRequestEnd(method, path string, duration time.Duration, err error) {}

// OnRetryAttempt does nothing
func (n *NoopObserver) OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error) {
}

// OnCircuitBreakerStateChange does nothing
func (n *NoopObserver) OnCircuitBreakerStateChange(endpoint string, oldState, newState CircuitState) {
}

// OnCacheHit does nothing
func (n *NoopObserver) OnCacheHit(key string) {}

// OnCacheMiss does nothing
func (n *NoopObserver) OnCacheMiss(key string) {}

// MetricsCollector is a simple in-memory metrics implementation.
// It collects basic metrics about SDK operations including request counts,
// latencies, error rates, retry attempts, and cache hit rates.
//
// Note: This implementation stores all data in memory and is primarily
// intended for debugging and testing. For production use, consider
// implementing Observer to export metrics to your monitoring system.
//
// Example:
//
//	metrics := sdk.NewMetricsCollector()
//	config := sdk.DefaultConfig().
//	    WithObserver(metrics)
//
//	client, _ := sdk.NewClient(config)
//	// Use client...
//
//	// Get metrics snapshot
//	snapshot := metrics.GetMetrics()
//	fmt.Printf("Cache hit rate: %.2f%%\n", snapshot["cache_hit_rate"].(float64) * 100)
//	fmt.Printf("Total requests: %v\n", snapshot["requests"])
type MetricsCollector struct {
	mu                  sync.RWMutex
	requestCount        map[string]int64
	latencies           map[string][]time.Duration
	errorCount          map[string]int64
	retryCount          map[string]int64
	circuitStateChanges map[string]int64
	cacheHitCount       int64
	cacheMissCount      int64
}

// NewMetricsCollector creates a new metrics collector for tracking SDK operations.
// The collector is thread-safe and can be used concurrently.
//
// Example:
//
//	metrics := sdk.NewMetricsCollector()
//
//	// Use with client
//	config := sdk.DefaultConfig().WithObserver(metrics)
//	client, _ := sdk.NewClient(config)
//
//	// Later, examine metrics
//	data := metrics.GetMetrics()
//	for endpoint, count := range data["requests"].(map[string]int64) {
//	    fmt.Printf("%s: %d requests\n", endpoint, count)
//	}
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestCount:        make(map[string]int64),
		latencies:           make(map[string][]time.Duration),
		errorCount:          make(map[string]int64),
		retryCount:          make(map[string]int64),
		circuitStateChanges: make(map[string]int64),
	}
}

// OnRequestStart increments request count
func (m *MetricsCollector) OnRequestStart(method, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + path
	m.requestCount[key]++
}

// OnRequestEnd records request duration and errors
func (m *MetricsCollector) OnRequestEnd(method, path string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + path
	m.latencies[key] = append(m.latencies[key], duration)
	if err != nil {
		m.errorCount[key]++
	}
}

// OnRetryAttempt increments retry count
func (m *MetricsCollector) OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + path
	m.retryCount[key]++
}

// OnCircuitBreakerStateChange tracks state changes
func (m *MetricsCollector) OnCircuitBreakerStateChange(endpoint string, oldState, newState CircuitState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitStateChanges[endpoint]++
}

// OnCacheHit increments cache hit count
func (m *MetricsCollector) OnCacheHit(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheHitCount++
}

// OnCacheMiss increments cache miss count
func (m *MetricsCollector) OnCacheMiss(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheMissCount++
}

// GetMetrics returns a snapshot of current metrics.
// The returned map is a copy and safe to read without locks.
//
// The metrics include:
//   - "requests": Map of endpoint to request count
//   - "latencies": Map of endpoint to latency measurements
//   - "errors": Map of endpoint to error count
//   - "retries": Map of endpoint to retry count
//   - "circuit_breaker_state_changes": Map of endpoint to state change count
//   - "cache_hits": Total cache hits
//   - "cache_misses": Total cache misses
//   - "cache_hit_rate": Calculated hit rate (0.0 to 1.0)
//
// Example:
//
//	metrics := collector.GetMetrics()
//
//	// Check cache performance
//	hitRate := metrics["cache_hit_rate"].(float64)
//	if hitRate < 0.8 {
//	    log.Printf("Warning: Low cache hit rate: %.2f%%", hitRate * 100)
//	}
//
//	// Check error rates
//	errors := metrics["errors"].(map[string]int64)
//	for endpoint, count := range errors {
//	    if count > 100 {
//	        log.Printf("High error count for %s: %d", endpoint, count)
//	    }
//	}
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create copies to avoid data races
	requestsCopy := make(map[string]int64)
	for k, v := range m.requestCount {
		requestsCopy[k] = v
	}

	latenciesCopy := make(map[string][]time.Duration)
	for k, v := range m.latencies {
		latenciesCopy[k] = append([]time.Duration(nil), v...)
	}

	errorsCopy := make(map[string]int64)
	for k, v := range m.errorCount {
		errorsCopy[k] = v
	}

	retriesCopy := make(map[string]int64)
	for k, v := range m.retryCount {
		retriesCopy[k] = v
	}

	circuitChangesCopy := make(map[string]int64)
	for k, v := range m.circuitStateChanges {
		circuitChangesCopy[k] = v
	}

	cacheTotal := m.cacheHitCount + m.cacheMissCount
	cacheHitRate := float64(0)
	if cacheTotal > 0 {
		cacheHitRate = float64(m.cacheHitCount) / float64(cacheTotal)
	}

	return map[string]interface{}{
		"requests":                      requestsCopy,
		"latencies":                     latenciesCopy,
		"errors":                        errorsCopy,
		"retries":                       retriesCopy,
		"circuit_breaker_state_changes": circuitChangesCopy,
		"cache_hits":                    m.cacheHitCount,
		"cache_misses":                  m.cacheMissCount,
		"cache_hit_rate":                cacheHitRate,
	}
}

// observedCircuitBreaker wraps a circuit breaker to notify observers of state changes.
// This allows monitoring systems to track circuit breaker behavior without
// modifying the circuit breaker implementation.
type observedCircuitBreaker struct {
	cb        CircuitBreaker
	endpoint  string
	observer  Observer
	lastState CircuitState
}

// newObservedCircuitBreaker creates a circuit breaker that notifies an observer
// of state changes. This is used internally by the SDK to integrate circuit
// breakers with the observer system.
//
// Parameters:
//   - cb: The circuit breaker to wrap
//   - endpoint: The endpoint this circuit breaker protects
//   - observer: The observer to notify of state changes
func newObservedCircuitBreaker(cb CircuitBreaker, endpoint string, observer Observer) CircuitBreaker {
	return &observedCircuitBreaker{
		cb:        cb,
		endpoint:  endpoint,
		observer:  observer,
		lastState: cb.State(),
	}
}

// Execute runs the function and notifies state changes
func (o *observedCircuitBreaker) Execute(fn func() error) error {
	err := o.cb.Execute(fn)

	// Check for state change
	currentState := o.cb.State()
	if currentState != o.lastState {
		o.observer.OnCircuitBreakerStateChange(o.endpoint, o.lastState, currentState)
		o.lastState = currentState
	}

	return err
}

// State returns the current state
func (o *observedCircuitBreaker) State() CircuitState {
	return o.cb.State()
}

// Reset resets the circuit and notifies of state change
func (o *observedCircuitBreaker) Reset() {
	oldState := o.cb.State()
	o.cb.Reset()
	newState := o.cb.State()

	if oldState != newState {
		o.observer.OnCircuitBreakerStateChange(o.endpoint, oldState, newState)
		o.lastState = newState
	}
}

// CompositeObserver allows multiple observers to be combined into one.
// All observer methods are called on each child observer in order.
// If an observer panics, it's caught to prevent affecting other observers.
//
// This is useful for combining different monitoring approaches:
//   - Logging observer for debugging
//   - Metrics observer for monitoring
//   - Tracing observer for distributed tracing
//
// Example:
//
//	logger := &LogObserver{log: log.Default()}
//	metrics := sdk.NewMetricsCollector()
//	tracer := &TracingObserver{tracer: myTracer}
//
//	composite := sdk.NewCompositeObserver(logger, metrics, tracer)
//
//	config := sdk.DefaultConfig().
//	    WithObserver(composite)
type CompositeObserver struct {
	observers []Observer
}

// NewCompositeObserver creates an observer that delegates to multiple observers.
// This allows you to use multiple monitoring strategies simultaneously.
//
// Example:
//
//	// Combine logging and metrics
//	observer := sdk.NewCompositeObserver(
//	    &ConsoleLogObserver{},
//	    sdk.NewMetricsCollector(),
//	    &PrometheusExporter{},
//	)
//
//	config := sdk.DefaultConfig().WithObserver(observer)
func NewCompositeObserver(observers ...Observer) Observer {
	return &CompositeObserver{observers: observers}
}

// OnRequestStart notifies all observers of request start.
// If an observer panics, the panic is caught and ignored to prevent
// one faulty observer from affecting others.
func (c *CompositeObserver) OnRequestStart(method, path string) {
	for _, obs := range c.observers {
		// Recover from panics to prevent one observer from breaking others
		func() {
			defer func() {
				if r := recover(); r != nil {
					// In production, you might want to log this
				}
			}()
			obs.OnRequestStart(method, path)
		}()
	}
}

// OnRequestEnd notifies all observers of request completion.
// Each observer is called in order with panic protection.
func (c *CompositeObserver) OnRequestEnd(method, path string, duration time.Duration, err error) {
	for _, obs := range c.observers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Observer panicked, ignore
				}
			}()
			obs.OnRequestEnd(method, path, duration, err)
		}()
	}
}

// OnRetryAttempt notifies all observers
func (c *CompositeObserver) OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error) {
	for _, obs := range c.observers {
		obs.OnRetryAttempt(method, path, attempt, delay, err)
	}
}

// OnCircuitBreakerStateChange notifies all observers
func (c *CompositeObserver) OnCircuitBreakerStateChange(endpoint string, oldState, newState CircuitState) {
	for _, obs := range c.observers {
		obs.OnCircuitBreakerStateChange(endpoint, oldState, newState)
	}
}

// OnCacheHit notifies all observers
func (c *CompositeObserver) OnCacheHit(key string) {
	for _, obs := range c.observers {
		obs.OnCacheHit(key)
	}
}

// OnCacheMiss notifies all observers
func (c *CompositeObserver) OnCacheMiss(key string) {
	for _, obs := range c.observers {
		obs.OnCacheMiss(key)
	}
}
