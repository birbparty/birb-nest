package sdk

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsCollector(t *testing.T) {
	t.Run("basic metrics collection", func(t *testing.T) {
		collector := NewMetricsCollector()

		// Record some operations
		collector.OnRequestStart("GET", "/test")
		collector.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)

		collector.OnRequestStart("POST", "/test")
		collector.OnRequestEnd("POST", "/test", 50*time.Millisecond, nil)

		collector.OnRequestStart("GET", "/test")
		collector.OnRequestEnd("GET", "/test", 200*time.Millisecond, errors.New("error"))

		metrics := collector.GetMetrics()

		// Check request counts
		requests := metrics["requests"].(map[string]int64)
		assert.Equal(t, int64(2), requests["GET /test"])
		assert.Equal(t, int64(1), requests["POST /test"])

		// Check error counts
		errors := metrics["errors"].(map[string]int64)
		assert.Equal(t, int64(1), errors["GET /test"])
		assert.Equal(t, int64(0), errors["POST /test"])

		// Check latencies
		latencies := metrics["latencies"].(map[string][]time.Duration)
		assert.Len(t, latencies["GET /test"], 2)
		assert.Len(t, latencies["POST /test"], 1)
	})

	t.Run("retry metrics", func(t *testing.T) {
		collector := NewMetricsCollector()

		collector.OnRetryAttempt("GET", "/retry", 1, 10*time.Millisecond, nil)
		collector.OnRetryAttempt("GET", "/retry", 2, 20*time.Millisecond, errors.New("retry error"))
		collector.OnRetryAttempt("GET", "/retry", 3, 40*time.Millisecond, nil)

		metrics := collector.GetMetrics()
		retries := metrics["retries"].(map[string]int64)
		assert.Equal(t, int64(3), retries["GET /retry"])
	})

	t.Run("circuit breaker metrics", func(t *testing.T) {
		collector := NewMetricsCollector()

		collector.OnCircuitBreakerStateChange("/endpoint1", CircuitClosed, CircuitOpen)
		collector.OnCircuitBreakerStateChange("/endpoint1", CircuitOpen, CircuitHalfOpen)
		collector.OnCircuitBreakerStateChange("/endpoint1", CircuitHalfOpen, CircuitClosed)
		collector.OnCircuitBreakerStateChange("/endpoint2", CircuitClosed, CircuitOpen)

		metrics := collector.GetMetrics()
		cbChanges := metrics["circuit_breaker_state_changes"].(map[string]int64)
		assert.Equal(t, int64(3), cbChanges["/endpoint1"])
		assert.Equal(t, int64(1), cbChanges["/endpoint2"])
	})

	t.Run("cache metrics", func(t *testing.T) {
		collector := NewMetricsCollector()

		collector.OnCacheHit("key1")
		collector.OnCacheHit("key2")
		collector.OnCacheHit("key1")
		collector.OnCacheMiss("key3")
		collector.OnCacheMiss("key4")

		metrics := collector.GetMetrics()
		assert.Equal(t, int64(3), metrics["cache_hits"])
		assert.Equal(t, int64(2), metrics["cache_misses"])

		// Check hit rate
		hitRate := float64(3) / float64(5)
		assert.InDelta(t, hitRate, metrics["cache_hit_rate"], 0.001)
	})

	t.Run("concurrent metrics collection", func(t *testing.T) {
		collector := NewMetricsCollector()
		numGoroutines := 100
		numOperations := 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					path := "/test"
					collector.OnRequestStart("GET", path)
					time.Sleep(time.Microsecond) // Simulate some work
					collector.OnRequestEnd("GET", path, time.Duration(j)*time.Microsecond, nil)
				}
			}(i)
		}

		wg.Wait()

		metrics := collector.GetMetrics()
		requests := metrics["requests"].(map[string]int64)
		expectedRequests := int64(numGoroutines * numOperations)
		assert.Equal(t, expectedRequests, requests["GET /test"])
	})

	t.Run("metric calculation accuracy", func(t *testing.T) {
		collector := NewMetricsCollector()

		// Add specific latencies
		latencies := []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			300 * time.Millisecond,
			400 * time.Millisecond,
			500 * time.Millisecond,
		}

		for _, latency := range latencies {
			collector.OnRequestStart("GET", "/test")
			collector.OnRequestEnd("GET", "/test", latency, nil)
		}

		metrics := collector.GetMetrics()
		latencyMetrics := metrics["latencies"].(map[string][]time.Duration)
		recordedLatencies := latencyMetrics["GET /test"]

		assert.Len(t, recordedLatencies, 5)

		// Verify all latencies were recorded
		for i, expected := range latencies {
			assert.Equal(t, expected, recordedLatencies[i])
		}
	})
}

