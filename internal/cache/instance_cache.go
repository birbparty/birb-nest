package cache

import (
	"context"
	"time"

	"github.com/birbparty/birb-nest/internal/instance"
)

// InstanceCache wraps a cache implementation to provide instance-aware key management
type InstanceCache struct {
	client     Cache
	keyBuilder *instance.KeyBuilder
}

// NewInstanceCache creates a new instance-aware cache wrapper
func NewInstanceCache(client Cache, instanceID string) *InstanceCache {
	return &InstanceCache{
		client:     client,
		keyBuilder: instance.NewKeyBuilder(instanceID),
	}
}

// Get retrieves a value from the cache using instance-aware key
func (ic *InstanceCache) Get(ctx context.Context, key string) ([]byte, error) {
	instanceKey := ic.keyBuilder.CacheKey(key)
	return ic.client.Get(ctx, instanceKey)
}

// Set stores a value in the cache with instance-aware key
func (ic *InstanceCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	instanceKey := ic.keyBuilder.CacheKey(key)
	return ic.client.Set(ctx, instanceKey, value, ttl)
}

// Delete removes a value from the cache using instance-aware key
func (ic *InstanceCache) Delete(ctx context.Context, key string) error {
	instanceKey := ic.keyBuilder.CacheKey(key)
	return ic.client.Delete(ctx, instanceKey)
}

// Exists checks if a key exists in the cache using instance-aware key
func (ic *InstanceCache) Exists(ctx context.Context, key string) (bool, error) {
	instanceKey := ic.keyBuilder.CacheKey(key)
	return ic.client.Exists(ctx, instanceKey)
}

// GetMultiple retrieves multiple values from the cache using instance-aware keys
func (ic *InstanceCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	// Transform keys to instance-aware keys
	instanceKeys := make([]string, len(keys))
	keyMap := make(map[string]string) // map instance key -> original key
	for i, key := range keys {
		instanceKey := ic.keyBuilder.CacheKey(key)
		instanceKeys[i] = instanceKey
		keyMap[instanceKey] = key
	}

	// Get values from underlying cache
	results, err := ic.client.GetMultiple(ctx, instanceKeys)
	if err != nil {
		return nil, err
	}

	// Transform results back to original keys
	transformedResults := make(map[string][]byte)
	for instanceKey, value := range results {
		if originalKey, ok := keyMap[instanceKey]; ok {
			transformedResults[originalKey] = value
		}
	}

	return transformedResults, nil
}

// SetMultiple stores multiple values in the cache using instance-aware keys
func (ic *InstanceCache) SetMultiple(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	// Transform items to use instance-aware keys
	instanceItems := make(map[string][]byte)
	for key, value := range items {
		instanceKey := ic.keyBuilder.CacheKey(key)
		instanceItems[instanceKey] = value
	}

	return ic.client.SetMultiple(ctx, instanceItems, ttl)
}

// DeleteMultiple removes multiple values from the cache using instance-aware keys
func (ic *InstanceCache) DeleteMultiple(ctx context.Context, keys []string) error {
	// Transform keys to instance-aware keys
	instanceKeys := make([]string, len(keys))
	for i, key := range keys {
		instanceKeys[i] = ic.keyBuilder.CacheKey(key)
	}

	return ic.client.DeleteMultiple(ctx, instanceKeys)
}

// Ping checks if the cache is healthy
func (ic *InstanceCache) Ping(ctx context.Context) error {
	return ic.client.Ping(ctx)
}

// Close closes the cache connection
func (ic *InstanceCache) Close() error {
	return ic.client.Close()
}

// GetInstanceID returns the instance ID being used
func (ic *InstanceCache) GetInstanceID() string {
	return ic.keyBuilder.InstanceID()
}

// HasInstance returns true if this cache is instance-aware
func (ic *InstanceCache) HasInstance() bool {
	return ic.keyBuilder.HasInstance()
}

// KeyBuilder returns the underlying key builder for advanced usage
func (ic *InstanceCache) KeyBuilder() *instance.KeyBuilder {
	return ic.keyBuilder
}

