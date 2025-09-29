//go:build integration
// +build integration

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Integration tests that test the full client lifecycle
// Run with: go test -tags=integration ./sdk

type integrationTestServer struct {
	*httptest.Server
	mu      sync.Mutex
	data    map[string]interface{}
	calls   map[string]int
	latency time.Duration
}

func newIntegrationTestServer() *integrationTestServer {
	its := &integrationTestServer{
		data:  make(map[string]interface{}),
		calls: make(map[string]int),
	}

	its.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		its.mu.Lock()
		defer its.mu.Unlock()

		// Simulate latency if configured
		if its.latency > 0 {
			time.Sleep(its.latency)
		}

		// Track API calls
		its.calls[r.Method+" "+r.URL.Path]++

		switch r.Method {
		case http.MethodGet:
			its.handleGet(w, r)
		case http.MethodPost:
			its.handlePost(w, r)
		case http.MethodDelete:
			its.handleDelete(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	return its
}

func (its *integrationTestServer) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		json.NewEncoder(w).Encode(HealthResponse{
			Status:  "healthy",
			Version: "1.0.0",
		})
		return
	}

	key := r.URL.Path[len("/v1/cache/"):]
	value, exists := its.data[key]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Key not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	data, _ := json.Marshal(value)
	resp := CacheResponse{
		Key:       key,
		Value:     json.RawMessage(data),
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	json.NewEncoder(w).Encode(resp)
}

func (its *integrationTestServer) handlePost(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[len("/v1/cache/"):]

	var req CacheRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid request body",
			"code":  "BAD_REQUEST",
		})
		return
	}

	// Store the raw value
	var value interface{}
	if err := json.Unmarshal(req.Value, &value); err != nil {
		value = string(req.Value)
	}
	its.data[key] = value

	resp := CacheResponse{
		Key:       key,
		Value:     req.Value,
		Version:   1,
		TTL:       req.TTL,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (its *integrationTestServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[len("/v1/cache/"):]

	_, exists := its.data[key]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Key not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	delete(its.data, key)
	w.WriteHeader(http.StatusNoContent)
}

func (its *integrationTestServer) getCallCount(method, path string) int {
	its.mu.Lock()
	defer its.mu.Unlock()
	return its.calls[method+" "+path]
}

func (its *integrationTestServer) reset() {
	its.mu.Lock()
	defer its.mu.Unlock()
	its.data = make(map[string]interface{})
	its.calls = make(map[string]int)
	its.latency = 0
}

func TestIntegration_FullClientLifecycle(t *testing.T) {
	server := newIntegrationTestServer()
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test 1: Ping the server
	t.Run("ping", func(t *testing.T) {
		err := client.Ping(ctx)
		if err != nil {
			t.Errorf("Ping() error = %v", err)
		}
	})

	// Test 2: Set and get a simple value
	t.Run("set_and_get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := client.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		var result string
		err = client.Get(ctx, key, &result)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if result != value {
			t.Errorf("Get() = %v, want %v", result, value)
		}
	})

	// Test 3: Complex data types
	t.Run("complex_types", func(t *testing.T) {
		type TestData struct {
			ID       int                    `json:"id"`
			Name     string                 `json:"name"`
			Tags     []string               `json:"tags"`
			Settings map[string]interface{} `json:"settings"`
		}

		original := TestData{
			ID:   123,
			Name: "Test Data",
			Tags: []string{"integration", "test"},
			Settings: map[string]interface{}{
				"enabled": true,
				"limit":   100,
			},
		}

		err := client.Set(ctx, "complex-data", original)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		var retrieved TestData
		err = client.Get(ctx, "complex-data", &retrieved)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved.ID != original.ID || retrieved.Name != original.Name {
			t.Errorf("Retrieved data doesn't match original")
		}
	})

	// Test 4: Delete operation
	t.Run("delete", func(t *testing.T) {
		key := "delete-test"

		// Set a value
		err := client.Set(ctx, key, "will be deleted")
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Delete it
		err = client.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Try to get it (should fail)
		var result string
		err = client.Get(ctx, key, &result)
		if !IsNotFound(err) {
			t.Errorf("Expected not found error, got %v", err)
		}
	})

	// Test 5: Error handling
	t.Run("error_handling", func(t *testing.T) {
		// Get non-existent key
		var result string
		err := client.Get(ctx, "non-existent", &result)
		if !IsNotFound(err) {
			t.Errorf("Expected not found error, got %v", err)
		}

		// Delete non-existent key
		err = client.Delete(ctx, "non-existent")
		if !IsNotFound(err) {
			t.Errorf("Expected not found error, got %v", err)
		}

		// Invalid operations
		err = client.Set(ctx, "", "value")
		if err == nil {
			t.Error("Expected error for empty key")
		}

		err = client.Get(ctx, "test", nil)
		if err == nil {
			t.Error("Expected error for nil destination")
		}
	})
}

