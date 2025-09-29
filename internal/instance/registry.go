package instance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// RegistryKeyPrefix is the prefix for all registry keys in Redis
	RegistryKeyPrefix = "registry:instance"
	// DefaultTTL is the default TTL for instance entries in Redis
	DefaultTTL = 24 * time.Hour
	// CacheTTL is the TTL for in-memory cache
	CacheTTL = 5 * time.Minute
	// ActivityUpdateInterval is the minimum interval between activity updates
	ActivityUpdateInterval = 1 * time.Minute
)

// CacheInterface defines the minimal cache operations needed by Registry
type CacheInterface interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Registry manages instance metadata storage and retrieval
type Registry struct {
	cache          CacheInterface
	memCache       map[string]*cacheEntry
	memCacheMu     sync.RWMutex
	lastActivityMu sync.Mutex
	lastActivity   map[string]time.Time
}

// cacheEntry represents an in-memory cached instance
type cacheEntry struct {
	context    *Context
	expiration time.Time
}

// NewRegistry creates a new instance registry
func NewRegistry(cacheClient CacheInterface) *Registry {
	return &Registry{
		cache:        cacheClient,
		memCache:     make(map[string]*cacheEntry),
		lastActivity: make(map[string]time.Time),
	}
}

// buildKey constructs a Redis key for an instance
func (r *Registry) buildKey(instanceID string) string {
	return fmt.Sprintf("%s:%s", RegistryKeyPrefix, instanceID)
}

// Register creates or updates an instance in the registry
func (r *Registry) Register(ctx context.Context, instCtx *Context) error {
	if err := instCtx.Validate(); err != nil {
		return err
	}

	// Update last active time
	instCtx.UpdateLastActive()

	// Serialize context
	data, err := instCtx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to serialize context: %w", err)
	}

	// Store in Redis with TTL
	key := r.buildKey(instCtx.InstanceID)
	if err := r.cache.Set(ctx, key, data, DefaultTTL); err != nil {
		return fmt.Errorf("failed to store in Redis: %w", err)
	}

	// Update memory cache
	r.updateMemCache(instCtx)

	return nil
}

// Get retrieves an instance from the registry
func (r *Registry) Get(ctx context.Context, instanceID string) (*Context, error) {
	if instanceID == "" {
		return nil, ErrEmptyInstanceID
	}

	// Check memory cache first
	if cached := r.getFromMemCache(instanceID); cached != nil {
		return cached, nil
	}

	// Fetch from Redis
	key := r.buildKey(instanceID)
	data, err := r.cache.Get(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "nil") {
			return nil, ErrInstanceNotFound
		}
		return nil, fmt.Errorf("failed to get from Redis: %w", err)
	}

	// Deserialize context
	instCtx := &Context{}
	if err := instCtx.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize context: %w", err)
	}

	// Refresh TTL in Redis
	go r.refreshTTL(context.Background(), instanceID)

	// Update memory cache
	r.updateMemCache(instCtx)

	return instCtx, nil
}

// GetOrCreate retrieves an instance or creates a new one if not found
func (r *Registry) GetOrCreate(ctx context.Context, instanceID string) (*Context, error) {
	if instanceID == "" {
		return nil, ErrEmptyInstanceID
	}

	// Try to get existing
	instCtx, err := r.Get(ctx, instanceID)
	if err == nil {
		return instCtx, nil
	}

	// If not found, create new
	if err == ErrInstanceNotFound {
		instCtx = NewContext(instanceID)
		if err := r.Register(ctx, instCtx); err != nil {
			return nil, fmt.Errorf("failed to register new instance: %w", err)
		}
		return instCtx, nil
	}

	return nil, err
}

// Update updates an existing instance in the registry
func (r *Registry) Update(ctx context.Context, instCtx *Context) error {
	if err := instCtx.Validate(); err != nil {
		return err
	}

	// Check if instance exists
	existing, err := r.Get(ctx, instCtx.InstanceID)
	if err != nil {
		return err
	}

	// Preserve created_at from existing
	instCtx.CreatedAt = existing.CreatedAt

	// Update and save
	return r.Register(ctx, instCtx)
}

