package sdk

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryStrategies_ExponentialBackoff(t *testing.T) {
	t.Run("basic exponential progression", func(t *testing.T) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          0,
			Budget:          DefaultRetryBudget(),
		}

		// Test interval calculations
		expectedIntervals := []time.Duration{
			10 * time.Millisecond,  // 10ms
			20 * time.Millisecond,  // 20ms
			40 * time.Millisecond,  // 40ms
			80 * time.Millisecond,  // 80ms
			100 * time.Millisecond, // 100ms (capped)
			100 * time.Millisecond, // 100ms (capped)
		}

		for i, expected := range expectedIntervals {
			actual := strategy.NextInterval(i + 1)
			assert.Equal(t, expected, actual, "Interval for attempt %d", i+1)
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Second,
			Multiplier:      2.0,
			Jitter:          0.5,
			Budget:          DefaultRetryBudget(),
		}

		// Test that jitter produces different values
		attempts := 10
		intervals := make([]time.Duration, attempts)
		for i := 0; i < attempts; i++ {
			intervals[i] = strategy.NextInterval(1)
		}

		// Check that at least some intervals are different (due to jitter)
		allSame := true
		for i := 1; i < len(intervals); i++ {
			if intervals[i] != intervals[0] {
				allSame = false
				break
			}
		}
		assert.False(t, allSame, "Jitter should produce different intervals")

		// Check bounds with jitter
		base := 100 * time.Millisecond
		minExpected := time.Duration(float64(base) * 0.5)
		maxExpected := time.Duration(float64(base) * 1.5)

		for _, interval := range intervals {
			assert.GreaterOrEqual(t, interval, minExpected, "Interval should be >= min bound")
			assert.LessOrEqual(t, interval, maxExpected, "Interval should be <= max bound")
		}
	})

	t.Run("different multipliers", func(t *testing.T) {
		multipliers := []float64{1.5, 2.0, 3.0}

		for _, multiplier := range multipliers {
			t.Run(fmt.Sprintf("multiplier_%.1f", multiplier), func(t *testing.T) {
				strategy := &ExponentialBackoffStrategy{
					InitialInterval: 10 * time.Millisecond,
					MaxInterval:     1 * time.Second,
					Multiplier:      multiplier,
					Jitter:          0,
					Budget:          DefaultRetryBudget(),
				}

				// Test first few intervals
				interval1 := strategy.NextInterval(1)
				interval2 := strategy.NextInterval(2)
				interval3 := strategy.NextInterval(3)

				assert.Equal(t, 10*time.Millisecond, interval1)
				assert.Equal(t, time.Duration(float64(10*time.Millisecond)*multiplier), interval2)
				assert.Equal(t, time.Duration(float64(10*time.Millisecond)*multiplier*multiplier), interval3)
			})
		}
	})

	t.Run("should retry", func(t *testing.T) {
		strategy := DefaultExponentialBackoff()

		testCases := []struct {
			name        string
			err         error
			shouldRetry bool
		}{
			{
				name:        "network error",
				err:         NewError(ErrorTypeNetwork, "network error", nil),
				shouldRetry: true,
			},
			{
				name:        "timeout error",
				err:         NewError(ErrorTypeTimeout, "timeout", nil),
				shouldRetry: true,
			},
			{
				name:        "validation error",
				err:         NewError(ErrorTypeValidation, "invalid input", nil),
				shouldRetry: false,
			},
			{
				name:        "rate limit error",
				err:         NewError(ErrorTypeRateLimit, "rate limited", nil),
				shouldRetry: true,
			},
			{
				name:        "generic error",
				err:         errors.New("generic error"),
				shouldRetry: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.shouldRetry, strategy.ShouldRetry(tc.err, 1))
			})
		}
	})
}