func TestIntegration_ExtendedClient(t *testing.T) {
	server := newIntegrationTestServer()
	defer server.Close()

	client, err := NewExtendedClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create extended client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test 1: SetWithOptions
	t.Run("set_with_options", func(t *testing.T) {
		ttl := 5 * time.Minute
		opts := &SetOptions{
			TTL: &ttl,
			Metadata: map[string]interface{}{
				"source": "integration-test",
				"env":    "test",
			},
		}

		err := client.SetWithOptions(ctx, "options-test", "value with options", opts)
		if err != nil {
			t.Errorf("SetWithOptions() error = %v", err)
		}
	})

	// Test 2: Exists
	t.Run("exists", func(t *testing.T) {
		// Set a value
		err := client.Set(ctx, "exists-test", "exists")
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Check it exists
		exists, err := client.Exists(ctx, "exists-test")
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if !exists {
			t.Error("Expected key to exist")
		}

		// Check non-existent key
		exists, err = client.Exists(ctx, "does-not-exist")
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if exists {
			t.Error("Expected key to not exist")
		}
	})

	// Test 3: GetMultiple
	t.Run("get_multiple", func(t *testing.T) {
		// Set multiple values
		keys := []string{"multi1", "multi2", "multi3"}
		for i, key := range keys {
			err := client.Set(ctx, key, fmt.Sprintf("value%d", i+1))
			if err != nil {
				t.Fatalf("Set() error = %v", err)
			}
		}

		// Get all values
		results, err := client.GetMultiple(ctx, keys)
		if err != nil {
			t.Fatalf("GetMultiple() error = %v", err)
		}

		if len(results) != len(keys) {
			t.Errorf("GetMultiple() returned %d results, want %d", len(results), len(keys))
		}

		// Verify values
		for i, key := range keys {
			if val, ok := results[key]; !ok {
				t.Errorf("Missing result for key %s", key)
			} else {
				expected := fmt.Sprintf("value%d", i+1)
				if str, ok := val.(string); !ok || str != expected {
					t.Errorf("GetMultiple()[%s] = %v, want %s", key, val, expected)
				}
			}
		}

		// Test with some non-existent keys
		mixedKeys := append(keys, "non-existent")
		results, err = client.GetMultiple(ctx, mixedKeys)
		if err != nil {
			t.Fatalf("GetMultiple() with mixed keys error = %v", err)
		}

		if len(results) != len(keys) {
			t.Errorf("GetMultiple() with mixed keys returned %d results, want %d", len(results), len(keys))
		}
	})
}

func TestIntegration_ConcurrentOperations(t *testing.T) {
	server := newIntegrationTestServer()
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	concurrency := 10
	operations := 100

	t.Run("concurrent_sets", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, concurrency*operations)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(goroutine int) {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					key := fmt.Sprintf("concurrent-%d-%d", goroutine, j)
					value := fmt.Sprintf("value-%d-%d", goroutine, j)
					if err := client.Set(ctx, key, value); err != nil {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		var errCount int
		for err := range errors {
			t.Errorf("Concurrent set error: %v", err)
			errCount++
		}

		if errCount > 0 {
			t.Errorf("Had %d errors during concurrent sets", errCount)
		}
	})

	t.Run("concurrent_gets", func(t *testing.T) {
		// First set some values
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("get-test-%d", i)
			value := fmt.Sprintf("value-%d", i)
			client.Set(ctx, key, value)
		}

		var wg sync.WaitGroup
		errors := make(chan error, concurrency*operations)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(goroutine int) {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					key := fmt.Sprintf("get-test-%d", j%10)
					var result string
					if err := client.Get(ctx, key, &result); err != nil {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent get error: %v", err)
		}
	})
}

