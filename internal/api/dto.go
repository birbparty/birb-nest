package api

import (
	"encoding/json"
	"time"
)

// CacheRequest represents the request body for cache operations
type CacheRequest struct {
	Value    json.RawMessage        `json:"value" validate:"required"`
	TTL      *int                   `json:"ttl,omitempty" validate:"omitempty,min=1"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CacheResponse represents the response for cache operations
type CacheResponse struct {
	Key       string                 `json:"key"`
	Value     json.RawMessage        `json:"value"`
	Version   int                    `json:"version"`
	TTL       *int                   `json:"ttl,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status   string            `json:"status"`
	Service  string            `json:"service"`
	Version  string            `json:"version"`
	Uptime   string            `json:"uptime"`
	Checks   map[string]string `json:"checks"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// MetricsResponse represents cache metrics
type MetricsResponse struct {
	CacheHits        int64   `json:"cache_hits"`
	CacheMisses      int64   `json:"cache_misses"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	TotalRequests    int64   `json:"total_requests"`
	TotalErrors      int64   `json:"total_errors"`
	AverageLatencyMs float64 `json:"average_latency_ms"`
}

// BatchGetRequest represents a request to get multiple cache entries
type BatchGetRequest struct {
	Keys []string `json:"keys" validate:"required,min=1,max=100"`
}

// BatchGetResponse represents the response for batch get operations
type BatchGetResponse struct {
	Entries map[string]*CacheResponse `json:"entries"`
	Missing []string                  `json:"missing"`
}

// BatchSetRequest represents a request to set multiple cache entries
type BatchSetRequest struct {
	Entries map[string]CacheRequest `json:"entries" validate:"required,min=1,max=100"`
}

// BatchSetResponse represents the response for batch set operations
type BatchSetResponse struct {
	Success []string          `json:"success"`
	Failed  map[string]string `json:"failed"`
}

// ListKeysRequest represents a request to list cache keys
type ListKeysRequest struct {
	Pattern string `json:"pattern,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	Limit   int    `json:"limit,omitempty" validate:"omitempty,min=1,max=1000"`
}

// ListKeysResponse represents the response for list keys operations
type ListKeysResponse struct {
	Keys       []string `json:"keys"`
	TotalCount int      `json:"total_count"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
}

// Error codes
const (
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeInvalidRequest  = "INVALID_REQUEST"
	ErrCodeInternalError   = "INTERNAL_ERROR"
	ErrCodeVersionMismatch = "VERSION_MISMATCH"
	ErrCodeTimeout         = "TIMEOUT"
	ErrCodeRateLimited     = "RATE_LIMITED"
)

// NewErrorResponse creates a new error response
func NewErrorResponse(err string, code string) *ErrorResponse {
	return &ErrorResponse{
		Error: err,
		Code:  code,
	}
}

// NewErrorResponseWithDetails creates a new error response with details
func NewErrorResponseWithDetails(err string, code string, details string) *ErrorResponse {
	return &ErrorResponse{
		Error:   err,
		Code:    code,
		Details: details,
	}
}

// ConvertToCacheResponse converts internal models to API response
func ConvertToCacheResponse(key string, value json.RawMessage, version int, ttl *int, metadata json.RawMessage, createdAt, updatedAt time.Time) *CacheResponse {
	resp := &CacheResponse{
		Key:       key,
		Value:     value,
		Version:   version,
		TTL:       ttl,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	// Parse metadata if present
	if metadata != nil && len(metadata) > 0 {
		var meta map[string]interface{}
		if err := json.Unmarshal(metadata, &meta); err == nil {
			resp.Metadata = meta
		}
	}

	return resp
}