func TestRetryStrategies_LinearBackoff(t *testing.T) {
	t.Run("constant intervals", func(t *testing.T) {
		strategy := &LinearBackoffStrategy{
			Interval: 50 * time.Millisecond,
			Jitter:   0,
			Budget:   DefaultRetryBudget(),
		}

		// All intervals should be the same
		for i := 1; i <= 10; i++ {
			interval := strategy.NextInterval(i)
			assert.Equal(t, 50*time.Millisecond, interval, "Interval for attempt %d", i)
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		strategy := &LinearBackoffStrategy{
			Interval: 100 * time.Millisecond,
			Jitter:   0.3,
			Budget:   DefaultRetryBudget(),
		}

		// Collect multiple intervals
		intervals := make([]time.Duration, 10)
		for i := 0; i < 10; i++ {
			intervals[i] = strategy.NextInterval(1)
		}

		// Check bounds
		base := 100 * time.Millisecond
		minExpected := time.Duration(float64(base) * 0.7)
		maxExpected := time.Duration(float64(base) * 1.3)

		for _, interval := range intervals {
			assert.GreaterOrEqual(t, interval, minExpected)
			assert.LessOrEqual(t, interval, maxExpected)
		}
	})
}

func TestRetryBudget(t *testing.T) {
	t.Run("max attempts", func(t *testing.T) {
		budget := RetryBudget{
			MaxAttempts: 3,
			MaxDuration: 1 * time.Minute,
		}

		assert.False(t, budget.IsExhausted(1, 0))
		assert.False(t, budget.IsExhausted(2, 0))
		assert.True(t, budget.IsExhausted(3, 0))
		assert.True(t, budget.IsExhausted(4, 0))
	})

	t.Run("max duration", func(t *testing.T) {
		budget := RetryBudget{
			MaxAttempts: 10,
			MaxDuration: 100 * time.Millisecond,
		}

		assert.False(t, budget.IsExhausted(1, 50*time.Millisecond))
		assert.False(t, budget.IsExhausted(2, 99*time.Millisecond))
		assert.True(t, budget.IsExhausted(3, 100*time.Millisecond))
		assert.True(t, budget.IsExhausted(4, 101*time.Millisecond))
	})

	t.Run("both limits", func(t *testing.T) {
		budget := RetryBudget{
			MaxAttempts: 3,
			MaxDuration: 100 * time.Millisecond,
		}

		// Should stop at whichever limit is hit first
		assert.True(t, budget.IsExhausted(3, 50*time.Millisecond), "Max attempts reached")
		assert.True(t, budget.IsExhausted(2, 100*time.Millisecond), "Max duration reached")
	})
}

func TestRetryExecutor_Detailed(t *testing.T) {
	t.Run("successful on first attempt", func(t *testing.T) {
		attempts := 0
		strategy := DefaultExponentialBackoff()
		executor := newRetryExecutor(strategy)

		err := executor.Execute(context.Background(), func() error {
			attempts++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("successful after retries", func(t *testing.T) {
		attempts := 0
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 1 * time.Millisecond,
			MaxInterval:     10 * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          0,
			Budget: RetryBudget{
				MaxAttempts: 5,
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

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("exhausts retry budget", func(t *testing.T) {
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
			return NewError(ErrorTypeNetwork, "network error", nil)
		})

		assert.Error(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		strategy := DefaultExponentialBackoff()
		executor := newRetryExecutor(strategy)

		err := executor.Execute(context.Background(), func() error {
			attempts++
			return NewError(ErrorTypeValidation, "validation error", nil)
		})

		assert.Error(t, err)
		assert.Equal(t, 1, attempts, "Should not retry on non-retryable error")
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      1.0,
			Jitter:          0,
			Budget:          DefaultRetryBudget(),
		}

		executor := newRetryExecutor(strategy)

		// Cancel context after first attempt
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := executor.Execute(ctx, func() error {
			attempts++
			return NewError(ErrorTypeNetwork, "network error", nil)
		})

		assert.Error(t, err)
		assert.Equal(t, 1, attempts, "Should stop retrying when context is cancelled")

		var enhancedErr *Error
		if assert.ErrorAs(t, err, &enhancedErr) {
			assert.Equal(t, ErrorTypeTimeout, enhancedErr.Type)
		}
	})

	t.Run("concurrent retries", func(t *testing.T) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 1 * time.Millisecond,
			MaxInterval:     10 * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          0.1,
			Budget: RetryBudget{
				MaxAttempts: 3,
				MaxDuration: 1 * time.Second,
			},
		}

		executor := newRetryExecutor(strategy)

		// Run multiple retry operations concurrently
		numGoroutines := 10
		errors := make(chan error, numGoroutines)
		attemptCounts := make([]int32, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				var localAttempts int32
				err := executor.Execute(context.Background(), func() error {
					atomic.AddInt32(&localAttempts, 1)
					if atomic.LoadInt32(&localAttempts) < 3 {
						return NewError(ErrorTypeNetwork, fmt.Sprintf("error from goroutine %d", id), nil)
					}
					return nil
				})
				atomic.StoreInt32(&attemptCounts[id], localAttempts)
				errors <- err
			}(i)
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			err := <-errors
			assert.NoError(t, err, "Goroutine %d should succeed", i)
		}

		// Verify all goroutines made the expected number of attempts
		for i, count := range attemptCounts {
			assert.Equal(t, int32(3), atomic.LoadInt32(&count), "Goroutine %d should make 3 attempts", i)
		}
	})
}

func TestRetryExecutor_WithCircuitBreaker(t *testing.T) {
	t.Run("circuit breaker prevents retries when open", func(t *testing.T) {
		// Create a circuit breaker that opens after 1 failure
		cbConfig := CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
			HalfOpenRequests: 1,
		}
		cb := NewCircuitBreaker(cbConfig)

		// Open the circuit
		cb.Execute(func() error {
			return errors.New("fail")
		})

		// Now create retry executor
		strategy := DefaultExponentialBackoff()
		executor := newRetryExecutor(strategy)

		attempts := 0
		err := executor.Execute(context.Background(), func() error {
			return cb.Execute(func() error {
				attempts++
				return NewError(ErrorTypeNetwork, "network error", nil)
			})
		})

		assert.Error(t, err)
		assert.Equal(t, 0, attempts, "Circuit breaker should prevent execution")

		var enhancedErr *Error
		if assert.ErrorAs(t, err, &enhancedErr) {
			assert.Equal(t, ErrorTypeCircuitOpen, enhancedErr.Type)
		}
	})
}

func TestHedgedExecutor_Detailed(t *testing.T) {
	t.Run("first request succeeds", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 3,
			Delay:       10 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		attempts := atomic.Int32{}
		err := executor.Execute(context.Background(), func() error {
			attempts.Add(1)
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, int32(1), attempts.Load(), "Should only make one request")
	})

	t.Run("hedged requests succeed", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 3,
			Delay:       5 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		attempts := atomic.Int32{}
		start := time.Now()

		err := executor.Execute(context.Background(), func() error {
			attemptNum := attempts.Add(1)
			// First two requests fail, third succeeds
			if attemptNum < 3 {
				return errors.New("test error")
			}
			// Third request succeeds after a delay
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Equal(t, int32(3), attempts.Load())
		assert.GreaterOrEqual(t, duration, 10*time.Millisecond, "Should wait for successful request")
	})

	t.Run("all requests fail", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 2,
			Delay:       5 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		attempts := atomic.Int32{}
		err := executor.Execute(context.Background(), func() error {
			attempts.Add(1)
			return errors.New("test error")
		})

		assert.Error(t, err)
		assert.Equal(t, int32(2), attempts.Load())
	})

	t.Run("context cancellation", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 3,
			Delay:       10 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		attempts := atomic.Int32{}
		err := executor.Execute(ctx, func() error {
			attempts.Add(1)
			time.Sleep(20 * time.Millisecond)
			return nil
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
		// Should start at least one request
		assert.GreaterOrEqual(t, attempts.Load(), int32(1))
	})

	t.Run("race condition handling", func(t *testing.T) {
		config := HedgedRequest{
			MaxRequests: 5,
			Delay:       1 * time.Millisecond,
		}
		executor := newHedgedExecutor(config)

		// Track which request finishes first
		winner := atomic.Int32{}
		attempts := atomic.Int32{}

		err := executor.Execute(context.Background(), func() error {
			attemptNum := attempts.Add(1)
			// Simulate varying latencies
			switch attemptNum {
			case 1:
				time.Sleep(50 * time.Millisecond)
			case 2:
				time.Sleep(30 * time.Millisecond)
			case 3:
				time.Sleep(10 * time.Millisecond) // This should win
				winner.Store(attemptNum)
				return nil
			case 4:
				time.Sleep(40 * time.Millisecond)
			case 5:
				time.Sleep(60 * time.Millisecond)
			}
			return errors.New("failed")
		})

		assert.NoError(t, err)
		assert.Equal(t, int32(3), winner.Load(), "Third request should win")
		// At least 3 requests should have been made
		assert.GreaterOrEqual(t, attempts.Load(), int32(3))
	})
}

// Benchmarks

func BenchmarkRetryStrategies(b *testing.B) {
	b.Run("ExponentialBackoff_NoJitter", func(b *testing.B) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     1 * time.Second,
			Multiplier:      2.0,
			Jitter:          0,
			Budget:          DefaultRetryBudget(),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			strategy.NextInterval(i%10 + 1)
		}
	})

	b.Run("ExponentialBackoff_WithJitter", func(b *testing.B) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     1 * time.Second,
			Multiplier:      2.0,
			Jitter:          0.5,
			Budget:          DefaultRetryBudget(),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			strategy.NextInterval(i%10 + 1)
		}
	})

	b.Run("LinearBackoff", func(b *testing.B) {
		strategy := &LinearBackoffStrategy{
			Interval: 50 * time.Millisecond,
			Jitter:   0.3,
			Budget:   DefaultRetryBudget(),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			strategy.NextInterval(i%10 + 1)
		}
	})
}

func BenchmarkRetryExecutor(b *testing.B) {
	b.Run("SuccessfulOperation", func(b *testing.B) {
		strategy := DefaultExponentialBackoff()
		executor := newRetryExecutor(strategy)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			executor.Execute(ctx, func() error {
				return nil
			})
		}
	})

	b.Run("WithRetries", func(b *testing.B) {
		strategy := &ExponentialBackoffStrategy{
			InitialInterval: 1 * time.Microsecond,
			MaxInterval:     10 * time.Microsecond,
			Multiplier:      2.0,
			Jitter:          0,
			Budget: RetryBudget{
				MaxAttempts: 3,
				MaxDuration: 1 * time.Second,
			},
		}
		executor := newRetryExecutor(strategy)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			attempts := 0
			executor.Execute(ctx, func() error {
				attempts++
				if attempts < 3 {
					return NewError(ErrorTypeNetwork, "network error", nil)
				}
				return nil
			})
		}
	})
}