// Scan performs a pattern-based scan for keys belonging to this instance
// The pattern parameter is applied after the instance prefix
func (ic *InstanceCache) Scan(ctx context.Context, pattern string, count int) ([]string, error) {
	// For Redis SCAN operations, we need to handle this at the Redis client level
	// This is a placeholder that would need to be implemented in the Redis client
	// For now, we'll just build the pattern
	scanPattern := ic.keyBuilder.BuildPattern(pattern)

	// This would need to be implemented in the underlying cache client
	// as it requires Redis-specific SCAN operations
	_ = scanPattern
	_ = count

	// Return empty for now - actual implementation would be in redis.go
	return []string{}, nil
}

// Context-aware methods that extract instance from context

// GetWithContext retrieves a value using instance ID from context
func (ic *InstanceCache) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		// Fall back to configured instance ID
		instanceID = ic.keyBuilder.InstanceID()
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return ic.client.Get(ctx, instanceKey)
}

// SetWithContext stores a value using instance ID from context
func (ic *InstanceCache) SetWithContext(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		// Fall back to configured instance ID
		instanceID = ic.keyBuilder.InstanceID()
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return ic.client.Set(ctx, instanceKey, value, ttl)
}

// DeleteWithContext removes a value using instance ID from context
func (ic *InstanceCache) DeleteWithContext(ctx context.Context, key string) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		// Fall back to configured instance ID
		instanceID = ic.keyBuilder.InstanceID()
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return ic.client.Delete(ctx, instanceKey)
}

// ExistsWithContext checks if a key exists using instance ID from context
func (ic *InstanceCache) ExistsWithContext(ctx context.Context, key string) (bool, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		// Fall back to configured instance ID
		instanceID = ic.keyBuilder.InstanceID()
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return ic.client.Exists(ctx, instanceKey)
}

// ContextCache provides a fully context-aware cache interface
type ContextCache struct {
	client Cache
}

// NewContextCache creates a new context-aware cache
func NewContextCache(client Cache) *ContextCache {
	return &ContextCache{
		client: client,
	}
}

// Get retrieves a value using instance ID from context
func (cc *ContextCache) Get(ctx context.Context, key string) ([]byte, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return cc.client.Get(ctx, instanceKey)
}

// Set stores a value using instance ID from context
func (cc *ContextCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return cc.client.Set(ctx, instanceKey, value, ttl)
}

// Delete removes a value using instance ID from context
func (cc *ContextCache) Delete(ctx context.Context, key string) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return cc.client.Delete(ctx, instanceKey)
}

// Exists checks if a key exists using instance ID from context
func (cc *ContextCache) Exists(ctx context.Context, key string) (bool, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)
	instanceKey := kb.CacheKey(key)
	return cc.client.Exists(ctx, instanceKey)
}

// Ping checks if the cache is healthy
func (cc *ContextCache) Ping(ctx context.Context) error {
	return cc.client.Ping(ctx)
}

// Close closes the cache connection
func (cc *ContextCache) Close() error {
	return cc.client.Close()
}

// GetMultiple retrieves multiple values using instance ID from context
func (cc *ContextCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)

	// Transform keys to instance-aware keys
	instanceKeys := make([]string, len(keys))
	keyMap := make(map[string]string) // map instance key -> original key
	for i, key := range keys {
		instanceKey := kb.CacheKey(key)
		instanceKeys[i] = instanceKey
		keyMap[instanceKey] = key
	}

	// Get values from underlying cache
	results, err := cc.client.GetMultiple(ctx, instanceKeys)
	if err != nil {
		return nil, err
	}

	// Transform results back to original keys
	transformedResults := make(map[string][]byte)
	for instanceKey, value := range results {
		if originalKey, ok := keyMap[instanceKey]; ok {
			transformedResults[originalKey] = value
		}
	}

	return transformedResults, nil
}

// SetMultiple stores multiple values using instance ID from context
func (cc *ContextCache) SetMultiple(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)

	// Transform items to use instance-aware keys
	instanceItems := make(map[string][]byte)
	for key, value := range items {
		instanceKey := kb.CacheKey(key)
		instanceItems[instanceKey] = value
	}

	return cc.client.SetMultiple(ctx, instanceItems, ttl)
}

// DeleteMultiple removes multiple values using instance ID from context
func (cc *ContextCache) DeleteMultiple(ctx context.Context, keys []string) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = "global" // Default to global instance
	}
	kb := instance.NewKeyBuilder(instanceID)

	// Transform keys to instance-aware keys
	instanceKeys := make([]string, len(keys))
	for i, key := range keys {
		instanceKeys[i] = kb.CacheKey(key)
	}

	return cc.client.DeleteMultiple(ctx, instanceKeys)
}
