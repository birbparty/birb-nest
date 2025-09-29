package database

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// CacheEntry represents a cache entry in the database
type CacheEntry struct {
	Key        string          `db:"key" json:"key"`
	Value      json.RawMessage `db:"value" json:"value"`
	InstanceID string          `db:"instance_id" json:"instance_id"`
	CreatedAt  time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at" json:"updated_at"`
	Version    int             `db:"version" json:"version"`
	TTL        *int            `db:"ttl" json:"ttl,omitempty"`
	Metadata   json.RawMessage `db:"metadata" json:"metadata"`
}

// DLQEntry represents a dead letter queue entry
type DLQEntry struct {
	ID          int             `db:"id" json:"id"`
	MessageID   string          `db:"message_id" json:"message_id"`
	Key         string          `db:"key" json:"key"`
	Value       json.RawMessage `db:"value" json:"value"`
	ErrorMsg    *string         `db:"error_message" json:"error_message,omitempty"`
	RetryCount  int             `db:"retry_count" json:"retry_count"`
	MaxRetries  int             `db:"max_retries" json:"max_retries"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	LastRetryAt time.Time       `db:"last_retry_at" json:"last_retry_at"`
	Status      string          `db:"status" json:"status"`
}

// CacheMetric represents a cache operation metric
type CacheMetric struct {
	ID         int             `db:"id" json:"id"`
	Timestamp  time.Time       `db:"timestamp" json:"timestamp"`
	Operation  string          `db:"operation" json:"operation"`
	Key        *string         `db:"key" json:"key,omitempty"`
	DurationMS *int            `db:"duration_ms" json:"duration_ms,omitempty"`
	Success    bool            `db:"success" json:"success"`
	ErrorMsg   *string         `db:"error_message" json:"error_message,omitempty"`
	Metadata   json.RawMessage `db:"metadata" json:"metadata"`
}

// JSONBMap is a helper type for JSONB columns
type JSONBMap map[string]interface{}

// Value implements driver.Valuer interface
func (j JSONBMap) Value() (driver.Value, error) {
	return json.Marshal(j)
}

// Scan implements sql.Scanner interface
func (j *JSONBMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONBMap)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}

// IsExpired checks if the cache entry is expired based on TTL
func (c *CacheEntry) IsExpired() bool {
	if c.TTL == nil || *c.TTL <= 0 {
		return false
	}

	expirationTime := c.UpdatedAt.Add(time.Duration(*c.TTL) * time.Second)
	return time.Now().After(expirationTime)
}

// DLQStatus constants
const (
	DLQStatusPending   = "pending"
	DLQStatusRetrying  = "retrying"
	DLQStatusFailed    = "failed"
	DLQStatusSucceeded = "succeeded"
)

// CacheOperation constants for metrics
const (
	OpGet    = "get"
	OpSet    = "set"
	OpDelete = "delete"
	OpExists = "exists"
)
