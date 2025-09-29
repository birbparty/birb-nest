package sdk

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("circuit opens after failure threshold", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
			HalfOpenRequests: 1,
		}
		cb := NewCircuitBreaker(config)

		// Fail 3 times to open circuit
		for i := 0; i < 3; i++ {
			err := cb.Execute(func() error {
				return errors.New("test error")
			})
			if err == nil {
				t.Errorf("Expected error, got nil")
			}
		}

		// Circuit should be open now
		if cb.State() != CircuitOpen {
			t.Errorf("Expected circuit to be open, got %v", cb.State())
		}

		// Further requests should fail immediately
		err := cb.Execute(func() error {
			t.Error("Function should not be executed when circuit is open")
			return nil
		})
		if err == nil {
			t.Error("Expected circuit open error")
		}
		var enhancedErr *Error
		if errors.As(err, &enhancedErr) {
			if enhancedErr.Type != ErrorTypeCircuitOpen {
				t.Errorf("Expected ErrorTypeCircuitOpen, got %v", enhancedErr.Type)
			}
		}
	})

	t.Run("circuit transitions to half-open after timeout", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 1,
			Timeout:          50 * time.Millisecond,
			HalfOpenRequests: 2,
		}
		cb := NewCircuitBreaker(config)

		// Open the circuit
		cb.Execute(func() error {
			return errors.New("test error")
		})

		if cb.State() != CircuitOpen {
			t.Errorf("Expected circuit to be open, got %v", cb.State())
		}

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Should be half-open now
		if cb.State() != CircuitHalfOpen {
			t.Errorf("Expected circuit to be half-open, got %v", cb.State())
		}

		// Success should close the circuit
		err := cb.Execute(func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}

		if cb.State() != CircuitClosed {
			t.Errorf("Expected circuit to be closed, got %v", cb.State())
		}
	})

	t.Run("half-open request limit", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 3, // Higher than HalfOpenRequests
			Timeout:          50 * time.Millisecond,
			HalfOpenRequests: 2,
		}
		cb := NewCircuitBreaker(config)

		// Open the circuit
		cb.Execute(func() error {
			return errors.New("test error")
		})

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Execute 2 requests (the limit)
		for i := 0; i < 2; i++ {
			err := cb.Execute(func() error {
				return nil
			})
			if err != nil {
				t.Errorf("Expected success on request %d, got error: %v", i+1, err)
			}
		}

		// Third request should fail because we've hit the half-open limit
		err := cb.Execute(func() error {
			return nil
		})
		if err == nil {
			t.Error("Expected half-open limit error")
		}
		var enhancedErr *Error
		if errors.As(err, &enhancedErr) {
			if enhancedErr.Type != ErrorTypeCircuitOpen {
				t.Errorf("Expected ErrorTypeCircuitOpen, got %v", enhancedErr.Type)
			}
		}
	})
}

func TestRetryStrategies(t *testing.T) {
	t.Run("exponential backoff", func(t *testing.T) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          0,
			Budget:          DefaultRetryBudget(),
		}

		// Test interval calculations
		intervals := []time.Duration{
			strategy.NextInterval(1), // 10ms
			strategy.NextInterval(2), // 20ms
			strategy.NextInterval(3), // 40ms
			strategy.NextInterval(4), // 80ms
			strategy.NextInterval(5), // 100ms (capped)
			strategy.NextInterval(6), // 100ms (capped)
		}

		expected := []time.Duration{
			10 * time.Millisecond,
			20 * time.Millisecond,
			40 * time.Millisecond,
			80 * time.Millisecond,
			100 * time.Millisecond,
			100 * time.Millisecond,
		}

		for i, interval := range intervals {
			if interval != expected[i] {
				t.Errorf("Attempt %d: expected %v, got %v", i+1, expected[i], interval)
			}
		}
	})

	t.Run("exponential backoff with jitter", func(t *testing.T) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Second,
			Multiplier:      2.0,
			Jitter:          0.5,
			Budget:          DefaultRetryBudget(),
		}

		// Test that jitter produces different values
		// Run multiple times to account for randomness
		different := false
		for i := 0; i < 10; i++ {
			interval1 := strategy.NextInterval(1)
			interval2 := strategy.NextInterval(1)
			if interval1 != interval2 {
				different = true
				break
			}
		}
		if !different {
			t.Error("Expected jitter to produce different intervals")
		}

		// Check bounds with jitter
		base := 100 * time.Millisecond
		min := time.Duration(float64(base) * 0.5)
		max := time.Duration(float64(base) * 1.5)

		for i := 0; i < 10; i++ {
			interval := strategy.NextInterval(1)
			if interval < min || interval > max {
				t.Errorf("Interval %v outside expected range [%v, %v]", interval, min, max)
			}
		}
	})

	t.Run("linear backoff", func(t *testing.T) {
		strategy := &LinearBackoffStrategy{
			Interval: 50 * time.Millisecond,
			Jitter:   0,
			Budget:   DefaultRetryBudget(),
		}

		// All intervals should be the same
		for i := 1; i <= 5; i++ {
			interval := strategy.NextInterval(i)
			if interval != 50*time.Millisecond {
				t.Errorf("Expected 50ms, got %v", interval)
			}
		}
	})

	t.Run("retry budget exhaustion", func(t *testing.T) {
		budget := RetryBudget{
			MaxAttempts: 2,
			MaxDuration: 100 * time.Millisecond,
		}

		// Test max attempts
		if !budget.IsExhausted(2, 0) {
			t.Error("Expected budget to be exhausted after max attempts")
		}

		// Test max duration
		if !budget.IsExhausted(1, 100*time.Millisecond) {
			t.Error("Expected budget to be exhausted after max duration")
		}
	})
}

