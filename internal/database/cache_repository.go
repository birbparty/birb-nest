package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CacheRepository handles cache-related database operations
type CacheRepository struct {
	db *DB
}

// NewCacheRepository creates a new cache repository
func NewCacheRepository(db *DB) *CacheRepository {
	return &CacheRepository{db: db}
}

// Get retrieves a cache entry by key (backward compatibility)
func (r *CacheRepository) Get(ctx context.Context, key string) (*CacheEntry, error) {
	return r.GetWithInstance(ctx, key, "global")
}

// GetWithInstance retrieves a cache entry by key and instance
func (r *CacheRepository) GetWithInstance(ctx context.Context, key, instanceID string) (*CacheEntry, error) {
	query := `
		SELECT key, value, instance_id, created_at, updated_at, version, ttl, metadata
		FROM cache_entries
		WHERE key = $1 AND instance_id = $2
	`

	var entry CacheEntry
	err := r.db.QueryRow(ctx, query, key, instanceID).Scan(
		&entry.Key,
		&entry.Value,
		&entry.InstanceID,
		&entry.CreatedAt,
		&entry.UpdatedAt,
		&entry.Version,
		&entry.TTL,
		&entry.Metadata,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	// Check if entry is expired
	if entry.IsExpired() {
		// Optional: Delete expired entry
		_ = r.DeleteWithInstance(ctx, key, instanceID)
		return nil, ErrNotFound
	}

	return &entry, nil
}

// Set creates or updates a cache entry (backward compatibility)
func (r *CacheRepository) Set(ctx context.Context, key string, value json.RawMessage, ttl *int, metadata json.RawMessage) error {
	return r.SetWithInstance(ctx, key, "global", value, ttl, metadata)
}

// SetWithInstance creates or updates a cache entry with instance awareness
func (r *CacheRepository) SetWithInstance(ctx context.Context, key, instanceID string, value json.RawMessage, ttl *int, metadata json.RawMessage) error {
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	query := `
		INSERT INTO cache_entries (key, value, instance_id, ttl, metadata, version)
		VALUES ($1, $2, $3, $4, $5, 1)
		ON CONFLICT (instance_id, key) DO UPDATE SET
			value = EXCLUDED.value,
			ttl = EXCLUDED.ttl,
			metadata = EXCLUDED.metadata,
			updated_at = CURRENT_TIMESTAMP,
			version = cache_entries.version + 1
		RETURNING version
	`

	var version int
	err := r.db.QueryRow(ctx, query, key, value, instanceID, ttl, metadata).Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	return nil
}

// SetWithVersion creates or updates a cache entry with optimistic locking
func (r *CacheRepository) SetWithVersion(ctx context.Context, key string, value json.RawMessage, ttl *int, metadata json.RawMessage, expectedVersion int) error {
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	query := `
		UPDATE cache_entries
		SET value = $2, ttl = $3, metadata = $4, updated_at = CURRENT_TIMESTAMP
		WHERE key = $1 AND version = $5
		RETURNING version
	`

	var newVersion int
	err := r.db.QueryRow(ctx, query, key, value, ttl, metadata, expectedVersion).Scan(&newVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVersionMismatch
		}
		return fmt.Errorf("failed to update cache entry with version: %w", err)
	}

	return nil
}

// Delete removes a cache entry (backward compatibility)
func (r *CacheRepository) Delete(ctx context.Context, key string) error {
	return r.DeleteWithInstance(ctx, key, "global")
}

