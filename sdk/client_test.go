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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates a test HTTP server that mimics the Birb Nest API
func mockServer() *httptest.Server {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status:  "healthy",
			Service: "birb-nest-api",
			Version: "1.0.0",
			Uptime:  "1h",
			Checks: map[string]string{
				"database": "healthy",
				"redis":    "healthy",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Cache endpoints
	mux.HandleFunc("/v1/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/v1/cache/"):]

		switch r.Method {
		case http.MethodGet:
			if key == "test-key" {
				resp := CacheResponse{
					Key:       key,
					Value:     json.RawMessage(`"test-value"`),
					Version:   1,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Cache entry not found",
					"code":  "NOT_FOUND",
				})
			}

		case http.MethodPost:
			var req CacheRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

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

		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	return httptest.NewServer(mux)
}

func TestClient_Ping(t *testing.T) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewClient(config)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	ctx := context.Background()
	err = client.Ping(ctx)
	assert.NoError(t, err, "Ping should succeed")
}

func TestClient_SetAndGet(t *testing.T) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewClient(config)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	ctx := context.Background()

	// Test Set
	err = client.Set(ctx, "my-key", "my-value")
	assert.NoError(t, err, "Set should succeed")

	// Test Get
	var value string
	err = client.Get(ctx, "test-key", &value)
	assert.NoError(t, err, "Get should succeed")
	assert.Equal(t, "test-value", value, "Retrieved value should match expected")
}

func TestClient_NotFound(t *testing.T) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewClient(config)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	ctx := context.Background()

	var value string
	err = client.Get(ctx, "non-existent", &value)
	assert.Error(t, err, "Get should return error for non-existent key")
	assert.True(t, IsNotFound(err), "Error should be NotFound type")
}

func TestClient_Delete(t *testing.T) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewClient(config)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	ctx := context.Background()

	err = client.Delete(ctx, "test-key")
	assert.NoError(t, err, "Delete should succeed")
}

func TestExtendedClient_SetWithOptions(t *testing.T) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewExtendedClient(config)
	require.NoError(t, err, "Failed to create extended client")
	defer client.Close()

	ctx := context.Background()

	ttl := 30 * time.Second
	opts := &SetOptions{
		TTL: &ttl,
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	err = client.SetWithOptions(ctx, "ttl-key", "ttl-value", opts)
	assert.NoError(t, err, "SetWithOptions should succeed")
}

func TestClient_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}))
	defer server.Close()

	config := DefaultConfig().
		WithBaseURL(server.URL).
		WithRetries(3)

	client, err := NewClient(config)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	ctx := context.Background()
	err = client.Ping(ctx)
	assert.NoError(t, err, "Ping should succeed after retries")
	assert.Equal(t, 3, attempts, "Should retry exactly 3 times")
}

