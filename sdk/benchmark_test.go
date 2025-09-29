package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Benchmarks for the SDK client

// BenchmarkClient_Operations benchmarks basic client operations
func BenchmarkClient_Operations(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := CacheResponse{
				Key:       "test",
				Value:     json.RawMessage(`"benchmark value"`),
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"key":"test","version":1}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	b.Run("Set", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			client.Set(ctx, "key", "value")
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		var result string
		for i := 0; i < b.N; i++ {
			client.Get(ctx, "key", &result)
		}
	})

	b.Run("Delete", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			client.Delete(ctx, "key")
		}
	})

	b.Run("Ping", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			client.Ping(ctx)
		}
	})
}

// BenchmarkSerialization benchmarks different serialization scenarios
func BenchmarkSerialization(b *testing.B) {
	type ComplexStruct struct {
		ID          int                    `json:"id"`
		Name        string                 `json:"name"`
		Tags        []string               `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
		CreatedAt   time.Time              `json:"created_at"`
		UpdatedAt   time.Time              `json:"updated_at"`
		Nested      *ComplexStruct         `json:"nested,omitempty"`
		BinaryData  []byte                 `json:"binary_data"`
		FloatValues []float64              `json:"float_values"`
	}

	smallStruct := ComplexStruct{
		ID:   1,
		Name: "Test",
	}

	mediumStruct := ComplexStruct{
		ID:   1,
		Name: "Test Object",
		Tags: []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	largeStruct := ComplexStruct{
		ID:   1,
		Name: "Large Test Object with Long Name",
		Tags: make([]string, 100),
		Metadata: map[string]interface{}{
			"config": map[string]interface{}{
				"setting1": "value1",
				"setting2": 42,
				"nested": map[string]interface{}{
					"deep1": "value",
					"deep2": []int{1, 2, 3, 4, 5},
				},
			},
		},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		BinaryData:  make([]byte, 1024), // 1KB of data
		FloatValues: make([]float64, 100),
		Nested: &ComplexStruct{
			ID:   2,
			Name: "Nested Object",
			Tags: []string{"nested1", "nested2"},
		},
	}

	// Initialize large struct data
	for i := range largeStruct.Tags {
		largeStruct.Tags[i] = fmt.Sprintf("tag-%d", i)
	}
	for i := range largeStruct.FloatValues {
		largeStruct.FloatValues[i] = float64(i) * 1.23456
	}

	b.Run("serialize_small", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = serialize(smallStruct)
		}
	})

	b.Run("serialize_medium", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = serialize(mediumStruct)
		}
	})

	b.Run("serialize_large", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = serialize(largeStruct)
		}
	})

	// Prepare serialized data for deserialization
	smallData, _ := serialize(smallStruct)
	mediumData, _ := serialize(mediumStruct)
	largeData, _ := serialize(largeStruct)

	b.Run("deserialize_small", func(b *testing.B) {
		b.ReportAllocs()
		var result ComplexStruct
		for i := 0; i < b.N; i++ {
			_ = deserialize(smallData, &result)
		}
	})

	b.Run("deserialize_medium", func(b *testing.B) {
		b.ReportAllocs()
		var result ComplexStruct
		for i := 0; i < b.N; i++ {
			_ = deserialize(mediumData, &result)
		}
	})

	b.Run("deserialize_large", func(b *testing.B) {
		b.ReportAllocs()
		var result ComplexStruct
		for i := 0; i < b.N; i++ {
			_ = deserialize(largeData, &result)
		}
	})
}

// BenchmarkConcurrentRequests benchmarks concurrent request handling
func BenchmarkConcurrentRequests(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)

		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		w.Write([]byte(`{"key":"test","value":"data","version":1}`))
	}))
	defer server.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("concurrency_%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					client.Set(ctx, "key", "value")
				}
			})
		})
	}
}

// BenchmarkExtendedClient benchmarks extended client operations
func BenchmarkExtendedClient(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/cache/exists-test":
			w.Write([]byte(`{"key":"exists-test","value":"exists","version":1}`))
		default:
			if r.Method == http.MethodGet {
				// Return different values for different keys
				key := r.URL.Path[len("/v1/cache/"):]
				value := fmt.Sprintf(`"value-%s"`, key)
				resp := fmt.Sprintf(`{"key":"%s","value":%s,"version":1}`, key, value)
				w.Write([]byte(resp))
			} else {
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"key":"test","version":1}`))
			}
		}
	}))
	defer server.Close()

	client, _ := NewExtendedClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	b.Run("Exists", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			client.Exists(ctx, "exists-test")
		}
	})

	b.Run("GetMultiple_3_keys", func(b *testing.B) {
		b.ReportAllocs()
		keys := []string{"key1", "key2", "key3"}
		for i := 0; i < b.N; i++ {
			client.GetMultiple(ctx, keys)
		}
	})

	b.Run("GetMultiple_10_keys", func(b *testing.B) {
		b.ReportAllocs()
		keys := make([]string, 10)
		for i := range keys {
			keys[i] = fmt.Sprintf("key%d", i)
		}
		for i := 0; i < b.N; i++ {
			client.GetMultiple(ctx, keys)
		}
	})

	b.Run("SetWithOptions", func(b *testing.B) {
		b.ReportAllocs()
		ttl := 5 * time.Minute
		opts := &SetOptions{
			TTL: &ttl,
			Metadata: map[string]interface{}{
				"source": "benchmark",
			},
		}
		for i := 0; i < b.N; i++ {
			client.SetWithOptions(ctx, "key", "value", opts)
		}
	})
}

