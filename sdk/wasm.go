//go:build wasm

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"
)

// newHTTPTransport creates a new WASM-compatible HTTP transport
func newHTTPTransport(config *Config) (*httpTransport, error) {
	return &httpTransport{
		client:  nil, // WASM doesn't use http.Client
		config:  config,
		baseURL: nil, // We'll handle URL building differently in WASM
	}, nil
}

// do executes an HTTP request using the fetch API
func (t *httpTransport) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Build full URL
	fullURL := t.config.BaseURL + path

	// Create fetch options
	opts := map[string]interface{}{
		"method": method,
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"User-Agent":   "birb-nest-go-sdk-wasm/1.0.0",
		},
		"mode":        "cors",
		"credentials": "same-origin",
	}

	// Add custom headers
	headers := opts["headers"].(map[string]interface{})
	for key, value := range t.config.Headers {
		headers[key] = value
	}

	// Add body if present
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		opts["body"] = string(jsonBody)
	}

	// Execute fetch with retry logic
	var lastErr error
	for attempt := 0; attempt <= t.config.RetryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff
			backoff := t.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return ErrContextCanceled
			case <-time.After(backoff):
				// Continue with retry
			}
		}

		// Perform fetch
		resp, err := t.fetch(ctx, fullURL, opts)
		if err != nil {
			lastErr = &NetworkError{Op: method + " " + path, Err: err}
			continue
		}

		// Handle successful response
		if result != nil && len(resp) > 0 {
			if err := json.Unmarshal(resp, result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
		}
		return nil
	}

	return lastErr
}

// fetch performs the actual fetch operation
func (t *httpTransport) fetch(ctx context.Context, url string, opts map[string]interface{}) ([]byte, error) {
	// Create channels for result and timeout
	resultChan := make(chan js.Value, 1)
	errChan := make(chan error, 1)

	// Get the global fetch function
	fetchFunc := js.Global().Get("fetch")
	if !fetchFunc.Truthy() {
		return nil, fmt.Errorf("fetch API not available")
	}

	// Convert options to JS object
	jsOpts := js.ValueOf(map[string]interface{}{})
	for key, value := range opts {
		jsOpts.Set(key, js.ValueOf(value))
	}

	// Perform fetch
	promise := fetchFunc.Invoke(url, jsOpts)

	// Handle promise resolution
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]

		// Check response status
		status := response.Get("status").Int()
		if status < 200 || status >= 300 {
			// Read error body
			response.Call("text").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				body := args[0].String()
				errChan <- parseAPIError(status, []byte(body))
				return nil
			}))
			return nil
		}

		// Read response body
		response.Call("text").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			resultChan <- args[0]
			return nil
		}))
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		err := args[0]
		errMsg := "network error"
		if err.Get("message").Truthy() {
			errMsg = err.Get("message").String()
		}
		errChan <- fmt.Errorf(errMsg)
		return nil
	}))

	// Wait for result with timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return []byte(result.String()), nil
	case err := <-errChan:
		return nil, err
	case <-time.After(t.config.Timeout):
		return nil, &TimeoutError{Op: "fetch " + url}
	}
}

// get performs a GET request
func (t *httpTransport) get(ctx context.Context, path string, result interface{}) error {
	return t.do(ctx, "GET", path, nil, result)
}

// post performs a POST request
func (t *httpTransport) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return t.do(ctx, "POST", path, body, result)
}

// put performs a PUT request
func (t *httpTransport) put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return t.do(ctx, "PUT", path, body, result)
}

// delete performs a DELETE request
func (t *httpTransport) delete(ctx context.Context, path string) error {
	return t.do(ctx, "DELETE", path, nil, nil)
}

// calculateBackoff calculates the backoff duration for a retry attempt
func (t *httpTransport) calculateBackoff(attempt int) time.Duration {
	// Simplified backoff for WASM (no math package in some WASM environments)
	backoff := t.config.RetryConfig.InitialInterval
	for i := 1; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * t.config.RetryConfig.Multiplier)
		if backoff > t.config.RetryConfig.MaxInterval {
			backoff = t.config.RetryConfig.MaxInterval
			break
		}
	}
	return backoff
}

// close closes the transport (no-op for WASM)
func (t *httpTransport) close() error {
	return nil
}

// WASM-specific helper to check if we're in a browser environment
func isBrowserEnvironment() bool {
	// Check if we have access to browser globals
	window := js.Global().Get("window")
	document := js.Global().Get("document")
	return window.Truthy() && document.Truthy()
}

// WASM-specific initialization
func init() {
	// No initialization needed - both browser and Node.js environments are supported
}