func TestCompositeObserver(t *testing.T) {
	t.Run("multiple observers called", func(t *testing.T) {
		var called1, called2, called3 atomic.Int32

		obs1 := &mockObserver{
			onRequestStart: func(method, path string) {
				called1.Add(1)
			},
		}
		obs2 := &mockObserver{
			onRequestStart: func(method, path string) {
				called2.Add(1)
			},
		}
		obs3 := &mockObserver{
			onRequestStart: func(method, path string) {
				called3.Add(1)
			},
		}

		composite := NewCompositeObserver(obs1, obs2, obs3)
		composite.OnRequestStart("GET", "/test")

		assert.Equal(t, int32(1), called1.Load())
		assert.Equal(t, int32(1), called2.Load())
		assert.Equal(t, int32(1), called3.Load())
	})

	t.Run("all observer methods called", func(t *testing.T) {
		calls := make(map[string]int)
		var mu sync.Mutex

		observer := &mockObserver{
			onRequestStart: func(method, path string) {
				mu.Lock()
				calls["onRequestStart"]++
				mu.Unlock()
			},
			onRequestEnd: func(method, path string, duration time.Duration, err error) {
				mu.Lock()
				calls["onRequestEnd"]++
				mu.Unlock()
			},
			onRetryAttempt: func(method, path string, attempt int, delay time.Duration, err error) {
				mu.Lock()
				calls["onRetryAttempt"]++
				mu.Unlock()
			},
			onCircuitBreakerStateChange: func(endpoint string, oldState, newState CircuitState) {
				mu.Lock()
				calls["onCircuitBreakerStateChange"]++
				mu.Unlock()
			},
			onCacheHit: func(key string) {
				mu.Lock()
				calls["onCacheHit"]++
				mu.Unlock()
			},
			onCacheMiss: func(key string) {
				mu.Lock()
				calls["onCacheMiss"]++
				mu.Unlock()
			},
		}

		composite := NewCompositeObserver(observer)

		// Call all methods
		composite.OnRequestStart("GET", "/test")
		composite.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)
		composite.OnRetryAttempt("POST", "/retry", 1, 10*time.Millisecond, nil)
		composite.OnCircuitBreakerStateChange("/endpoint", CircuitClosed, CircuitOpen)
		composite.OnCacheHit("key1")
		composite.OnCacheMiss("key2")

		// Verify all methods were called
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, calls["onRequestStart"])
		assert.Equal(t, 1, calls["onRequestEnd"])
		assert.Equal(t, 1, calls["onRetryAttempt"])
		assert.Equal(t, 1, calls["onCircuitBreakerStateChange"])
		assert.Equal(t, 1, calls["onCacheHit"])
		assert.Equal(t, 1, calls["onCacheMiss"])
	})

	t.Run("observer error handling", func(t *testing.T) {
		// One observer that panics
		panicObserver := &mockObserver{
			onRequestStart: func(method, path string) {
				panic("observer panic")
			},
		}

		// Another observer that should still be called
		called := false
		normalObserver := &mockObserver{
			onRequestStart: func(method, path string) {
				called = true
			},
		}

		composite := NewCompositeObserver(panicObserver, normalObserver)

		// Should not panic and should call the second observer
		assert.NotPanics(t, func() {
			composite.OnRequestStart("GET", "/test")
		})

		assert.True(t, called, "Normal observer should have been called despite panic in first observer")
	})

	t.Run("empty composite observer", func(t *testing.T) {
		composite := NewCompositeObserver()

		// Should not panic with no observers
		assert.NotPanics(t, func() {
			composite.OnRequestStart("GET", "/test")
			composite.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)
			composite.OnRetryAttempt("POST", "/retry", 1, 10*time.Millisecond, nil)
			composite.OnCircuitBreakerStateChange("/endpoint", CircuitClosed, CircuitOpen)
			composite.OnCacheHit("key")
			composite.OnCacheMiss("key")
		})
	})
}

