//go:build !wasm

package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// newHTTPTransport creates a native HTTP transport
func newHTTPTransport(config *Config) (*httpTransport, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Validate that it's a proper URL with scheme and host
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("base URL must have a scheme and host")
	}

	// Configure the HTTP transport
	transport := &http.Transport{
		MaxIdleConns:        config.TransportConfig.MaxIdleConns,
		MaxConnsPerHost:     config.TransportConfig.MaxConnsPerHost,
		IdleConnTimeout:     config.TransportConfig.IdleConnTimeout,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	// Create circuit breaker if configured
	var circuitBreaker CircuitBreaker
	var perEndpointCB *perEndpointCircuitBreaker
	if config.CircuitBreakerConfig != nil {
		if config.EnablePerEndpointCircuitBreaker {
			perEndpointCB = NewPerEndpointCircuitBreaker(*config.CircuitBreakerConfig)
			circuitBreaker = NewNoopCircuitBreaker() // Use noop for the regular interface
		} else {
			cb := NewCircuitBreaker(*config.CircuitBreakerConfig)
			// Wrap with observer if configured
			if config.Observer != nil {
				circuitBreaker = newObservedCircuitBreaker(cb, "default", config.Observer)
			} else {
				circuitBreaker = cb
			}
		}
	} else {
		circuitBreaker = NewNoopCircuitBreaker()
	}

	// Create retry executor
	var retryStrategy RetryStrategy
	if config.RetryStrategy != nil {
		retryStrategy = config.RetryStrategy
	} else {
		// Use default exponential backoff
		retryStrategy = &ExponentialBackoffStrategy{
			InitialInterval: config.RetryConfig.InitialInterval,
			MaxInterval:     config.RetryConfig.MaxInterval,
			Multiplier:      config.RetryConfig.Multiplier,
			Jitter:          0.3, // Default 30% jitter
			Budget: RetryBudget{
				MaxAttempts: config.RetryConfig.MaxRetries + 1, // +1 for initial attempt
				MaxDuration: 0,                                 // No duration limit by default
			},
		}
	}

	return &httpTransport{
		client:                    client,
		config:                    config,
		baseURL:                   baseURL,
		circuitBreaker:            circuitBreaker,
		perEndpointCircuitBreaker: perEndpointCB,
		retryExecutor:             newRetryExecutor(retryStrategy),
		observer:                  config.Observer,
	}, nil
}

// do executes an HTTP request with retry logic
func (t *httpTransport) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Notify observer of request start
	if t.observer != nil {
		t.observer.OnRequestStart(method, path)
	}

	start := time.Now()
	var finalErr error

	// Execute with circuit breaker and retry logic
	endpoint := method + " " + path
	executeFn := func() error {
		return t.executeRequest(ctx, method, path, body, result)
	}

	// Wrap with circuit breaker
	if t.perEndpointCircuitBreaker != nil {
		finalErr = t.perEndpointCircuitBreaker.Execute(endpoint, executeFn)
	} else {
		finalErr = t.circuitBreaker.Execute(executeFn)
	}

	// Notify observer of request end
	if t.observer != nil {
		duration := time.Since(start)
		t.observer.OnRequestEnd(method, path, duration, finalErr)
	}

	return finalErr
}

// executeRequest performs the actual HTTP request
func (t *httpTransport) executeRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Use retry executor for the actual request
	return t.retryExecutor.Execute(ctx, func() error {
		return t.performHTTPRequest(ctx, method, path, body, result)
	})
}

// performHTTPRequest performs a single HTTP request
func (t *httpTransport) performHTTPRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// Build the full URL
	fullURL := t.baseURL.ResolveReference(&url.URL{Path: path})

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "birb-nest-go-sdk/1.0.0")

	// Add custom headers
	for key, value := range t.config.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		netErr := &NetworkError{Op: method + " " + path, Err: err}
		return netErr.ToError()
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		netErr := &NetworkError{Op: "reading response", Err: err}
		return netErr.ToError()
	}

	// Check status code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success - parse result if needed
		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
		}
		return nil
	}

	// Handle error response
	apiErr := parseAPIError(resp.StatusCode, respBody)

	// Convert to enhanced error
	if apiErrTyped, ok := apiErr.(*APIError); ok {
		enhancedErr := apiErrTyped.ToError()
		// Add request context
		enhancedErr.WithContext(&ErrorContext{
			URL:    fullURL.String(),
			Method: method,
		})
		// Extract request ID from headers if available
		if reqID := resp.Header.Get("X-Request-ID"); reqID != "" {
			enhancedErr.RequestID = reqID
		}
		return enhancedErr
	}

	return apiErr
}

// get performs a GET request
func (t *httpTransport) get(ctx context.Context, path string, result interface{}) error {
	return t.do(ctx, http.MethodGet, path, nil, result)
}

// post performs a POST request
func (t *httpTransport) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return t.do(ctx, http.MethodPost, path, body, result)
}

// put performs a PUT request
func (t *httpTransport) put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return t.do(ctx, http.MethodPut, path, body, result)
}

// delete performs a DELETE request
func (t *httpTransport) delete(ctx context.Context, path string) error {
	return t.do(ctx, http.MethodDelete, path, nil, nil)
}

// close closes the transport
func (t *httpTransport) close() error {
	t.client.CloseIdleConnections()
	return nil
}
