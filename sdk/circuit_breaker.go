package sdk

import (
	"errors"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
// The circuit breaker pattern prevents cascading failures by monitoring
// error rates and temporarily blocking requests when a threshold is exceeded.
//
// State transitions:
//   - Closed -> Open: When failure threshold is reached
//   - Open -> Half-Open: After timeout period expires
//   - Half-Open -> Closed: When success threshold is reached
//   - Half-Open -> Open: On any failure
//
// Example:
//
//	if client.CircuitBreaker().State() == sdk.CircuitOpen {
//	    // Service is down, fail fast
//	    return errors.New("service unavailable")
//	}
type CircuitState int

const (
	// CircuitClosed is the normal operating state.
	// All requests pass through and errors are counted.
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks all requests immediately.
	// This state prevents overwhelming a failing service.
	CircuitOpen
	// CircuitHalfOpen allows limited requests to test if the service has recovered.
	// If these test requests succeed, the circuit closes.
	// If they fail, the circuit opens again.
	CircuitHalfOpen
)

// String returns the string representation of the circuit state
func (cs CircuitState) String() string {
	switch cs {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker protects resources by preventing cascading failures.
// It monitors the error rate of operations and temporarily blocks requests
// when the failure rate exceeds a threshold, giving the protected resource
// time to recover.
//
// The circuit breaker is particularly useful for:
//   - Preventing cascading failures in distributed systems
//   - Providing fail-fast behavior when services are down
//   - Reducing load on struggling services
//   - Improving overall system resilience
//
// Example usage:
//
//	config := sdk.DefaultConfig().WithCircuitBreaker(sdk.CircuitBreakerConfig{
//	    FailureThreshold: 5,      // Open after 5 consecutive failures
//	    SuccessThreshold: 2,      // Close after 2 consecutive successes
//	    Timeout:          30 * time.Second, // Try recovery after 30s
//	})
//
//	client, _ := sdk.NewClient(config)
//
//	// The SDK automatically uses the circuit breaker for all operations
//	err := client.Set(ctx, "key", "value")
//	if errors.Is(err, sdk.ErrCircuitOpen) {
//	    // Circuit is open, service is unavailable
//	}
type CircuitBreaker interface {
	// Execute runs the given function if the circuit allows it.
	// Returns ErrCircuitOpen if the circuit is open.
	// The function's error (if any) is used to update circuit state.
	//
	// Example:
	//
	//	err := cb.Execute(func() error {
	//	    return riskyOperation()
	//	})
	//	if errors.Is(err, sdk.ErrCircuitOpen) {
	//	    // Circuit prevented the operation
	//	}
	Execute(fn func() error) error

	// State returns the current state of the circuit breaker.
	// This is useful for monitoring and debugging.
	//
	// Example:
	//
	//	switch cb.State() {
	//	case sdk.CircuitClosed:
	//	    log.Println("Circuit is operating normally")
	//	case sdk.CircuitOpen:
	//	    log.Println("Circuit is open, requests are blocked")
	//	case sdk.CircuitHalfOpen:
	//	    log.Println("Circuit is testing recovery")
	//	}
	State() CircuitState

	// Reset manually resets the circuit to closed state.
	// This should be used sparingly, typically only when you know
	// the underlying issue has been resolved.
	//
	// Example:
	//
	//	// After fixing the underlying issue
	//	cb.Reset()
	Reset()
}

// CircuitBreakerConfig holds configuration for circuit breaker behavior.
// All fields have sensible defaults if not specified.
//
// Example:
//
//	config := sdk.CircuitBreakerConfig{
//	    FailureThreshold: 10,     // Open after 10 failures
//	    SuccessThreshold: 3,      // Need 3 successes to close
//	    Timeout:          time.Minute, // Wait 1 minute before trying
//	    HalfOpenRequests: 5,      // Allow 5 test requests
//	}
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before
	// the circuit opens. Lower values make the circuit more sensitive.
	// Default: 5
	FailureThreshold int

	// SuccessThreshold is the number of consecutive successes required
	// in half-open state before the circuit closes.
	// Default: 2
	SuccessThreshold int

	// Timeout is how long the circuit stays open before transitioning
	// to half-open state to test recovery.
	// Default: 30s
	Timeout time.Duration

	// HalfOpenRequests is the maximum number of requests allowed
	// in half-open state. This limits the test traffic to the
	// recovering service.
	// Default: 3
	HalfOpenRequests int
}

// DefaultCircuitBreakerConfig returns a circuit breaker configuration
// with sensible defaults suitable for most use cases.
//
// Default values:
//   - FailureThreshold: 5 (opens after 5 consecutive failures)
//   - SuccessThreshold: 2 (closes after 2 consecutive successes)
//   - Timeout: 30s (waits 30 seconds before testing recovery)
//   - HalfOpenRequests: 3 (allows 3 test requests in half-open state)
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithCircuitBreaker(sdk.DefaultCircuitBreakerConfig())
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 3,
	}
}

