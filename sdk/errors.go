package sdk

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Common errors returned by the SDK. These can be used with errors.Is()
// to check for specific error conditions.
//
// Example:
//
//	err := client.Get(ctx, "key", &value)
//	if errors.Is(err, sdk.ErrNotFound) {
//	    // Handle missing key
//	} else if errors.Is(err, sdk.ErrTimeout) {
//	    // Handle timeout
//	} else if errors.Is(err, sdk.ErrCircuitOpen) {
//	    // Circuit breaker is open, service is down
//	}
var (
	// ErrInvalidConfig is returned when the configuration is invalid
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrNotFound is returned when a key is not found in the cache
	ErrNotFound = errors.New("key not found")

	// ErrTimeout is returned when a request times out
	ErrTimeout = errors.New("request timeout")

	// ErrServerError is returned for 5xx server errors
	ErrServerError = errors.New("server error")

	// ErrInvalidResponse is returned when the server response cannot be parsed
	ErrInvalidResponse = errors.New("invalid response from server")

	// ErrContextCanceled is returned when the context is canceled before completion
	ErrContextCanceled = errors.New("context canceled")

	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrRateLimited is returned when the request is rate limited
	ErrRateLimited = errors.New("rate limited")

	// ErrRetryBudgetExhausted is returned when retry budget is exhausted
	ErrRetryBudgetExhausted = errors.New("retry budget exhausted")
)

// ErrorType represents the type of error for categorization and handling.
// Different error types may have different retry behaviors.
//
// Example:
//
//	var sdkErr *sdk.Error
//	if errors.As(err, &sdkErr) {
//	    switch sdkErr.Type {
//	    case sdk.ErrorTypeNetwork:
//	        // Handle network errors
//	    case sdk.ErrorTypeRateLimit:
//	        // Back off and retry later
//	    case sdk.ErrorTypeCircuitOpen:
//	        // Service is down, fail fast
//	    }
//	}
type ErrorType int

const (
	// ErrorTypeUnknown represents an unknown or unclassified error
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeNetwork represents network-related errors (connection refused, DNS, etc.)
	ErrorTypeNetwork
	// ErrorTypeTimeout represents timeout errors (request timeout, context deadline)
	ErrorTypeTimeout
	// ErrorTypeServer represents server errors (5xx HTTP status codes)
	ErrorTypeServer
	// ErrorTypeClient represents client errors (4xx HTTP status codes)
	ErrorTypeClient
	// ErrorTypeCircuitOpen represents circuit breaker open state errors
	ErrorTypeCircuitOpen
	// ErrorTypeRateLimit represents rate limiting errors (429 Too Many Requests)
	ErrorTypeRateLimit
	// ErrorTypeValidation represents validation errors (invalid input, config, etc.)
	ErrorTypeValidation
	// ErrorTypeRetryBudget represents retry budget exhausted errors
	ErrorTypeRetryBudget
)

// String returns the string representation of the error type
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeNetwork:
		return "network"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeServer:
		return "server"
	case ErrorTypeClient:
		return "client"
	case ErrorTypeCircuitOpen:
		return "circuit_open"
	case ErrorTypeRateLimit:
		return "rate_limit"
	case ErrorTypeValidation:
		return "validation"
	case ErrorTypeRetryBudget:
		return "retry_budget"
	default:
		return "unknown"
	}
}

// Error represents an enhanced error with additional context and metadata.
// It provides detailed information about what went wrong, whether the error
// is retryable, and context about the operation that failed.
//
// The Error type implements the error interface and supports error wrapping
// via errors.Is() and errors.As().
//
// Example:
//
//	var sdkErr *sdk.Error
//	if errors.As(err, &sdkErr) {
//	    fmt.Printf("Error Type: %s\n", sdkErr.Type)
//	    fmt.Printf("Retryable: %v\n", sdkErr.IsRetryable())
//	    if sdkErr.Context != nil {
//	        fmt.Printf("Failed URL: %s\n", sdkErr.Context.URL)
//	        fmt.Printf("Retry Count: %d\n", sdkErr.Context.RetryCount)
//	    }
//	}
type Error struct {
	// Type categorizes the error for handling decisions
	Type ErrorType `json:"type"`
	// Code is an optional error code from the server
	Code string `json:"code,omitempty"`
	// Message is a human-readable error description
	Message string `json:"message"`
	// Details contains additional error metadata
	Details map[string]interface{} `json:"details,omitempty"`
	// RequestID is the unique request identifier for tracing
	RequestID string `json:"request_id,omitempty"`
	// Timestamp is when the error occurred
	Timestamp time.Time `json:"timestamp"`
	// Retryable indicates if the operation can be retried
	Retryable bool `json:"retryable"`
	// Context provides additional context about the failed operation
	Context *ErrorContext `json:"context,omitempty"`
	// wrapped is the underlying error, if any
	wrapped error
}