// Delete removes an instance from the registry
func (r *Registry) Delete(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return ErrEmptyInstanceID
	}

	// Remove from Redis
	key := r.buildKey(instanceID)
	if err := r.cache.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}

	// Remove from memory cache
	r.memCacheMu.Lock()
	delete(r.memCache, instanceID)
	r.memCacheMu.Unlock()

	// Remove from activity tracking
	r.lastActivityMu.Lock()
	delete(r.lastActivity, instanceID)
	r.lastActivityMu.Unlock()

	return nil
}

// UpdateLastActive updates the last active timestamp for an instance
func (r *Registry) UpdateLastActive(ctx context.Context, instanceID string) error {
	// Check if we should throttle the update
	if !r.shouldUpdateActivity(instanceID) {
		return nil
	}

	// Get current context
	instCtx, err := r.Get(ctx, instanceID)
	if err != nil {
		return err
	}

	// Update last active
	instCtx.UpdateLastActive()

	// Save back to registry
	return r.Register(ctx, instCtx)
}

// List retrieves all instances matching the filter criteria
func (r *Registry) List(ctx context.Context, filter ListFilter) ([]*Context, error) {
	// This is a simplified implementation
	// In production, you might want to use Redis SCAN or maintain indices

	pattern := fmt.Sprintf("%s:*", RegistryKeyPrefix)
	keys, err := r.scanKeys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	var instances []*Context
	for _, key := range keys {
		instanceID := strings.TrimPrefix(key, RegistryKeyPrefix+":")
		instCtx, err := r.Get(ctx, instanceID)
		if err != nil {
			continue // Skip failed instances
		}

		// Apply filters
		if filter.Status != "" && instCtx.Status != filter.Status {
			continue
		}
		if filter.Region != "" && instCtx.Region != filter.Region {
			continue
		}
		if filter.GameType != "" && instCtx.GameType != filter.GameType {
			continue
		}

		instances = append(instances, instCtx)
	}

	return instances, nil
}

// ListFilter defines criteria for filtering instances
type ListFilter struct {
	Status   InstanceStatus
	Region   string
	GameType string
}

// Memory cache management

func (r *Registry) getFromMemCache(instanceID string) *Context {
	r.memCacheMu.RLock()
	entry, exists := r.memCache[instanceID]
	r.memCacheMu.RUnlock()

	if !exists || entry.expiration.Before(time.Now()) {
		return nil
	}

	// Return a clone to prevent external modification
	return entry.context.Clone()
}

func (r *Registry) updateMemCache(instCtx *Context) {
	r.memCacheMu.Lock()
	r.memCache[instCtx.InstanceID] = &cacheEntry{
		context:    instCtx.Clone(),
		expiration: time.Now().Add(CacheTTL),
	}
	r.memCacheMu.Unlock()
}

// Activity throttling

func (r *Registry) shouldUpdateActivity(instanceID string) bool {
	r.lastActivityMu.Lock()
	defer r.lastActivityMu.Unlock()

	lastUpdate, exists := r.lastActivity[instanceID]
	if !exists || time.Since(lastUpdate) >= ActivityUpdateInterval {
		r.lastActivity[instanceID] = time.Now()
		return true
	}

	return false
}

// Redis operations

func (r *Registry) refreshTTL(ctx context.Context, instanceID string) {
	key := r.buildKey(instanceID)
	data, err := r.cache.Get(ctx, key)
	if err != nil {
		return
	}

	// Re-set with new TTL
	r.cache.Set(ctx, key, data, DefaultTTL)
}

func (r *Registry) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	// This is a placeholder implementation
	// In a real implementation, you would use Redis SCAN command
	// For now, we'll return an empty slice
	// The actual SCAN implementation would be in the Redis client
	return []string{}, nil
}

// Stats returns registry statistics
func (r *Registry) Stats() map[string]interface{} {
	r.memCacheMu.RLock()
	memCacheSize := len(r.memCache)
	r.memCacheMu.RUnlock()

	r.lastActivityMu.Lock()
	activityTrackingSize := len(r.lastActivity)
	r.lastActivityMu.Unlock()

	return map[string]interface{}{
		"memory_cache_size":      memCacheSize,
		"activity_tracking_size": activityTrackingSize,
	}
}

// ClearMemoryCache clears the in-memory cache
func (r *Registry) ClearMemoryCache() {
	r.memCacheMu.Lock()
	r.memCache = make(map[string]*cacheEntry)
	r.memCacheMu.Unlock()
}