// BenchmarkTransportOperations benchmarks transport-level operations
func BenchmarkTransportOperations(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	transport, _ := newHTTPTransport(config)
	defer transport.close()

	ctx := context.Background()

	b.Run("GET_request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var result map[string]interface{}
			transport.get(ctx, "/test", &result)
		}
	})

	b.Run("POST_request", func(b *testing.B) {
		b.ReportAllocs()
		body := map[string]interface{}{"key": "value"}
		for i := 0; i < b.N; i++ {
			var result map[string]interface{}
			transport.post(ctx, "/test", body, &result)
		}
	})

	// retry_backoff_calculation is now benchmarked as part of retry strategies in resilience_test.go
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a large response
		largeData := make([]byte, 10*1024) // 10KB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		resp := CacheResponse{
			Key:       "test",
			Value:     json.RawMessage(fmt.Sprintf(`"%s"`, string(largeData))),
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b.Run("client_creation", func(b *testing.B) {
		b.ReportAllocs()
		config := DefaultConfig().WithBaseURL(server.URL)
		for i := 0; i < b.N; i++ {
			client, _ := NewClient(config)
			client.Close()
		}
	})

	b.Run("large_payload_handling", func(b *testing.B) {
		b.ReportAllocs()
		client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
		defer client.Close()

		ctx := context.Background()
		var result string

		for i := 0; i < b.N; i++ {
			client.Get(ctx, "large-key", &result)
		}
	})
}