func TestObserverIntegration(t *testing.T) {
	t.Run("observer with client operations", func(t *testing.T) {
		// Create a mock server
		server := mockServer()
		defer server.Close()

		// Create metrics collector
		collector := NewMetricsCollector()

		// Create client with observer
		config := DefaultConfig().
			WithBaseURL(server.URL).
			WithObserver(collector)

		client, err := NewClient(config)
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()

		// Perform operations
		err = client.Set(ctx, "key1", "value1")
		assert.NoError(t, err)

		var value string
		err = client.Get(ctx, "test-key", &value)
		assert.NoError(t, err)

		err = client.Get(ctx, "non-existent", &value)
		assert.Error(t, err)

		// Check metrics
		metrics := collector.GetMetrics()
		requests := metrics["requests"].(map[string]int64)
		errors := metrics["errors"].(map[string]int64)

		assert.Equal(t, int64(1), requests["POST /v1/cache/key1"])
		assert.Equal(t, int64(1), requests["GET /v1/cache/test-key"])
		assert.Equal(t, int64(1), requests["GET /v1/cache/non-existent"])
		assert.Equal(t, int64(1), errors["GET /v1/cache/non-existent"])
	})

	t.Run("observer with retry operations", func(t *testing.T) {
		collector := NewMetricsCollector()

		// Create config with observer
		_ = DefaultConfig().
			WithRetries(3).
			WithObserver(collector)

		// Test retry with observer
		strategy := DefaultExponentialBackoff()
		executor := newRetryExecutor(strategy)

		attemptCount := 0
		err := executor.Execute(context.Background(), func() error {
			attemptCount++
			if attemptCount < 3 {
				collector.OnRetryAttempt("GET", "/test", attemptCount, 10*time.Millisecond, errors.New("retry error"))
				return NewError(ErrorTypeNetwork, "network error", nil)
			}
			return nil
		})

		assert.NoError(t, err)

		metrics := collector.GetMetrics()
		retries := metrics["retries"].(map[string]int64)
		assert.Equal(t, int64(2), retries["GET /test"]) // 2 retries before success
	})
}

// mockObserver is a test observer implementation specific to observer tests
type mockObserver struct {
	onRequestStart              func(method, path string)
	onRequestEnd                func(method, path string, duration time.Duration, err error)
	onRetryAttempt              func(method, path string, attempt int, delay time.Duration, err error)
	onCircuitBreakerStateChange func(endpoint string, oldState, newState CircuitState)
	onCacheHit                  func(key string)
	onCacheMiss                 func(key string)
}

func (m *mockObserver) OnRequestStart(method, path string) {
	if m.onRequestStart != nil {
		m.onRequestStart(method, path)
	}
}

func (m *mockObserver) OnRequestEnd(method, path string, duration time.Duration, err error) {
	if m.onRequestEnd != nil {
		m.onRequestEnd(method, path, duration, err)
	}
}

func (m *mockObserver) OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error) {
	if m.onRetryAttempt != nil {
		m.onRetryAttempt(method, path, attempt, delay, err)
	}
}

func (m *mockObserver) OnCircuitBreakerStateChange(endpoint string, oldState, newState CircuitState) {
	if m.onCircuitBreakerStateChange != nil {
		m.onCircuitBreakerStateChange(endpoint, oldState, newState)
	}
}

func (m *mockObserver) OnCacheHit(key string) {
	if m.onCacheHit != nil {
		m.onCacheHit(key)
	}
}

func (m *mockObserver) OnCacheMiss(key string) {
	if m.onCacheMiss != nil {
		m.onCacheMiss(key)
	}
}

// Mock transport for testing
type mockTransport struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.roundTrip != nil {
		return m.roundTrip(req)
	}
	return nil, errors.New("not implemented")
}

// Benchmarks

func BenchmarkMetricsCollector(b *testing.B) {
	b.Run("OnRequestStart", func(b *testing.B) {
		collector := NewMetricsCollector()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			collector.OnRequestStart("GET", "/test")
		}
	})

	b.Run("OnRequestEnd", func(b *testing.B) {
		collector := NewMetricsCollector()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			collector.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		collector := NewMetricsCollector()
		// Pre-populate with some data
		for i := 0; i < 1000; i++ {
			collector.OnRequestStart("GET", "/test")
			collector.OnRequestEnd("GET", "/test", time.Duration(i)*time.Millisecond, nil)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = collector.GetMetrics()
		}
	})

	b.Run("ConcurrentOperations", func(b *testing.B) {
		collector := NewMetricsCollector()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%2 == 0 {
					collector.OnRequestStart("GET", "/test")
				} else {
					collector.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)
				}
				i++
			}
		})
	})
}

func BenchmarkCompositeObserver(b *testing.B) {
	// Create multiple observers
	observers := make([]Observer, 5)
	for i := range observers {
		observers[i] = &mockObserver{
			onRequestStart: func(method, path string) {
				// Minimal work
				_ = method + path
			},
		}
	}

	composite := NewCompositeObserver(observers...)

	b.Run("OnRequestStart", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			composite.OnRequestStart("GET", "/test")
		}
	})

	b.Run("AllMethods", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			composite.OnRequestStart("GET", "/test")
			composite.OnRequestEnd("GET", "/test", 100*time.Millisecond, nil)
			composite.OnRetryAttempt("POST", "/retry", 1, 10*time.Millisecond, nil)
			composite.OnCircuitBreakerStateChange("/endpoint", CircuitClosed, CircuitOpen)
			composite.OnCacheHit("key")
			composite.OnCacheMiss("key")
		}
	})
}
