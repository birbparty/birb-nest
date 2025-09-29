package sdk

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// httpTransport handles HTTP communication with the Birb Nest API.
// It provides a platform-agnostic interface for making HTTP requests
// with built-in retry logic, circuit breaking, and observability.
//
// The actual implementation is split between:
//   - native.go: Standard Go HTTP client for regular builds
//   - wasm.go: Fetch API wrapper for WebAssembly builds
//
// This design allows the SDK to work seamlessly in both server-side
// Go applications and browser-based WASM applications.
type httpTransport struct {
	// client is the underlying HTTP client (native Go only)
	client *http.Client
	// config holds the SDK configuration
	config *Config
	// baseURL is the parsed base URL for the API
	baseURL *url.URL
	// circuitBreaker provides fault tolerance
	circuitBreaker CircuitBreaker
	// perEndpointCircuitBreaker provides per-endpoint circuit breaking
	perEndpointCircuitBreaker *perEndpointCircuitBreaker
	// retryExecutor handles retry logic
	retryExecutor *retryExecutor
	// observer for monitoring operations
	observer Observer
}

// newHTTPTransport creates a new HTTP transport.
// The implementation is defined in platform-specific files:
//   - native.go: Uses Go's standard net/http package
//   - wasm.go: Uses the browser's Fetch API via syscall/js
//
// This allows the SDK to work in both environments without
// requiring different APIs or separate packages.
//
// The transport provides these methods (defined in platform files):
//   - get(ctx, path, response): Performs GET requests
//   - post(ctx, path, body, response): Performs POST requests
//   - delete(ctx, path): Performs DELETE requests
//   - close(): Closes the transport and releases resources

// buildPath builds a URL path with proper escaping for path parameters.
// It replaces placeholders like {0}, {1}, etc. with the provided arguments,
// ensuring all special characters are properly URL-encoded.
//
// This function is particularly important for cache keys that may contain
// special characters like spaces, slashes, or other URL-unsafe characters.
//
// Example:
//
//	path := buildPath("/v1/cache/{0}", "my key/with=special&chars")
//	// Result: "/v1/cache/my%20key%2Fwith%3Dspecial%26chars"
//
// Parameters:
//   - pattern: Path pattern with {0}, {1}, etc. placeholders
//   - args: Values to substitute for the placeholders
//
// The function uses QueryEscape for encoding, then replaces '+' with '%20'
// to ensure proper space encoding in URL paths (as '+' is only valid in
// query strings, not paths).
func buildPath(pattern string, args ...string) string {
	path := pattern
	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		// Use QueryEscape to encode all special characters including =, &, etc.
		// Then manually replace + with %20 for proper space encoding in paths
		escaped := url.QueryEscape(arg)
		escaped = strings.Replace(escaped, "+", "%20", -1)
		path = strings.Replace(path, placeholder, escaped, 1)
	}
	return path
}
