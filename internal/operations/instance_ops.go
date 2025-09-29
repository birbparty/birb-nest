package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/jackc/pgx/v5"
)

// InstanceOperations provides instance-specific data operations
type InstanceOperations struct {
	cache    cache.Cache
	db       *database.DB // Use DB directly for SQL access
	registry *instance.Registry
}

// NewInstanceOperations creates a new instance operations handler
func NewInstanceOperations(cache cache.Cache, db *database.DB, registry *instance.Registry) *InstanceOperations {
	return &InstanceOperations{
		cache:    cache,
		db:       db,
		registry: registry,
	}
}

// LoadInstance loads all data for an instance from database to cache
func (o *InstanceOperations) LoadInstance(ctx context.Context, instanceID string) error {
	// Verify instance exists
	inst, err := o.registry.Get(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	// Update status to indicate loading
	inst.Status = instance.StatusMigrating
	if err := o.registry.Update(ctx, inst); err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}

	// Query all data for instance
	rows, err := o.db.Query(ctx, `
        SELECT key, value, ttl, metadata
        FROM cache_entries
        WHERE instance_id = $1
    `, instanceID)
	if err != nil {
		return fmt.Errorf("failed to query instance data: %w", err)
	}
	defer rows.Close()

	// Build key builder for this instance
	kb := instance.NewKeyBuilder(instanceID)

	// Batch load into cache
	count := 0
	batch := make(map[string][]byte)

	for rows.Next() {
		var key string
		var value json.RawMessage
		var ttl *int
		var metadata json.RawMessage

		if err := rows.Scan(&key, &value, &ttl, &metadata); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Build cache key
		cacheKey := kb.CacheKey(key)
		batch[cacheKey] = value
		count++

		// Flush batch every 1000 items
		if len(batch) >= 1000 {
			if err := o.cache.SetMultiple(ctx, batch, 0); err != nil {
				return fmt.Errorf("failed to batch set cache: %w", err)
			}
			batch = make(map[string][]byte)
		}
	}

	// Flush remaining items
	if len(batch) > 0 {
		if err := o.cache.SetMultiple(ctx, batch, 0); err != nil {
			return fmt.Errorf("failed to batch set cache: %w", err)
		}
	}

	// Update instance status
	inst.Status = instance.StatusActive
	inst.Metadata["last_loaded"] = time.Now().Format(time.RFC3339)
	inst.Metadata["loaded_keys"] = fmt.Sprintf("%d", count)

	if err := o.registry.Update(ctx, inst); err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}

	return nil
}

// DeleteInstance removes all data for an instance
func (o *InstanceOperations) DeleteInstance(ctx context.Context, instanceID string) error {
	// Update instance status
	inst, err := o.registry.Get(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	inst.Status = instance.StatusDeleting
	if err := o.registry.Update(ctx, inst); err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}

	// 1. Delete from cache
	// Note: Since cache interface doesn't have Scan, we'll need to handle this differently
	// For now, we'll skip cache deletion and rely on TTL expiration
	// In production, you'd want to implement a scan method or track keys separately
	log.Printf("Warning: Cache deletion for instance %s requires scan implementation", instanceID)

	// Cache deletion would happen here if we had scan capability
	// TODO: Implement key tracking or add scan to cache interface

	// 2. Delete from database
	result, err := o.db.Exec(ctx, `
        DELETE FROM cache_entries WHERE instance_id = $1
    `, instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete from database: %w", err)
	}

	rowsAffected := result.RowsAffected()

	// 3. Remove from registry
	if err := o.registry.Delete(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to delete from registry: %w", err)
	}

	// Log deletion info
	log.Printf("Deleted instance %s: %d database rows",
		instanceID, rowsAffected)

	return nil
}

// BackupInstance exports instance data to a writer
func (o *InstanceOperations) BackupInstance(ctx context.Context, instanceID string, w io.Writer) error {
	// For now, we'll use a simpler approach
	rows, err := o.db.Query(ctx, `
        SELECT key, value, version, ttl, metadata, created_at, updated_at
        FROM cache_entries
        WHERE instance_id = $1
        ORDER BY key
    `, instanceID)
	if err != nil {
		return fmt.Errorf("failed to query instance data: %w", err)
	}
	defer rows.Close()

	// Write as JSONL (JSON Lines) format for easy processing
	encoder := json.NewEncoder(w)
	count := 0

	for rows.Next() {
		var entry struct {
			InstanceID string          `json:"instance_id"`
			Key        string          `json:"key"`
			Value      json.RawMessage `json:"value"`
			Version    int             `json:"version"`
			TTL        *int            `json:"ttl,omitempty"`
			Metadata   json.RawMessage `json:"metadata,omitempty"`
			CreatedAt  time.Time       `json:"created_at"`
			UpdatedAt  time.Time       `json:"updated_at"`
		}

		entry.InstanceID = instanceID

		if err := rows.Scan(&entry.Key, &entry.Value, &entry.Version,
			&entry.TTL, &entry.Metadata, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to encode entry: %w", err)
		}

		count++
	}

	log.Printf("Backed up instance %s: %d entries", instanceID, count)
	return nil
}

// RestoreInstance imports instance data from a reader
func (o *InstanceOperations) RestoreInstance(ctx context.Context, instanceID string, r io.Reader) error {
	// Parse JSONL format
	decoder := json.NewDecoder(r)
	count := 0

	// Begin transaction for atomicity
	tx, err := o.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert query
	insertQuery := `
        INSERT INTO cache_entries (instance_id, key, value, version, ttl, metadata, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (instance_id, key) DO UPDATE SET
            value = EXCLUDED.value,
            version = EXCLUDED.version,
            ttl = EXCLUDED.ttl,
            metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at
    `

	// Process each line
	for {
		var entry struct {
			InstanceID string          `json:"instance_id"`
			Key        string          `json:"key"`
			Value      json.RawMessage `json:"value"`
			Version    int             `json:"version"`
			TTL        *int            `json:"ttl,omitempty"`
			Metadata   json.RawMessage `json:"metadata,omitempty"`
			CreatedAt  time.Time       `json:"created_at"`
			UpdatedAt  time.Time       `json:"updated_at"`
		}

		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed to decode entry: %w", err)
		}

		// Use provided instanceID instead of the one in backup
		_, err := tx.Exec(ctx, insertQuery, instanceID, entry.Key, entry.Value,
			entry.Version, entry.TTL, entry.Metadata, entry.CreatedAt, entry.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert entry: %w", err)
		}

		count++
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Restored instance %s: %d entries", instanceID, count)
	return nil
}