func TestRetryExecutor(t *testing.T) {
	t.Run("successful retry", func(t *testing.T) {
		attempts := 0
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 1 * time.Millisecond,
			MaxInterval:     10 * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          0,
			Budget: RetryBudget{
				MaxAttempts: 3,
				MaxDuration: 1 * time.Second,
			},
		}

		executor := newRetryExecutor(strategy)
		err := executor.Execute(context.Background(), func() error {
			attempts++
			if attempts < 3 {
				return NewError(ErrorTypeNetwork, "network error", nil)
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected success after retries, got error: %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		strategy := DefaultExponentialBackoff()
		executor := newRetryExecutor(strategy)

		err := executor.Execute(context.Background(), func() error {
			attempts++
			return NewError(ErrorTypeValidation, "validation error", nil)
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      1.0,
			Jitter:          0,
			Budget:          DefaultRetryBudget(),
		}

		executor := newRetryExecutor(strategy)

		// Cancel context after 50ms
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := executor.Execute(ctx, func() error {
			return NewError(ErrorTypeNetwork, "network error", nil)
		})

		if err == nil {
			t.Error("Expected context cancellation error")
		}
		var enhancedErr *Error
		if errors.As(err, &enhancedErr) {
			if enhancedErr.Type != ErrorTypeTimeout {
				t.Errorf("Expected ErrorTypeTimeout, got %v", enhancedErr.Type)
			}
		}
	})
}

func TestHedgedExecutor(t *testing.T) {
	t.Run("successful hedged request", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 3,
			Delay:       10 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		attempts := 0
		start := time.Now()

		err := executor.Execute(context.Background(), func() error {
			attempts++
			// First two requests fail quickly, third succeeds
			if attempts < 3 {
				return errors.New("test error")
			}
			// Third request succeeds after delay
			time.Sleep(20 * time.Millisecond)
			return nil
		})

		duration := time.Since(start)

		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}

		// Should have made 3 attempts
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}

		// Should have taken at least 20ms (delay between requests)
		if duration < 20*time.Millisecond {
			t.Errorf("Expected duration >= 20ms, got %v", duration)
		}
	})

	t.Run("all hedged requests fail", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 2,
			Delay:       5 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		err := executor.Execute(context.Background(), func() error {
			return errors.New("test error")
		})

		if err == nil {
			t.Error("Expected error when all requests fail")
		}
	})
}

