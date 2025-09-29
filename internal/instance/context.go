// Package instance provides instance-aware context management for multi-tenant isolation
package instance

import (
	"context"
	"encoding/json"
	"time"
)

// InstanceStatus represents the current state of an instance
type InstanceStatus string

const (
	// StatusActive indicates the instance is running and accepting requests
	StatusActive InstanceStatus = "active"
	// StatusInactive indicates the instance is not currently running
	StatusInactive InstanceStatus = "inactive"
	// StatusMigrating indicates the instance is being migrated
	StatusMigrating InstanceStatus = "migrating"
	// StatusDeleting indicates the instance is being deleted
	StatusDeleting InstanceStatus = "deleting"
	// StatusPaused indicates the instance is temporarily paused
	StatusPaused InstanceStatus = "paused"
)

// Instance type constants
const (
	InstanceTypeOverworld = "overworld"
	InstanceTypeDungeon   = "dungeon"
	InstanceTypeTemporary = "temporary"
)

// ResourceQuota defines resource limits for an instance
type ResourceQuota struct {
	MaxMemoryMB   int64 `json:"max_memory_mb"`
	MaxStorageGB  int64 `json:"max_storage_gb"`
	MaxCPUCores   int   `json:"max_cpu_cores"`
	MaxConcurrent int   `json:"max_concurrent_connections"`
}

// Context contains complete instance information including metadata and resource limits
type Context struct {
	InstanceID    string            `json:"instance_id"`
	GameType      string            `json:"game_type"`
	Region        string            `json:"region"`
	CreatedAt     time.Time         `json:"created_at"`
	LastActive    time.Time         `json:"last_active"`
	Status        InstanceStatus    `json:"status"`
	Metadata      map[string]string `json:"metadata"`
	ResourceQuota *ResourceQuota    `json:"resource_quota,omitempty"`
	IsPermanent   bool              `json:"is_permanent"`
}

// DefaultResourceQuota returns generous default resource limits
func DefaultResourceQuota() *ResourceQuota {
	return &ResourceQuota{
		MaxMemoryMB:   8192,  // 8GB memory
		MaxStorageGB:  100,   // 100GB storage
		MaxCPUCores:   4,     // 4 CPU cores
		MaxConcurrent: 10000, // 10k concurrent connections
	}
}

// NewContext creates a new instance context with defaults
func NewContext(instanceID string) *Context {
	now := time.Now()
	return &Context{
		InstanceID:    instanceID,
		GameType:      "default",
		Region:        "default",
		CreatedAt:     now,
		LastActive:    now,
		Status:        StatusActive,
		Metadata:      make(map[string]string),
		ResourceQuota: DefaultResourceQuota(),
	}
}

// contextKey is used for storing instance context in context.Context
type contextKey struct{}

// InjectContext injects an instance context into a Go context
func InjectContext(ctx context.Context, instCtx *Context) context.Context {
	return context.WithValue(ctx, contextKey{}, instCtx)
}

// ExtractContext extracts an instance context from a Go context
func ExtractContext(ctx context.Context) (*Context, bool) {
	instCtx, ok := ctx.Value(contextKey{}).(*Context)
	return instCtx, ok
}

// ExtractInstanceID is a convenience function to get just the instance ID from context
func ExtractInstanceID(ctx context.Context) string {
	if instCtx, ok := ExtractContext(ctx); ok {
		return instCtx.InstanceID
	}
	return ""
}

// IsActive returns true if the instance is in active status
func (c *Context) IsActive() bool {
	return c.Status == StatusActive
}

// CanAcceptRequests returns true if the instance can process requests
func (c *Context) CanAcceptRequests() bool {
	return c.Status == StatusActive || c.Status == StatusMigrating
}

// CanBeAutoDeleted returns true if the instance can be automatically deleted
func (c *Context) CanBeAutoDeleted() bool {
	return !c.IsPermanent &&
		c.Status == StatusActive &&
		time.Since(c.CreatedAt) >= 30*time.Minute
}

// UpdateLastActive updates the last active timestamp to now
func (c *Context) UpdateLastActive() {
	c.LastActive = time.Now()
}

// Clone creates a deep copy of the context
func (c *Context) Clone() *Context {
	clone := &Context{
		InstanceID:  c.InstanceID,
		GameType:    c.GameType,
		Region:      c.Region,
		CreatedAt:   c.CreatedAt,
		LastActive:  c.LastActive,
		Status:      c.Status,
		Metadata:    make(map[string]string),
		IsPermanent: c.IsPermanent,
	}

	// Deep copy metadata
	for k, v := range c.Metadata {
		clone.Metadata[k] = v
	}

	// Deep copy resource quota
	if c.ResourceQuota != nil {
		clone.ResourceQuota = &ResourceQuota{
			MaxMemoryMB:   c.ResourceQuota.MaxMemoryMB,
			MaxStorageGB:  c.ResourceQuota.MaxStorageGB,
			MaxCPUCores:   c.ResourceQuota.MaxCPUCores,
			MaxConcurrent: c.ResourceQuota.MaxConcurrent,
		}
	}

	return clone
}

// MarshalBinary implements encoding.BinaryMarshaler for Redis storage
func (c *Context) MarshalBinary() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for Redis storage
func (c *Context) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, c)
}

// Validate performs basic validation on the context
func (c *Context) Validate() error {
	if c.InstanceID == "" {
		return ErrEmptyInstanceID
	}
	return nil
}

// Errors for instance context operations
var (
	ErrEmptyInstanceID     = &InstanceError{Code: "EMPTY_INSTANCE_ID", Message: "instance ID cannot be empty"}
	ErrInstanceNotFound    = &InstanceError{Code: "INSTANCE_NOT_FOUND", Message: "instance not found"}
	ErrInstanceNotActive   = &InstanceError{Code: "INSTANCE_NOT_ACTIVE", Message: "instance is not active"}
	ErrInvalidInstanceData = &InstanceError{Code: "INVALID_INSTANCE_DATA", Message: "invalid instance data"}
)

// InstanceError represents an instance-related error
type InstanceError struct {
	Code    string
	Message string
}

// Error implements the error interface
func (e *InstanceError) Error() string {
	return e.Message
}