// BenchmarkRealWorldScenarios benchmarks realistic usage patterns
func BenchmarkRealWorldScenarios(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Simulate cache hit/miss
			if r.URL.Path == "/v1/cache/miss" {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":"Not found","code":"NOT_FOUND"}`))
			} else {
				w.Write([]byte(`{"key":"test","value":{"id":123,"name":"Test User","email":"test@example.com"},"version":1}`))
			}
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"key":"test","version":1}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	b.Run("cache_hit_scenario", func(b *testing.B) {
		b.ReportAllocs()
		var user User
		for i := 0; i < b.N; i++ {
			err := client.Get(ctx, "user:123", &user)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("cache_miss_scenario", func(b *testing.B) {
		b.ReportAllocs()
		var user User
		for i := 0; i < b.N; i++ {
			err := client.Get(ctx, "miss", &user)
			if !IsNotFound(err) && err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("write_through_cache", func(b *testing.B) {
		b.ReportAllocs()
		user := User{
			ID:    123,
			Name:  "Test User",
			Email: "test@example.com",
		}

		for i := 0; i < b.N; i++ {
			// Simulate write-through cache pattern
			err := client.Set(ctx, fmt.Sprintf("user:%d", i), user)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("read_modify_write", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Read
			var user User
			client.Get(ctx, "user:123", &user)

			// Modify
			user.Name = fmt.Sprintf("Updated User %d", i)

			// Write back
			client.Set(ctx, "user:123", user)
		}
	})
}

// BenchmarkConnectionPooling benchmarks connection pool efficiency
func BenchmarkConnectionPooling(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some server processing
		time.Sleep(100 * time.Microsecond)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	configs := []struct {
		name            string
		maxIdleConns    int
		maxConnsPerHost int
	}{
		{"default", 100, 10},
		{"low_pool", 10, 2},
		{"high_pool", 500, 50},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			config := DefaultConfig().WithBaseURL(server.URL)
			config.TransportConfig.MaxIdleConns = cfg.maxIdleConns
			config.TransportConfig.MaxConnsPerHost = cfg.maxConnsPerHost

			client, _ := NewClient(config)
			defer client.Close()

			ctx := context.Background()

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					client.Ping(ctx)
				}
			})
		})
	}
}

// BenchmarkErrorHandling benchmarks error handling performance
func BenchmarkErrorHandling(b *testing.B) {
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return errors
		switch r.URL.Query().Get("type") {
		case "timeout":
			time.Sleep(2 * time.Second)
		case "server_error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"Internal server error","code":"INTERNAL_ERROR"}`))
		case "not_found":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"Not found","code":"NOT_FOUND"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Bad request","code":"BAD_REQUEST"}`))
		}
	}))
	defer errorServer.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(errorServer.URL))
	defer client.Close()

	ctx := context.Background()

	b.Run("parse_api_error", func(b *testing.B) {
		b.ReportAllocs()
		errorBody := []byte(`{"error":"Test error","code":"TEST_ERROR","details":"Additional details"}`)
		for i := 0; i < b.N; i++ {
			_ = parseAPIError(400, errorBody)
		}
	})

	b.Run("handle_not_found", func(b *testing.B) {
		b.ReportAllocs()
		var result string
		for i := 0; i < b.N; i++ {
			err := client.Get(ctx, "notfound?type=not_found", &result)
			_ = IsNotFound(err)
		}
	})

	b.Run("handle_server_error", func(b *testing.B) {
		b.ReportAllocs()
		var result string
		for i := 0; i < b.N; i++ {
			err := client.Get(ctx, "error?type=server_error", &result)
			_ = IsRetryable(err)
		}
	})
}

// BenchmarkTypedOperations benchmarks type-safe operations
func BenchmarkTypedOperations(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		w.Write([]byte(`{"key":"test","value":"benchmark string","version":1}`))
	}))
	defer server.Close()

	baseClient, _ := NewExtendedClient(DefaultConfig().WithBaseURL(server.URL))
	defer baseClient.Close()

	ctx := context.Background()

	b.Run("typed_client_string", func(b *testing.B) {
		b.ReportAllocs()
		client := NewStringClient(baseClient)

		b.Run("set", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				client.Set(ctx, "key", "value")
			}
		})

		b.Run("get", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = client.Get(ctx, "key")
			}
		})
	})

	b.Run("generic_functions", func(b *testing.B) {
		b.ReportAllocs()

		b.Run("SetTyped", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				SetTyped(ctx, baseClient, "key", "value")
			}
		})

		b.Run("GetTyped", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = GetTyped[string](ctx, baseClient, "key")
			}
		})
	})
}

// BenchmarkClientLifecycle benchmarks the full client lifecycle
func BenchmarkClientLifecycle(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	b.Run("create_use_close", func(b *testing.B) {
		b.ReportAllocs()
		config := DefaultConfig().WithBaseURL(server.URL)
		ctx := context.Background()

		for i := 0; i < b.N; i++ {
			client, err := NewClient(config)
			if err != nil {
				b.Fatal(err)
			}

			// Use the client
			client.Ping(ctx)

			// Close the client
			client.Close()
		}
	})

	b.Run("concurrent_lifecycle", func(b *testing.B) {
		b.ReportAllocs()
		config := DefaultConfig().WithBaseURL(server.URL)
		ctx := context.Background()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				client, _ := NewClient(config)
				client.Ping(ctx)
				client.Close()
			}
		})
	})
}

// Helper function to measure operation latency distribution
func measureLatencyDistribution(b *testing.B, operation func()) {
	latencies := make([]time.Duration, 0, b.N)

	for i := 0; i < b.N; i++ {
		start := time.Now()
		operation()
		latencies = append(latencies, time.Since(start))
	}

	// Calculate percentiles
	// This is a simplified version - in production you'd want proper sorting
	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		avg := total / time.Duration(len(latencies))
		b.ReportMetric(float64(avg.Nanoseconds()), "ns/op_avg")
	}
}