func TestObserver(t *testing.T) {
	t.Run("metrics collector", func(t *testing.T) {
		collector := NewMetricsCollector()

		// Record some operations
		collector.OnRequestStart("GET", "/test")
		collector.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)
		collector.OnRequestStart("GET", "/test")
		collector.OnRequestEnd("GET", "/test", 200*time.Millisecond, errors.New("error"))

		collector.OnRetryAttempt("POST", "/retry", 1, 50*time.Millisecond, errors.New("retry error"))

		collector.OnCircuitBreakerStateChange("/endpoint", CircuitClosed, CircuitOpen)

		collector.OnCacheHit("key1")
		collector.OnCacheHit("key2")
		collector.OnCacheMiss("key3")

		metrics := collector.GetMetrics()

		// Check request count
		requests := metrics["requests"].(map[string]int64)
		if requests["GET /test"] != 2 {
			t.Errorf("Expected 2 requests, got %d", requests["GET /test"])
		}

		// Check error count
		errors := metrics["errors"].(map[string]int64)
		if errors["GET /test"] != 1 {
			t.Errorf("Expected 1 error, got %d", errors["GET /test"])
		}

		// Check cache metrics
		if metrics["cache_hits"].(int64) != 2 {
			t.Errorf("Expected 2 cache hits, got %d", metrics["cache_hits"])
		}
		if metrics["cache_misses"].(int64) != 1 {
			t.Errorf("Expected 1 cache miss, got %d", metrics["cache_misses"])
		}
	})

	t.Run("composite observer", func(t *testing.T) {
		called1 := false
		called2 := false

		obs1 := &testObserver{onRequestStart: func(method, path string) {
			called1 = true
		}}
		obs2 := &testObserver{onRequestStart: func(method, path string) {
			called2 = true
		}}

		composite := NewCompositeObserver(obs1, obs2)
		composite.OnRequestStart("GET", "/test")

		if !called1 || !called2 {
			t.Error("Expected both observers to be called")
		}
	})
}

// Test observer implementation
type testObserver struct {
	onRequestStart func(method, path string)
}

func (t *testObserver) OnRequestStart(method, path string) {
	if t.onRequestStart != nil {
		t.onRequestStart(method, path)
	}
}

func (t *testObserver) OnRequestEnd(method, path string, duration time.Duration, err error) {}
func (t *testObserver) OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error) {
}
func (t *testObserver) OnCircuitBreakerStateChange(endpoint string, oldState, newState CircuitState) {
}
func (t *testObserver) OnCacheHit(key string)  {}
func (t *testObserver) OnCacheMiss(key string) {}

func TestErrorTypes(t *testing.T) {
	t.Run("enhanced error creation", func(t *testing.T) {
		baseErr := errors.New("base error")
		err := NewError(ErrorTypeNetwork, "network failure", baseErr)

		if err.Type != ErrorTypeNetwork {
			t.Errorf("Expected ErrorTypeNetwork, got %v", err.Type)
		}
		if !err.Retryable {
			t.Error("Expected network error to be retryable")
		}
		if err.wrapped != baseErr {
			t.Error("Expected wrapped error to be preserved")
		}
	})

	t.Run("error context", func(t *testing.T) {
		err := NewError(ErrorTypeTimeout, "request timeout", nil)
		ctx := &ErrorContext{
			URL:        "http://example.com/api",
			Method:     "GET",
			RetryCount: 3,
			Duration:   5 * time.Second,
		}
		err.WithContext(ctx)

		if err.Context.URL != ctx.URL {
			t.Errorf("Expected URL %s, got %s", ctx.URL, err.Context.URL)
		}
		if err.Context.RetryCount != 3 {
			t.Errorf("Expected retry count 3, got %d", err.Context.RetryCount)
		}
	})

	t.Run("error details", func(t *testing.T) {
		err := NewError(ErrorTypeServer, "server error", nil)
		err.WithDetail("status_code", 500)
		err.WithDetail("request_id", "123")

		if err.Details["status_code"] != 500 {
			t.Errorf("Expected status_code 500, got %v", err.Details["status_code"])
		}
		if err.Details["request_id"] != "123" {
			t.Errorf("Expected request_id 123, got %v", err.Details["request_id"])
		}
	})

	t.Run("API error conversion", func(t *testing.T) {
		apiErr := &APIError{
			StatusCode: 429,
			Message:    "Too Many Requests",
			Code:       "RATE_LIMITED",
		}

		enhanced := apiErr.ToError()
		if enhanced.Type != ErrorTypeRateLimit {
			t.Errorf("Expected ErrorTypeRateLimit, got %v", enhanced.Type)
		}
		if !enhanced.Retryable {
			t.Error("Expected rate limit error to be retryable")
		}
		if enhanced.Code != "RATE_LIMITED" {
			t.Errorf("Expected code RATE_LIMITED, got %s", enhanced.Code)
		}
	})
}
