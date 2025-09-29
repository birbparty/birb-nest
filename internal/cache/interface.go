package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with optional TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// GetMultiple retrieves multiple values from the cache
	GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error)

	// SetMultiple stores multiple values in the cache
	SetMultiple(ctx context.Context, items map[string][]byte, ttl time.Duration) error

	// DeleteMultiple removes multiple values from the cache
	DeleteMultiple(ctx context.Context, keys []string) error

	// Ping checks if the cache is healthy
	Ping(ctx context.Context) error

	// Close closes the cache connection
	Close() error
}

// Common errors
var (
	ErrKeyNotFound = NewCacheError("key not found", true)
	ErrCacheClosed = NewCacheError("cache is closed", false)
)

// CacheError represents a cache-specific error
type CacheError struct {
	Message    string
	Retryable  bool
	Underlying error
}

// NewCacheError creates a new cache error
func NewCacheError(message string, retryable bool) *CacheError {
	return &CacheError{
		Message:   message,
		Retryable: retryable,
	}
}

// Error implements the error interface
func (e *CacheError) Error() string {
	if e.Underlying != nil {
		return e.Message + ": " + e.Underlying.Error()
	}
	return e.Message
}

// WithError adds an underlying error
func (e *CacheError) WithError(err error) *CacheError {
	e.Underlying = err
	return e
}

// IsRetryable returns whether the error is retryable
func (e *CacheError) IsRetryable() bool {
	return e.Retryable
}
