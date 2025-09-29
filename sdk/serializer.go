package sdk

import (
	"encoding/json"
	"fmt"
	"time"
)

// CacheRequest represents the request body for cache operations.
// It contains the value to be cached along with optional TTL and metadata.
//
// The SDK automatically handles serialization of your Go values into this format,
// so you typically don't need to create these directly.
//
// Example of what gets sent to the API:
//
//	{
//	    "value": {"name": "Alice", "age": 30},
//	    "ttl": 3600,
//	    "metadata": {
//	        "source": "api",
//	        "version": 2
//	    }
//	}
type CacheRequest struct {
	// Value is the JSON-encoded value to store
	Value json.RawMessage `json:"value"`
	// TTL is the time-to-live in seconds (optional)
	TTL *int `json:"ttl,omitempty"`
	// Metadata is additional metadata to store with the entry
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CacheResponse represents the response from cache operations.
// It contains the cached value along with metadata about when it was
// created, updated, and when it will expire.
//
// The SDK automatically deserializes the Value field into your target type,
// so you typically work with your Go types rather than this struct directly.
//
// Example response from the API:
//
//	{
//	    "key": "user:123",
//	    "value": {"name": "Alice", "age": 30},
//	    "version": 1,
//	    "ttl": 3600,
//	    "metadata": {"source": "api"},
//	    "created_at": "2024-01-01T12:00:00Z",
//	    "updated_at": "2024-01-01T12:00:00Z"
//	}
type CacheResponse struct {
	// Key is the cache key
	Key string `json:"key"`
	// Value is the JSON-encoded cached value
	Value json.RawMessage `json:"value"`
	// Version is the version number of this cache entry
	Version int `json:"version"`
	// TTL is the remaining time-to-live in seconds (optional)
	TTL *int `json:"ttl,omitempty"`
	// Metadata is additional metadata stored with the entry
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// CreatedAt is when the entry was first created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the entry was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// HealthResponse represents the health check response from the Birb Nest API.
// It provides information about the service status and various health checks.
//
// This is used by the Ping() method to verify connectivity and service health.
//
// Example response:
//
//	{
//	    "status": "healthy",
//	    "service": "birb-nest",
//	    "version": "1.0.0",
//	    "uptime": "72h15m30s",
//	    "checks": {
//	        "database": "healthy",
//	        "cache": "healthy",
//	        "queue": "healthy"
//	    }
//	}
type HealthResponse struct {
	// Status is the overall health status ("healthy" or "unhealthy")
	Status string `json:"status"`
	// Service is the service name
	Service string `json:"service"`
	// Version is the service version
	Version string `json:"version"`
	// Uptime is the service uptime as a string
	Uptime string `json:"uptime"`
	// Checks contains individual component health statuses
	Checks map[string]string `json:"checks"`
}

// serialize converts any Go value to json.RawMessage for storage.
// This function handles various input types and ensures they can be
// properly stored in the cache.
//
// Special handling:
//   - json.RawMessage: Passed through as-is
//   - strings: If valid JSON, stored as JSON; otherwise as string
//   - All other types: Marshaled to JSON
//
// The function validates that values are serializable before attempting
// to marshal them, providing better error messages.
//
// Example:
//
//	user := User{Name: "Alice", Age: 30}
//	raw, err := serialize(user)
//	// raw contains: {"name":"Alice","age":30}
//
//	jsonStr := `{"key": "value"}`
//	raw, err = serialize(jsonStr)
//	// raw contains: {"key": "value"} (stored as JSON, not string)
func serialize(value interface{}) (json.RawMessage, error) {
	// If it's already json.RawMessage, return as is
	if raw, ok := value.(json.RawMessage); ok {
		return raw, nil
	}

	// If it's a string, try to parse it as JSON first
	if str, ok := value.(string); ok {
		// Check if it's valid JSON
		var temp interface{}
		if err := json.Unmarshal([]byte(str), &temp); err == nil {
			// It's valid JSON, return as raw message
			return json.RawMessage(str), nil
		}
		// Not valid JSON, treat as string value
		value = str
	}

	// Validate that the value is serializable
	if err := validateSerializable(value); err != nil {
		return nil, fmt.Errorf("value is not serializable: %w", err)
	}

	// Marshal the value
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize value: %w", err)
	}

	return json.RawMessage(data), nil
}

// validateSerializable checks if a value can be serialized to JSON.
// This pre-validation helps provide better error messages than waiting
// for json.Marshal to fail.
//
// The following types are always considered serializable:
//   - Basic types: bool, string, numeric types
//   - time.Time (serializes to RFC3339 format)
//   - []byte (serializes to base64)
//   - json.RawMessage
//   - nil
//
// For other types (structs, slices, maps), it attempts a test marshal
// to verify serializability.
//
// Example:
//
//	err := validateSerializable(User{Name: "Alice"}) // nil
//	err = validateSerializable(make(chan int))       // error
func validateSerializable(value interface{}) error {
	if value == nil {
		return nil
	}

	// Common types that are always serializable
	switch value.(type) {
	case bool, string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, time.Time,
		[]byte, json.RawMessage:
		return nil
	}

	// For other types, try to marshal to check
	_, err := json.Marshal(value)
	return err
}

// deserialize converts json.RawMessage to the target type.
// The target must be a pointer to the desired type.
//
// Special handling:
//   - *json.RawMessage: Direct assignment
//   - All other types: JSON unmarshal
//
// This function is used internally by Get operations to convert
// the cached JSON data back into Go types.
//
// Example:
//
//	var user User
//	raw := json.RawMessage(`{"name":"Alice","age":30}`)
//	err := deserialize(raw, &user)
//	// user now contains: User{Name: "Alice", Age: 30}
//
//	var str string
//	raw = json.RawMessage(`"hello world"`)
//	err = deserialize(raw, &str)
//	// str now contains: "hello world"
func deserialize(data json.RawMessage, target interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	// If target is *json.RawMessage, just assign
	if raw, ok := target.(*json.RawMessage); ok {
		*raw = data
		return nil
	}

	// Otherwise unmarshal
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to deserialize value: %w", err)
	}

	return nil
}

