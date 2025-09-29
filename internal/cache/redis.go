package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache interface using Redis
type RedisCache struct {
	client *redis.Client
	config *Config
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(config *Config) (*RedisCache, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:            config.Address(),
		Password:        config.Password,
		DB:              config.DB,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.MinRetryBackoff,
		MaxRetryBackoff: config.MaxRetryBackoff,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		ConnMaxIdleTime: config.MaxIdleTime,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		config: config,
	}, nil
}

// Get retrieves a value from the cache
func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrKeyNotFound
		}
		return nil, NewCacheError("failed to get key", true).WithError(err)
	}
	return val, nil
}

// Set stores a value in the cache with optional TTL
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Use default TTL if not specified
	if ttl == 0 {
		ttl = r.config.DefaultTTL
	}

	err := r.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return NewCacheError("failed to set key", true).WithError(err)
	}
	return nil
}

// Delete removes a value from the cache
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	result := r.client.Del(ctx, key)
	if err := result.Err(); err != nil {
		return NewCacheError("failed to delete key", true).WithError(err)
	}

	// Check if key existed
	if result.Val() == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// Exists checks if a key exists in the cache
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result := r.client.Exists(ctx, key)
	if err := result.Err(); err != nil {
		return false, NewCacheError("failed to check existence", true).WithError(err)
	}
	return result.Val() > 0, nil
}

// GetMultiple retrieves multiple values from the cache
func (r *RedisCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// Use MGET for batch retrieval
	values, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, NewCacheError("failed to get multiple keys", true).WithError(err)
	}

	result := make(map[string][]byte)
	for i, val := range values {
		if val != nil {
			// Type assertion to string then convert to bytes
			if strVal, ok := val.(string); ok {
				result[keys[i]] = []byte(strVal)
			}
		}
	}

	return result, nil
}

// SetMultiple stores multiple values in the cache
func (r *RedisCache) SetMultiple(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	// Use pipeline for batch operations
	pipe := r.client.Pipeline()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = r.config.DefaultTTL
	}

	for key, value := range items {
		pipe.Set(ctx, key, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return NewCacheError("failed to set multiple keys", true).WithError(err)
	}

	return nil
}

// DeleteMultiple removes multiple values from the cache
func (r *RedisCache) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	result := r.client.Del(ctx, keys...)
	if err := result.Err(); err != nil {
		return NewCacheError("failed to delete multiple keys", true).WithError(err)
	}

	return nil
}

// Ping checks if the cache is healthy
func (r *RedisCache) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return NewCacheError("ping failed", false).WithError(err)
	}
	return nil
}

// Close closes the cache connection
func (r *RedisCache) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Stats returns Redis connection pool stats
func (r *RedisCache) Stats() *redis.PoolStats {
	if r.client != nil {
		return r.client.PoolStats()
	}
	return nil
}

// FlushDB flushes the current database (use with caution!)
func (r *RedisCache) FlushDB(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// TTL returns the remaining time to live of a key
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, NewCacheError("failed to get TTL", true).WithError(err)
	}

	// Key doesn't exist
	if ttl == -2 {
		return 0, ErrKeyNotFound
	}

	// Key exists but has no TTL
	if ttl == -1 {
		return 0, nil
	}

	return ttl, nil
}

// Expire sets a new expiration time for a key
func (r *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	ok, err := r.client.Expire(ctx, key, ttl).Result()
	if err != nil {
		return NewCacheError("failed to set expiration", true).WithError(err)
	}

	if !ok {
		return ErrKeyNotFound
	}

	return nil
}