// circuitBreaker is the default implementation
type circuitBreaker struct {
	config CircuitBreakerConfig

	mu               sync.Mutex
	state            CircuitState
	failures         int
	successes        int
	halfOpenRequests int
	lastFailureTime  time.Time
	lastStateChange  time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
// The circuit breaker starts in the closed state.
//
// Example:
//
//	cb := sdk.NewCircuitBreaker(sdk.CircuitBreakerConfig{
//	    FailureThreshold: 5,
//	    SuccessThreshold: 2,
//	    Timeout:          30 * time.Second,
//	})
//
//	err := cb.Execute(func() error {
//	    return someRiskyOperation()
//	})
func NewCircuitBreaker(config CircuitBreakerConfig) CircuitBreaker {
	return &circuitBreaker{
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Execute runs the given function if the circuit allows it
func (cb *circuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition states
	cb.checkStateTransition()

	state := cb.state

	// If circuit is open, reject immediately
	if state == CircuitOpen {
		cb.mu.Unlock()
		return NewError(ErrorTypeCircuitOpen, "circuit breaker is open", ErrCircuitOpen)
	}

	// If half-open, check if we've exceeded the limit
	if state == CircuitHalfOpen {
		if cb.halfOpenRequests >= cb.config.HalfOpenRequests {
			cb.mu.Unlock()
			return NewError(ErrorTypeCircuitOpen, "circuit breaker half-open limit reached", ErrCircuitOpen)
		}
		cb.halfOpenRequests++
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	// Record the result
	cb.recordResult(err)

	return err
}

// State returns the current state of the circuit
func (cb *circuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkStateTransition()
	return cb.state
}

// Reset manually resets the circuit to closed state
func (cb *circuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
	cb.lastStateChange = time.Now()
}

// checkStateTransition checks if the circuit should transition states
func (cb *circuitBreaker) checkStateTransition() {
	now := time.Now()

	switch cb.state {
	case CircuitOpen:
		// Check if timeout has elapsed
		if now.Sub(cb.lastFailureTime) >= cb.config.Timeout {
			cb.transitionTo(CircuitHalfOpen)
		}
	}
}

// recordResult records the result of a function execution
func (cb *circuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess handles successful executions
func (cb *circuitBreaker) onSuccess() {
	switch cb.state {
	case CircuitClosed:
		// Reset failure count on success
		cb.failures = 0

	case CircuitHalfOpen:
		cb.successes++
		// Check if we've reached success threshold
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}
}

// onFailure handles failed executions
func (cb *circuitBreaker) onFailure() {
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failures++
		// Check if we've reached failure threshold
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitOpen)
		}

	case CircuitHalfOpen:
		// Any failure in half-open goes back to open
		cb.transitionTo(CircuitOpen)
	}
}

// transitionTo transitions the circuit to a new state
func (cb *circuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	cb.state = newState
	cb.lastStateChange = time.Now()

	// Reset counters on state change
	switch newState {
	case CircuitClosed:
		cb.failures = 0
		cb.successes = 0
		cb.halfOpenRequests = 0

	case CircuitHalfOpen:
		cb.successes = 0
		cb.halfOpenRequests = 0

	case CircuitOpen:
		cb.failures = 0
		cb.successes = 0
		cb.halfOpenRequests = 0
	}
}

// perEndpointCircuitBreaker manages individual circuit breakers for each endpoint.
// This allows fine-grained control where issues with one endpoint don't affect others.
//
// For example, if the /v1/cache/:key endpoint is failing, it won't prevent
// requests to the /health endpoint from succeeding.
type perEndpointCircuitBreaker struct {
	mu       sync.RWMutex
	breakers map[string]CircuitBreaker
	config   CircuitBreakerConfig
}

// NewPerEndpointCircuitBreaker creates a manager for per-endpoint circuit breakers.
// Each endpoint gets its own circuit breaker with the same configuration.
//
// This is useful when different endpoints have different reliability
// characteristics or when you want to isolate failures.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithCircuitBreaker(sdk.DefaultCircuitBreakerConfig()).
//	    WithPerEndpointCircuitBreaker()
//
//	client, _ := sdk.NewClient(config)
//	// Now each endpoint has its own circuit breaker
func NewPerEndpointCircuitBreaker(config CircuitBreakerConfig) *perEndpointCircuitBreaker {
	return &perEndpointCircuitBreaker{
		breakers: make(map[string]CircuitBreaker),
		config:   config,
	}
}

// Execute runs a function for a specific endpoint using its circuit breaker.
// If no circuit breaker exists for the endpoint, one is created.
//
// Example:
//
//	err := pecb.Execute("/v1/cache/key123", func() error {
//	    return makeRequest("/v1/cache/key123")
//	})
func (pecb *perEndpointCircuitBreaker) Execute(endpoint string, fn func() error) error {
	cb := pecb.getOrCreate(endpoint)
	return cb.Execute(fn)
}

// State returns the state of a specific endpoint's circuit breaker.
// Returns CircuitClosed if no circuit breaker exists for the endpoint.
//
// Example:
//
//	state := pecb.State("/v1/cache/key123")
//	if state == sdk.CircuitOpen {
//	    log.Printf("Endpoint /v1/cache/key123 circuit is open")
//	}
func (pecb *perEndpointCircuitBreaker) State(endpoint string) CircuitState {
	pecb.mu.RLock()
	cb, exists := pecb.breakers[endpoint]
	pecb.mu.RUnlock()

	if !exists {
		return CircuitClosed
	}

	return cb.State()
}

// Reset resets a specific endpoint's circuit breaker to closed state.
// If no circuit breaker exists for the endpoint, this is a no-op.
//
// Example:
//
//	// After fixing an issue with a specific endpoint
//	pecb.Reset("/v1/cache/problematic-key")
func (pecb *perEndpointCircuitBreaker) Reset(endpoint string) {
	pecb.mu.RLock()
	cb, exists := pecb.breakers[endpoint]
	pecb.mu.RUnlock()

	if exists {
		cb.Reset()
	}
}

// ResetAll resets all circuit breakers to closed state.
// This is useful after resolving a system-wide issue.
//
// Example:
//
//	// After fixing a system-wide issue
//	pecb.ResetAll()
//	log.Println("All circuits reset")
func (pecb *perEndpointCircuitBreaker) ResetAll() {
	pecb.mu.RLock()
	defer pecb.mu.RUnlock()

	for _, cb := range pecb.breakers {
		cb.Reset()
	}
}

// getOrCreate gets or creates a circuit breaker for an endpoint
func (pecb *perEndpointCircuitBreaker) getOrCreate(endpoint string) CircuitBreaker {
	pecb.mu.RLock()
	cb, exists := pecb.breakers[endpoint]
	pecb.mu.RUnlock()

	if exists {
		return cb
	}

	pecb.mu.Lock()
	defer pecb.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := pecb.breakers[endpoint]; exists {
		return cb
	}

	// Create new circuit breaker
	cb = NewCircuitBreaker(pecb.config)
	pecb.breakers[endpoint] = cb
	return cb
}

// noopCircuitBreaker is a circuit breaker that does nothing
type noopCircuitBreaker struct{}

// Execute always executes the function
func (ncb *noopCircuitBreaker) Execute(fn func() error) error {
	return fn()
}

// State always returns closed
func (ncb *noopCircuitBreaker) State() CircuitState {
	return CircuitClosed
}

// Reset does nothing
func (ncb *noopCircuitBreaker) Reset() {}

// NewNoopCircuitBreaker creates a circuit breaker that does nothing.
// This is useful for testing or when you want to disable circuit breaking
// without changing code structure.
//
// The no-op circuit breaker:
//   - Always executes functions (never blocks)
//   - Always reports closed state
//   - Ignores reset calls
//
// Example:
//
//	// For testing without circuit breaker interference
//	cb := sdk.NewNoopCircuitBreaker()
//	err := cb.Execute(func() error {
//	    return someOperation() // Always executes
//	})
func NewNoopCircuitBreaker() CircuitBreaker {
	return &noopCircuitBreaker{}
}

var (
	// ErrCircuitBreakerNotConfigured is returned when circuit breaker is used but not configured
	ErrCircuitBreakerNotConfigured = errors.New("circuit breaker not configured")
)