// buildCacheRequest creates a CacheRequest from a Go value.
// This function handles the serialization of the value and converts
// the TTL from time.Duration to seconds.
//
// Parameters:
//   - value: The value to cache (must be JSON-serializable)
//   - ttl: Optional time-to-live duration
//   - metadata: Optional metadata to store with the cache entry
//
// Example:
//
//	user := User{Name: "Alice", Age: 30}
//	ttl := 5 * time.Minute
//	metadata := map[string]interface{}{
//	    "source": "api",
//	    "version": 2,
//	}
//
//	req, err := buildCacheRequest(user, &ttl, metadata)
//	// req.Value contains serialized user
//	// req.TTL contains 300 (seconds)
//	// req.Metadata contains the metadata map
func buildCacheRequest(value interface{}, ttl *time.Duration, metadata map[string]interface{}) (*CacheRequest, error) {
	serialized, err := serialize(value)
	if err != nil {
		return nil, err
	}

	req := &CacheRequest{
		Value:    serialized,
		Metadata: metadata,
	}

	// Convert duration to seconds if provided
	if ttl != nil {
		seconds := int(ttl.Seconds())
		req.TTL = &seconds
	}

	return req, nil
}

// parseAPIError parses an API error response into an APIError.
// This function attempts to parse the error body as JSON, but falls
// back to using the raw body as the error message if parsing fails.
//
// The function ensures that APIError always has a StatusCode set,
// even if the response body is empty or malformed.
//
// Example responses it handles:
//
//	// Well-formed API error
//	{"error": "Key not found", "code": "NOT_FOUND"}
//
//	// Plain text error
//	"Internal server error"
//
//	// Empty response
//	(empty body results in "HTTP 500 error")
func parseAPIError(statusCode int, body []byte) error {
	if len(body) == 0 {
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("HTTP %d error", statusCode),
		}
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// If we can't parse the error, return a generic one
		return &APIError{
			StatusCode: statusCode,
			Message:    string(body),
		}
	}

	apiErr.StatusCode = statusCode
	return &apiErr
}