func TestSerialization(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{
			name:  "string",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "number",
			input: 42,
			want:  `42`,
		},
		{
			name:  "struct",
			input: struct{ Name string }{Name: "test"},
			want:  `{"Name":"test"}`,
		},
		{
			name:  "raw json",
			input: json.RawMessage(`{"key":"value"}`),
			want:  `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := serialize(tt.input)
			require.NoError(t, err, "serialize should not return error")
			assert.Equal(t, tt.want, string(result), "Serialized output mismatch")
		})
	}
}

func BenchmarkClient_Set(b *testing.B) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, _ := NewClient(config)
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		client.Set(ctx, key, "value")
	}
}

func BenchmarkClient_Get(b *testing.B) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, _ := NewClient(config)
	defer client.Close()

	ctx := context.Background()
	var value string

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Get(ctx, "test-key", &value)
	}
}

// TestClient_TableDriven provides comprehensive table-driven tests for client operations
func TestClient_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		key       string
		value     interface{}
		setup     func(*http.ServeMux)
		wantErr   bool
		errCheck  func(t *testing.T, err error)
	}{
		{
			name:      "successful_set_string",
			operation: "set",
			key:       "string-key",
			value:     "string-value",
			wantErr:   false,
		},
		{
			name:      "successful_set_struct",
			operation: "set",
			key:       "struct-key",
			value:     struct{ Name string }{Name: "test"},
			wantErr:   false,
		},
		{
			name:      "network_error",
			operation: "set",
			key:       "error-key",
			value:     "value",
			setup: func(mux *http.ServeMux) {
				mux.HandleFunc("/v1/cache/error-key", func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				})
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				var enhancedErr *Error
				if assert.ErrorAs(t, err, &enhancedErr) {
					assert.Equal(t, ErrorTypeServer, enhancedErr.Type)
					if statusCode, ok := enhancedErr.Details["status_code"].(int); ok {
						assert.Equal(t, http.StatusInternalServerError, statusCode)
					}
				}
			},
		},
		{
			name:      "rate_limit_error",
			operation: "get",
			key:       "rate-limited-key",
			setup: func(mux *http.ServeMux) {
				mux.HandleFunc("/v1/cache/rate-limited-key", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Too many requests",
						"code":  "RATE_LIMITED",
					})
				})
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				var enhancedErr *Error
				if assert.ErrorAs(t, err, &enhancedErr) {
					assert.Equal(t, ErrorTypeRateLimit, enhancedErr.Type)
					if statusCode, ok := enhancedErr.Details["status_code"].(int); ok {
						assert.Equal(t, http.StatusTooManyRequests, statusCode)
					}
					assert.Equal(t, "RATE_LIMITED", enhancedErr.Code)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()

			// Set up default handlers
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			})

			// Default cache handler
			mux.HandleFunc("/v1/cache/", func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusCreated)
					json.NewEncoder(w).Encode(map[string]interface{}{"key": r.URL.Path[len("/v1/cache/"):]})
				} else if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{"value": "test"})
				}
			})

			// Apply custom setup if provided
			if tt.setup != nil {
				tt.setup(mux)
			}

			server := httptest.NewServer(mux)
			defer server.Close()

			config := DefaultConfig().WithBaseURL(server.URL)
			client, err := NewClient(config)
			require.NoError(t, err)
			defer client.Close()

			ctx := context.Background()
			var resultErr error

			switch tt.operation {
			case "set":
				resultErr = client.Set(ctx, tt.key, tt.value)
			case "get":
				var result interface{}
				resultErr = client.Get(ctx, tt.key, &result)
			}

			if tt.wantErr {
				assert.Error(t, resultErr)
				if tt.errCheck != nil {
					tt.errCheck(t, resultErr)
				}
			} else {
				assert.NoError(t, resultErr)
			}
		})
	}
}

// TestClient_ContextCancellation tests that operations respect context cancellation
func TestClient_ContextCancellation(t *testing.T) {
	// Create a server that delays responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = client.Set(ctx, "key", "value")
	assert.Error(t, err, "Expected error due to context cancellation")
	assert.Contains(t, err.Error(), "context")
}

// TestClient_ConcurrentOperations tests thread safety of client operations
func TestClient_ConcurrentOperations(t *testing.T) {
	server := mockServer()
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	numGoroutines := 10
	numOperations := 100

	// Use a wait group to coordinate goroutines
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)

				// Perform Set operation
				if err := client.Set(ctx, key, value); err != nil {
					errors <- err
				}

				// Perform Get operation
				var result string
				if err := client.Get(ctx, "test-key", &result); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	var errorCount int
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "No errors should occur during concurrent operations")
}

// TestClient_GetMultiple tests the GetMultiple functionality
func TestExtendedClient_GetMultiple(t *testing.T) {
	mux := http.NewServeMux()

	// Set up test data
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	mux.HandleFunc("/v1/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/v1/cache/"):]
		if value, ok := testData[key]; ok {
			resp := CacheResponse{
				Key:       key,
				Value:     json.RawMessage(fmt.Sprintf(`"%s"`, value)),
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Cache entry not found",
				"code":  "NOT_FOUND",
			})
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewExtendedClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Test GetMultiple with existing keys
	keys := []string{"key1", "key2", "key3"}
	results, err := client.GetMultiple(ctx, keys)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	for key, expectedValue := range testData {
		if value, ok := results[key]; ok {
			// The value is already deserialized by GetMultiple
			assert.Equal(t, expectedValue, value)
		} else {
			t.Errorf("Missing result for key: %s", key)
		}
	}

	// Test GetMultiple with some missing keys
	keysWithMissing := []string{"key1", "missing-key", "key3"}
	results, err = client.GetMultiple(ctx, keysWithMissing)
	assert.NoError(t, err)
	assert.Len(t, results, 2, "Should only return existing keys")
}

// TestClient_Exists tests the Exists functionality
func TestExtendedClient_Exists(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	mux.HandleFunc("/v1/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/v1/cache/"):]
		if key == "existing-key" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CacheResponse{
				Key:   key,
				Value: json.RawMessage(`"value"`),
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not found",
				"code":  "NOT_FOUND",
			})
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	client, err := NewExtendedClient(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Test existing key
	exists, err := client.Exists(ctx, "existing-key")
	assert.NoError(t, err)
	assert.True(t, exists, "Key should exist")

	// Test non-existing key
	exists, err = client.Exists(ctx, "non-existing-key")
	assert.NoError(t, err)
	assert.False(t, exists, "Key should not exist")
}