// ErrorContext provides additional context about the operation that failed.
// This helps with debugging and understanding the circumstances of the error.
//
// Example:
//
//	if sdkErr.Context != nil {
//	    log.Printf("Failed after %d retries to %s %s (took %v)",
//	        sdkErr.Context.RetryCount,
//	        sdkErr.Context.Method,
//	        sdkErr.Context.URL,
//	        sdkErr.Context.Duration)
//	}
type ErrorContext struct {
	// URL is the full URL of the failed request
	URL string `json:"url,omitempty"`
	// Method is the HTTP method used (GET, POST, DELETE, etc.)
	Method string `json:"method,omitempty"`
	// Headers contains relevant request headers (excluding sensitive data)
	Headers map[string]string `json:"headers,omitempty"`
	// Duration is how long the operation took before failing
	Duration time.Duration `json:"duration,omitempty"`
	// RetryCount is the number of retry attempts made
	RetryCount int `json:"retry_count,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Context != nil && e.Context.URL != "" {
		return fmt.Sprintf("%s error: %s (url: %s, retries: %d)", e.Type, e.Message, e.Context.URL, e.Context.RetryCount)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.wrapped
}

// Is implements errors.Is
func (e *Error) Is(target error) bool {
	switch e.Type {
	case ErrorTypeTimeout:
		return errors.Is(target, ErrTimeout)
	case ErrorTypeServer:
		return errors.Is(target, ErrServerError)
	case ErrorTypeCircuitOpen:
		return errors.Is(target, ErrCircuitOpen)
	case ErrorTypeRateLimit:
		return errors.Is(target, ErrRateLimited)
	case ErrorTypeRetryBudget:
		return errors.Is(target, ErrRetryBudgetExhausted)
	}
	return false
}

// IsRetryable returns true if the error is retryable
func (e *Error) IsRetryable() bool {
	return e.Retryable
}

// WithContext adds error context
func (e *Error) WithContext(ctx *ErrorContext) *Error {
	e.Context = ctx
	return e
}

// WithDetail adds a detail to the error
func (e *Error) WithDetail(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// NewError creates a new enhanced error
func NewError(errType ErrorType, message string, wrapped error) *Error {
	return &Error{
		Type:      errType,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryableType(errType),
		wrapped:   wrapped,
	}
}

// NewErrorWithCode creates a new enhanced error with a code
func NewErrorWithCode(errType ErrorType, code, message string, wrapped error) *Error {
	err := NewError(errType, message, wrapped)
	err.Code = code
	return err
}

// isRetryableType determines if an error type is retryable
func isRetryableType(errType ErrorType) bool {
	switch errType {
	case ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeServer, ErrorTypeRateLimit:
		return true
	default:
		return false
	}
}

// APIError represents an error response from the Birb Nest API.
// It contains the HTTP status code and error details from the server.
//
// Example:
//
//	var apiErr *sdk.APIError
//	if errors.As(err, &apiErr) {
//	    if apiErr.IsNotFound() {
//	        // Handle 404
//	    } else if apiErr.IsServerError() {
//	        // Handle 5xx - maybe retry
//	    } else if apiErr.StatusCode == 429 {
//	        // Handle rate limiting
//	    }
//	}
type APIError struct {
	// StatusCode is the HTTP status code from the response
	StatusCode int `json:"-"`
	// Message is the error message from the server
	Message string `json:"error"`
	// Code is an optional error code for programmatic handling
	Code string `json:"code,omitempty"`
	// Details provides additional error information
	Details string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error (status %d): %s - %s", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a not found error
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound || e.Code == "NOT_FOUND"
}

// IsServerError returns true if the error is a server error
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500
}

// IsClientError returns true if the error is a client error
func (e *APIError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsRetryable returns true if the error is retryable
func (e *APIError) IsRetryable() bool {
	// Retry on server errors and specific client errors
	if e.IsServerError() {
		return true
	}
	// Retry on rate limiting
	if e.StatusCode == http.StatusTooManyRequests {
		return true
	}
	// Retry on timeout
	if e.StatusCode == http.StatusRequestTimeout || e.StatusCode == http.StatusGatewayTimeout {
		return true
	}
	return false
}

// ToError converts APIError to the enhanced Error type
func (e *APIError) ToError() *Error {
	errType := ErrorTypeClient
	if e.IsServerError() {
		errType = ErrorTypeServer
	} else if e.StatusCode == http.StatusTooManyRequests {
		errType = ErrorTypeRateLimit
	} else if e.StatusCode == http.StatusRequestTimeout || e.StatusCode == http.StatusGatewayTimeout {
		errType = ErrorTypeTimeout
	}

	err := NewErrorWithCode(errType, e.Code, e.Message, e)
	if e.Details != "" {
		err.WithDetail("api_details", e.Details)
	}
	err.WithDetail("status_code", e.StatusCode)
	return err
}

// NetworkError represents a network-related error such as connection
// refused, DNS resolution failure, or connection timeout.
//
// Example:
//
//	var netErr *sdk.NetworkError
//	if errors.As(err, &netErr) {
//	    log.Printf("Network error during %s: %v", netErr.Op, netErr.Err)
//	    // Network errors are typically retryable
//	}
type NetworkError struct {
	// Op is the operation that failed (e.g., "dial", "read", "write")
	Op string
	// Err is the underlying network error
	Err error
}

// Error implements the error interface
func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the network error is retryable
func (e *NetworkError) IsRetryable() bool {
	// Most network errors are retryable
	return true
}

// ToError converts NetworkError to the enhanced Error type
func (e *NetworkError) ToError() *Error {
	err := NewError(ErrorTypeNetwork, e.Error(), e)
	err.WithDetail("operation", e.Op)
	return err
}

// TimeoutError represents an operation that exceeded its time limit.
// This could be due to request timeout, context deadline, or slow server response.
//
// Example:
//
//	var timeoutErr *sdk.TimeoutError
//	if errors.As(err, &timeoutErr) {
//	    log.Printf("Operation %s timed out", timeoutErr.Op)
//	    // Timeout errors are retryable
//	}
type TimeoutError struct {
	// Op is the operation that timed out
	Op string
}

// Error implements the error interface
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout during %s", e.Op)
}

// IsRetryable returns true - timeout errors are always retryable
func (e *TimeoutError) IsRetryable() bool {
	return true
}

// ToError converts TimeoutError to the enhanced Error type
func (e *TimeoutError) ToError() *Error {
	err := NewError(ErrorTypeTimeout, e.Error(), e)
	err.WithDetail("operation", e.Op)
	return err
}

// IsNotFound checks if the error represents a "not found" condition.
// This includes checking for ErrNotFound, 404 status codes, and "NOT_FOUND" error codes.
//
// Example:
//
//	var data MyData
//	err := client.Get(ctx, "key", &data)
//	if sdk.IsNotFound(err) {
//	    // Key doesn't exist, maybe create default
//	    data = MyData{/* defaults */}
//	} else if err != nil {
//	    return err
//	}
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}
	var enhancedErr *Error
	if errors.As(err, &enhancedErr) {
		if apiErr, ok := enhancedErr.wrapped.(*APIError); ok {
			return apiErr.IsNotFound()
		}
	}
	return false
}

// IsRetryable checks if an error is retryable.
// Retryable errors include:
//   - Network errors (connection issues)
//   - Timeout errors
//   - Server errors (5xx status codes)
//   - Rate limiting errors (429 status)
//   - Certain infrastructure errors
//
// Non-retryable errors include:
//   - Client errors (4xx status codes except 429)
//   - Validation errors
//   - Circuit breaker open (fail fast)
//
// Example:
//
//	err := client.Set(ctx, "key", value)
//	if err != nil {
//	    if sdk.IsRetryable(err) {
//	        // Maybe implement custom retry logic
//	        time.Sleep(time.Second)
//	        err = client.Set(ctx, "key", value)
//	    }
//	    if err != nil {
//	        return err
//	    }
//	}
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific retryable errors
	if errors.Is(err, ErrTimeout) || errors.Is(err, ErrServerError) || errors.Is(err, ErrRateLimited) {
		return true
	}

	// Check enhanced errors
	var enhancedErr *Error
	if errors.As(err, &enhancedErr) {
		return enhancedErr.IsRetryable()
	}

	// Check API errors
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable()
	}

	// Check network errors
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return netErr.IsRetryable()
	}

	// Check timeout errors
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return timeoutErr.IsRetryable()
	}

	return false
}

// WrapError wraps an error with additional context and type information.
// If the error is already an enhanced Error, it updates the message.
// Otherwise, it creates a new Error with the specified type and message.
//
// Example:
//
//	err := someOperation()
//	if err != nil {
//	    return sdk.WrapError(err, sdk.ErrorTypeNetwork,
//	        "failed to connect to cache service")
//	}
func WrapError(err error, errType ErrorType, message string) *Error {
	if err == nil {
		return nil
	}

	// If it's already an enhanced error, just update it
	var enhancedErr *Error
	if errors.As(err, &enhancedErr) {
		enhancedErr.Message = message
		return enhancedErr
	}

	// Create new enhanced error
	return NewError(errType, message, err)
}
