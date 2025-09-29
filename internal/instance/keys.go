// Package instance provides instance-aware key management for multi-tenant isolation
package instance

import (
	"fmt"
	"strings"
)

const (
	// Separator is the delimiter used in key construction
	Separator = ":"
	// Prefix is the namespace prefix for instance-scoped keys
	Prefix = "instance"
)

// KeyBuilder handles instance-aware key generation and parsing
type KeyBuilder struct {
	instanceID string
}

// NewKeyBuilder creates a new KeyBuilder for the given instance ID
func NewKeyBuilder(instanceID string) *KeyBuilder {
	return &KeyBuilder{
		instanceID: strings.TrimSpace(instanceID),
	}
}

// BuildKey constructs an instance-aware key from components
// Format: instance:{instance_id}:{component}:{identifiers}
// If instanceID is empty, returns components joined without instance prefix (backward compatibility)
func (kb *KeyBuilder) BuildKey(components ...string) string {
	if kb.instanceID == "" {
		// Backward compatibility: no prefix for empty instance
		return strings.Join(components, Separator)
	}

	// Build instance-prefixed key
	parts := []string{Prefix, kb.instanceID}
	parts = append(parts, components...)
	return strings.Join(parts, Separator)
}

// ParseKey extracts the instance ID and components from a key
// Returns empty instanceID if the key is not instance-prefixed
func (kb *KeyBuilder) ParseKey(key string) (instanceID string, components []string) {
	parts := strings.Split(key, Separator)

	// Check if key has instance prefix
	if len(parts) >= 2 && parts[0] == Prefix {
		instanceID = parts[1]
		if len(parts) > 2 {
			components = parts[2:]
		}
	} else {
		// Non-instance key
		components = parts
	}

	return instanceID, components
}

// HasInstance returns true if the KeyBuilder has a non-empty instance ID
func (kb *KeyBuilder) HasInstance() bool {
	return kb.instanceID != ""
}

// InstanceID returns the instance ID
func (kb *KeyBuilder) InstanceID() string {
	return kb.instanceID
}

// BuildPattern creates a pattern for scanning keys by instance
// Returns pattern like "instance:inst_123:*" or "*" for empty instance
func (kb *KeyBuilder) BuildPattern(prefix string) string {
	if kb.instanceID == "" {
		if prefix == "" {
			return "*"
		}
		return fmt.Sprintf("%s*", prefix)
	}

	basePattern := fmt.Sprintf("%s:%s", Prefix, kb.instanceID)
	if prefix != "" {
		return fmt.Sprintf("%s:%s*", basePattern, prefix)
	}
	return fmt.Sprintf("%s:*", basePattern)
}

// IsInstanceKey checks if a key belongs to this instance
func (kb *KeyBuilder) IsInstanceKey(key string) bool {
	if kb.instanceID == "" {
		// Empty instance accepts all non-instance keys
		return !strings.HasPrefix(key, Prefix+Separator)
	}

	expectedPrefix := fmt.Sprintf("%s:%s:", Prefix, kb.instanceID)
	return strings.HasPrefix(key, expectedPrefix)
}

// StripInstance removes the instance prefix from a key if present
func (kb *KeyBuilder) StripInstance(key string) string {
	if kb.instanceID == "" || !kb.IsInstanceKey(key) {
		return key
	}

	prefix := fmt.Sprintf("%s:%s:", Prefix, kb.instanceID)
	return strings.TrimPrefix(key, prefix)
}

// Common key component helpers

// TableKey builds a key for table data
func (kb *KeyBuilder) TableKey(table, rowID string) string {
	return kb.BuildKey("table", table, "row", rowID)
}

// IndexKey builds a key for index data
func (kb *KeyBuilder) IndexKey(index, value string) string {
	return kb.BuildKey("index", index, value)
}

// CacheKey builds a generic cache key
func (kb *KeyBuilder) CacheKey(components ...string) string {
	parts := []string{"cache"}
	parts = append(parts, components...)
	return kb.BuildKey(parts...)
}

// SchemaKey builds a key for schema metadata
func (kb *KeyBuilder) SchemaKey(table string) string {
	return kb.BuildKey("schema", table)
}

// EventLogKey builds a key for event log entries
func (kb *KeyBuilder) EventLogKey(timestamp string) string {
	return kb.BuildKey("eventlog", timestamp)
}