// DeleteWithInstance removes a cache entry with instance awareness
func (r *CacheRepository) DeleteWithInstance(ctx context.Context, key, instanceID string) error {
	query := `DELETE FROM cache_entries WHERE key = $1 AND instance_id = $2`

	result, err := r.db.Exec(ctx, query, key, instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Exists checks if a cache entry exists and is not expired (backward compatibility)
func (r *CacheRepository) Exists(ctx context.Context, key string) (bool, error) {
	return r.ExistsWithInstance(ctx, key, "global")
}

// ExistsWithInstance checks if a cache entry exists with instance awareness
func (r *CacheRepository) ExistsWithInstance(ctx context.Context, key, instanceID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM cache_entries 
			WHERE key = $1 AND instance_id = $2
			AND (ttl IS NULL OR updated_at + interval '1 second' * ttl > CURRENT_TIMESTAMP)
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, key, instanceID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check cache entry existence: %w", err)
	}

	return exists, nil
}

// BatchGet retrieves multiple cache entries
func (r *CacheRepository) BatchGet(ctx context.Context, keys []string) ([]*CacheEntry, error) {
	if len(keys) == 0 {
		return []*CacheEntry{}, nil
	}

	query := `
		SELECT key, value, instance_id, created_at, updated_at, version, ttl, metadata
		FROM cache_entries
		WHERE key = ANY($1)
	`

	rows, err := r.db.Query(ctx, query, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get cache entries: %w", err)
	}
	defer rows.Close()

	var entries []*CacheEntry
	for rows.Next() {
		var entry CacheEntry
		err := rows.Scan(
			&entry.Key,
			&entry.Value,
			&entry.InstanceID,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&entry.Version,
			&entry.TTL,
			&entry.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan cache entry: %w", err)
		}

		// Skip expired entries
		if !entry.IsExpired() {
			entries = append(entries, &entry)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cache entries: %w", err)
	}

	return entries, nil
}

// BatchGetWithInstance retrieves multiple cache entries with instance awareness
func (r *CacheRepository) BatchGetWithInstance(ctx context.Context, keys []string, instanceID string) ([]*CacheEntry, error) {
	if len(keys) == 0 {
		return []*CacheEntry{}, nil
	}

	query := `
		SELECT key, value, instance_id, created_at, updated_at, version, ttl, metadata
		FROM cache_entries
		WHERE key = ANY($1) AND instance_id = $2
	`

	rows, err := r.db.Query(ctx, query, keys, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get cache entries: %w", err)
	}
	defer rows.Close()

	var entries []*CacheEntry
	for rows.Next() {
		var entry CacheEntry
		err := rows.Scan(
			&entry.Key,
			&entry.Value,
			&entry.InstanceID,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&entry.Version,
			&entry.TTL,
			&entry.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan cache entry: %w", err)
		}

		// Skip expired entries
		if !entry.IsExpired() {
			entries = append(entries, &entry)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cache entries: %w", err)
	}

	return entries, nil
}

// GetExpiredKeys returns keys of expired entries
func (r *CacheRepository) GetExpiredKeys(ctx context.Context, limit int) ([]string, error) {
	query := `
		SELECT key
		FROM cache_entries
		WHERE ttl IS NOT NULL 
		AND updated_at + interval '1 second' * ttl < CURRENT_TIMESTAMP
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating expired keys: %w", err)
	}

	return keys, nil
}

// CleanupExpired removes expired entries
func (r *CacheRepository) CleanupExpired(ctx context.Context, batchSize int) (int, error) {
	query := `
		DELETE FROM cache_entries
		WHERE key IN (
			SELECT key
			FROM cache_entries
			WHERE ttl IS NOT NULL 
			AND updated_at + interval '1 second' * ttl < CURRENT_TIMESTAMP
			LIMIT $1
		)
	`

	result, err := r.db.Exec(ctx, query, batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired entries: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// GetAllKeys returns all cache keys for rehydration
func (r *CacheRepository) GetAllKeys(ctx context.Context, offset, limit int) ([]string, error) {
	query := `
		SELECT key
		FROM cache_entries
		WHERE ttl IS NULL OR updated_at + interval '1 second' * ttl > CURRENT_TIMESTAMP
		ORDER BY key
		OFFSET $1 LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating keys: %w", err)
	}

	return keys, nil
}

// GetKeysByInstance returns all cache keys for a specific instance
func (r *CacheRepository) GetKeysByInstance(ctx context.Context, instanceID string, offset, limit int) ([]string, error) {
	query := `
		SELECT key
		FROM cache_entries
		WHERE instance_id = $1 
		AND (ttl IS NULL OR updated_at + interval '1 second' * ttl > CURRENT_TIMESTAMP)
		ORDER BY key
		OFFSET $2 LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, instanceID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating keys: %w", err)
	}

	return keys, nil
}

// DeleteByInstance removes all entries for a specific instance
func (r *CacheRepository) DeleteByInstance(ctx context.Context, instanceID string) (int, error) {
	query := `DELETE FROM cache_entries WHERE instance_id = $1`

	result, err := r.db.Exec(ctx, query, instanceID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete instance entries: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// RecordMetric records a cache operation metric
func (r *CacheRepository) RecordMetric(ctx context.Context, operation string, key *string, duration time.Duration, success bool, errorMsg *string) error {
	metadata := json.RawMessage("{}")
	durationMs := int(duration.Milliseconds())

	query := `
		INSERT INTO cache_metrics (operation, key, duration_ms, success, error_message, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Exec(ctx, query, operation, key, &durationMs, success, errorMsg, metadata)
	if err != nil {
		// Don't fail operations due to metrics recording failures
		return nil
	}

	return nil
}

// Common errors
var (
	ErrNotFound        = errors.New("cache entry not found")
	ErrVersionMismatch = errors.New("version mismatch")
)
