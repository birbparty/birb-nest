package testdata

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MockServer provides a configurable test HTTP server
type MockServer struct {
	*httptest.Server
	mu           sync.RWMutex
	handlers     map[string]HandlerFunc
	requestCount atomic.Int32
	requests     []RecordedRequest
}

// HandlerFunc is a custom handler function type
type HandlerFunc func(w http.ResponseWriter, r *http.Request) (int, interface{})

// RecordedRequest stores information about a received request
type RecordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
	Time    time.Time
}

// NewMockServer creates a new mock server
func NewMockServer() *MockServer {
	ms := &MockServer{
		handlers: make(map[string]HandlerFunc),
		requests: make([]RecordedRequest, 0),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", ms.handleRequest)

	ms.Server = httptest.NewServer(mux)
	ms.setupDefaultHandlers()

	return ms
}

// setupDefaultHandlers sets up common handlers
func (ms *MockServer) setupDefaultHandlers() {
	// Health endpoint
	ms.RegisterHandler("GET /health", func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		return http.StatusOK, map[string]interface{}{
			"status":  "healthy",
			"service": "birb-nest-api",
			"version": "1.0.0",
			"uptime":  "1h",
			"checks": map[string]string{
				"database": "healthy",
				"redis":    "healthy",
				"nats":     "healthy",
			},
		}
	})

	// Default cache handlers
	ms.RegisterHandler("GET /v1/cache/test-key", func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		return http.StatusOK, map[string]interface{}{
			"key":        "test-key",
			"value":      "test-value",
			"version":    1,
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		}
	})

	ms.RegisterHandler("POST /v1/cache/", func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		key := r.URL.Path[len("/v1/cache/"):]
		return http.StatusCreated, map[string]interface{}{
			"key":        key,
			"value":      body["value"],
			"version":    1,
			"ttl":        body["ttl"],
			"metadata":   body["metadata"],
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		}
	})

	ms.RegisterHandler("DELETE /v1/cache/", func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		return http.StatusNoContent, nil
	})
}

// RegisterHandler registers a custom handler for a specific method and path pattern
func (ms *MockServer) RegisterHandler(pattern string, handler HandlerFunc) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.handlers[pattern] = handler
}

// handleRequest routes requests to appropriate handlers
func (ms *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Record the request
	body := make([]byte, 0)
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	ms.mu.Lock()
	ms.requests = append(ms.requests, RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
		Body:    body,
		Time:    time.Now(),
	})
	ms.mu.Unlock()

	ms.requestCount.Add(1)

	// Find matching handler
	pattern := r.Method + " " + r.URL.Path
	ms.mu.RLock()
	handler, exact := ms.handlers[pattern]
	if !exact {
		// Try prefix match for dynamic paths
		for p, h := range ms.handlers {
			if strings.HasSuffix(p, "/") && strings.HasPrefix(pattern, p) {
				handler = h
				break
			}
		}
	}
	ms.mu.RUnlock()

	if handler == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	// Execute handler
	status, response := handler(w, r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if response != nil {
		json.NewEncoder(w).Encode(response)
	}
}

// GetRequestCount returns the total number of requests received
func (ms *MockServer) GetRequestCount() int {
	return int(ms.requestCount.Load())
}

// GetRequests returns all recorded requests
func (ms *MockServer) GetRequests() []RecordedRequest {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make([]RecordedRequest, len(ms.requests))
	copy(result, ms.requests)
	return result
}

// Reset clears all recorded requests
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.requestCount.Store(0)
	ms.requests = ms.requests[:0]
}

// WithErrorResponse sets up a handler that returns an error
func (ms *MockServer) WithErrorResponse(pattern string, statusCode int, errorMsg string) {
	ms.RegisterHandler(pattern, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		return statusCode, map[string]string{
			"error": errorMsg,
			"code":  http.StatusText(statusCode),
		}
	})
}

// WithDelayedResponse sets up a handler that delays before responding
func (ms *MockServer) WithDelayedResponse(pattern string, delay time.Duration, handler HandlerFunc) {
	ms.RegisterHandler(pattern, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		time.Sleep(delay)
		return handler(w, r)
	})
}

// WithRetryResponse sets up a handler that fails N times before succeeding
func (ms *MockServer) WithRetryResponse(pattern string, failCount int, failStatus int) {
	attempts := atomic.Int32{}
	ms.RegisterHandler(pattern, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		current := int(attempts.Add(1))
		if current <= failCount {
			return failStatus, map[string]string{
				"error": "Temporary failure",
				"code":  "TEMP_ERROR",
			}
		}
		return http.StatusOK, map[string]string{"status": "success"}
	})
}

// Close shuts down the mock server
func (ms *MockServer) Close() {
	if ms.Server != nil {
		ms.Server.Close()
	}
}