func TestIntegration_Retries(t *testing.T) {
	server := newIntegrationTestServer()
	defer server.Close()

	// Configure client with short retry delays for testing
	config := DefaultConfig().
		WithBaseURL(server.URL).
		WithRetries(3)
	config.RetryConfig.InitialInterval = 10 * time.Millisecond
	config.RetryConfig.MaxInterval = 50 * time.Millisecond

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Add latency to simulate slow server
	server.latency = 5 * time.Millisecond

	t.Run("operations_with_latency", func(t *testing.T) {
		// These should succeed despite latency
		err := client.Set(ctx, "retry-test", "value")
		if err != nil {
			t.Errorf("Set() with latency error = %v", err)
		}

		var result string
		err = client.Get(ctx, "retry-test", &result)
		if err != nil {
			t.Errorf("Get() with latency error = %v", err)
		}
	})
}

func TestIntegration_TypedClient(t *testing.T) {
	server := newIntegrationTestServer()
	defer server.Close()

	baseClient, err := NewExtendedClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer baseClient.Close()

	ctx := context.Background()

	t.Run("string_client", func(t *testing.T) {
		client := NewStringClient(baseClient)

		// Set and get string
		err := client.Set(ctx, "string-key", "hello world")
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		result, err := client.Get(ctx, "string-key")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if result != "hello world" {
			t.Errorf("Get() = %v, want %v", result, "hello world")
		}
	})

	t.Run("int_client", func(t *testing.T) {
		client := NewIntClient(baseClient)

		// Set and get int
		err := client.Set(ctx, "int-key", 12345)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		result, err := client.Get(ctx, "int-key")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if result != 12345 {
			t.Errorf("Get() = %v, want %v", result, 12345)
		}
	})

	t.Run("map_client", func(t *testing.T) {
		client := NewMapClient(baseClient)

		// Set and get map
		data := map[string]interface{}{
			"name":    "test",
			"enabled": true,
			"count":   42,
		}

		err := client.Set(ctx, "map-key", data)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		result, err := client.Get(ctx, "map-key")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if result["name"] != "test" || result["count"] != float64(42) {
			t.Errorf("Get() returned unexpected map: %+v", result)
		}
	})
}

// TestIntegration_StressTest performs a stress test with high load
func TestIntegration_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	server := newIntegrationTestServer()
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	duration := 5 * time.Second
	workers := 50

	t.Run("stress_test", func(t *testing.T) {
		start := time.Now()
		var wg sync.WaitGroup
		var operations int64
		var errors int64

		// Start workers
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				count := 0
				for time.Since(start) < duration {
					key := fmt.Sprintf("stress-%d-%d", worker, count)
					value := fmt.Sprintf("value-%d", count)

					// Alternate between set and get
					if count%2 == 0 {
						if err := client.Set(ctx, key, value); err != nil {
							errors++
						}
					} else {
						var result string
						client.Get(ctx, key, &result)
					}

					count++
					operations++
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		t.Logf("Stress test completed:")
		t.Logf("  Duration: %v", elapsed)
		t.Logf("  Workers: %d", workers)
		t.Logf("  Operations: %d", operations)
		t.Logf("  Operations/sec: %.2f", float64(operations)/elapsed.Seconds())
		t.Logf("  Errors: %d", errors)

		if errors > 0 {
			errorRate := float64(errors) / float64(operations) * 100
			if errorRate > 1.0 {
				t.Errorf("Error rate too high: %.2f%%", errorRate)
			}
		}
	})
}
