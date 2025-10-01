package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/birbparty/birb-nest/internal/instance"
)

// Interface defines the database operations interface
type Interface interface {
	// Get retrieves a value from the database
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the database
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes a value from the database
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the database
	Exists(ctx context.Context, key string) (bool, error)

	// GetWithInstance retrieves a value with instance awareness
	GetWithInstance(ctx context.Context, key, instanceID string) ([]byte, error)

	// SetWithInstance stores a value with instance awareness
	SetWithInstance(ctx context.Context, key, instanceID string, value []byte) error

	// DeleteWithInstance removes a value with instance awareness
	DeleteWithInstance(ctx context.Context, key, instanceID string) error

	// ExistsWithInstance checks if a key exists with instance awareness
	ExistsWithInstance(ctx context.Context, key, instanceID string) (bool, error)

	// Health checks if the database is healthy
	Health(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// Context-aware methods that extract instance from context

	// GetFromContext retrieves a value using instance ID from context
	GetFromContext(ctx context.Context, key string) ([]byte, error)

	// SetFromContext stores a value using instance ID from context
	SetFromContext(ctx context.Context, key string, value []byte) error

	// DeleteFromContext removes a value using instance ID from context
	DeleteFromContext(ctx context.Context, key string) error

	// ExistsFromContext checks if a key exists using instance ID from context
	ExistsFromContext(ctx context.Context, key string) (bool, error)
}

// PostgreSQLClient implements the Interface for PostgreSQL
type PostgreSQLClient struct {
	db         *DB
	repo       *CacheRepository
	instanceID string
}

// NewPostgreSQLClient creates a new PostgreSQL client
func NewPostgreSQLClient(cfg *Config, instanceID string) (Interface, error) {
	db, err := NewDB(cfg)
	if err != nil {
		return nil, err
	}

	return &PostgreSQLClient{
		db:         db,
		repo:       NewCacheRepository(db),
		instanceID: instanceID,
	}, nil
}

// Get retrieves a value from the database
func (c *PostgreSQLClient) Get(ctx context.Context, key string) ([]byte, error) {
	return c.GetWithInstance(ctx, key, c.instanceID)
}

// GetWithInstance retrieves a value with instance awareness
func (c *PostgreSQLClient) GetWithInstance(ctx context.Context, key, instanceID string) ([]byte, error) {
	entry, err := c.repo.GetWithInstance(ctx, key, instanceID)
	if err != nil {
		return nil, err
	}
	return entry.Value, nil
}

// Set stores a value in the database
func (c *PostgreSQLClient) Set(ctx context.Context, key string, value []byte) error {
	return c.SetWithInstance(ctx, key, c.instanceID, value)
}

// SetWithInstance stores a value with instance awareness
func (c *PostgreSQLClient) SetWithInstance(ctx context.Context, key, instanceID string, value []byte) error {
	// Validate that value is valid JSON before passing to repository
	// The cache_entries.value column is JSONB and requires valid JSON
	jsonValue := value

	// Check if the value is already valid JSON
	if !json.Valid(value) {
		// Value is not valid JSON, wrap it as a JSON string
		wrapped, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to encode non-JSON value as JSON: %w", err)
		}
		jsonValue = wrapped

		// Log the conversion for debugging
		log.Printf("Warning: Non-JSON value for key '%s' in instance '%s' was wrapped as JSON. Original length: %d bytes",
			key, instanceID, len(value))
	}

	return c.repo.SetWithInstance(ctx, key, instanceID, jsonValue, nil, nil)
}

// Delete removes a value from the database
func (c *PostgreSQLClient) Delete(ctx context.Context, key string) error {
	return c.DeleteWithInstance(ctx, key, c.instanceID)
}

// DeleteWithInstance removes a value with instance awareness
func (c *PostgreSQLClient) DeleteWithInstance(ctx context.Context, key, instanceID string) error {
	return c.repo.DeleteWithInstance(ctx, key, instanceID)
}

// Exists checks if a key exists in the database
func (c *PostgreSQLClient) Exists(ctx context.Context, key string) (bool, error) {
	return c.ExistsWithInstance(ctx, key, c.instanceID)
}

// ExistsWithInstance checks if a key exists with instance awareness
func (c *PostgreSQLClient) ExistsWithInstance(ctx context.Context, key, instanceID string) (bool, error) {
	return c.repo.ExistsWithInstance(ctx, key, instanceID)
}

// Health checks if the database is healthy
func (c *PostgreSQLClient) Health(ctx context.Context) error {
	return c.db.Health(ctx)
}

// Close closes the database connection
func (c *PostgreSQLClient) Close() error {
	c.db.Close()
	return nil
}

// GetFromContext retrieves a value using instance ID from context
func (c *PostgreSQLClient) GetFromContext(ctx context.Context, key string) ([]byte, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = c.instanceID
	}
	return c.GetWithInstance(ctx, key, instanceID)
}

// SetFromContext stores a value using instance ID from context
func (c *PostgreSQLClient) SetFromContext(ctx context.Context, key string, value []byte) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = c.instanceID
	}
	return c.SetWithInstance(ctx, key, instanceID, value)
}

// DeleteFromContext removes a value using instance ID from context
func (c *PostgreSQLClient) DeleteFromContext(ctx context.Context, key string) error {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = c.instanceID
	}
	return c.DeleteWithInstance(ctx, key, instanceID)
}

// ExistsFromContext checks if a key exists using instance ID from context
func (c *PostgreSQLClient) ExistsFromContext(ctx context.Context, key string) (bool, error) {
	instanceID := instance.ExtractInstanceID(ctx)
	if instanceID == "" {
		instanceID = c.instanceID
	}
	return c.ExistsWithInstance(ctx, key, instanceID)
}
