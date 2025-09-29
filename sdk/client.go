package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Client represents a Birb Nest cache client that provides thread-safe
// operations for storing and retrieving data from the cache service.
//
// The Client interface defines the core operations available for interacting
// with the Birb Nest cache. All methods are safe for concurrent use.
//
// Example:
//
//	client, err := sdk.NewClient(sdk.DefaultConfig())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	ctx := context.Background()
//
//	// Store data
//	err = client.Set(ctx, "user:123", User{Name: "Alice"})
//	if err != nil {
//	    log.Printf("Failed to set: %v", err)
//	}
//
//	// Retrieve data
//	var user User
//	err = client.Get(ctx, "user:123", &user)
//	if err != nil {
//	    if sdk.IsNotFound(err) {
//	        log.Println("User not found")
//	    } else {
//	        log.Printf("Failed to get: %v", err)
//	    }
//	}
type Client interface {
	// Set stores a value in the cache with the given key.
	// The value can be any type that is JSON-serializable.
	// To set a value with TTL or metadata, use ExtendedClient.SetWithOptions.
	//
	// Example:
	//
	//	err := client.Set(ctx, "config:app", AppConfig{
	//	    Debug: true,
	//	    MaxConnections: 100,
	//	})
	Set(ctx context.Context, key string, value interface{}) error

	// Get retrieves a value from the cache by its key.
	// The value is deserialized into the provided destination.
	// The destination must be a pointer to the expected type.
	//
	// Returns an error if the key doesn't exist (use IsNotFound to check),
	// if deserialization fails, or if there's a network/server error.
	//
	// Example:
	//
	//	var config AppConfig
	//	err := client.Get(ctx, "config:app", &config)
	//	if sdk.IsNotFound(err) {
	//	    // Key doesn't exist
	//	}
	Get(ctx context.Context, key string, dest interface{}) error

	// Delete removes a key from the cache.
	// It's not an error if the key doesn't exist.
	//
	// Example:
	//
	//	err := client.Delete(ctx, "session:expired")
	Delete(ctx context.Context, key string) error

	// Ping checks connectivity to the server.
	// Useful for health checks and connection validation.
	//
	// Example:
	//
	//	if err := client.Ping(ctx); err != nil {
	//	    log.Printf("Server is not reachable: %v", err)
	//	}
	Ping(ctx context.Context) error

	// Close closes the client and releases all resources.
	// After calling Close, the client should not be used.
	// Close is safe to call multiple times.
	//
	// Example:
	//
	//	client, _ := sdk.NewClient(config)
	//	defer client.Close() // Always close when done
	Close() error
}

// ExtendedClient provides additional functionality beyond the basic Client interface.
// It includes all Client methods plus advanced operations like batch operations,
// existence checks, and setting values with options.
//
// Example:
//
//	extClient, err := sdk.NewExtendedClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Set with TTL
//	ttl := 5 * time.Minute
//	err = extClient.SetWithOptions(ctx, "session:abc", sessionData, &sdk.SetOptions{
//	    TTL: &ttl,
//	})
//
//	// Check if key exists
//	exists, err := extClient.Exists(ctx, "session:abc")
//
//	// Get multiple keys at once
//	values, err := extClient.GetMultiple(ctx, []string{"key1", "key2", "key3"})
type ExtendedClient interface {
	Client

	// SetWithOptions stores a value with additional options such as TTL and metadata.
	// This provides more control over cache entries than the basic Set method.
	//
	// Example:
	//
	//	ttl := 30 * time.Minute
	//	err := client.SetWithOptions(ctx, "cache:temp", data, &sdk.SetOptions{
	//	    TTL: &ttl,
	//	    Metadata: map[string]interface{}{
	//	        "created_by": "user123",
	//	        "version": "1.0",
	//	    },
	//	})
	SetWithOptions(ctx context.Context, key string, value interface{}, opts *SetOptions) error

	// Exists checks if a key exists in the cache without retrieving its value.
	// More efficient than Get when you only need to check existence.
	//
	// Example:
	//
	//	if exists, err := client.Exists(ctx, "user:123"); exists {
	//	    log.Println("User is cached")
	//	}
	Exists(ctx context.Context, key string) (bool, error)

	// GetMultiple retrieves multiple values by their keys in a single operation.
	// Returns a map of key to value for all found keys. Missing keys are not included.
	// Currently implemented as parallel individual gets, but may use batch endpoint in future.
	//
	// Example:
	//
	//	keys := []string{"user:1", "user:2", "user:3"}
	//	values, err := client.GetMultiple(ctx, keys)
	//	for key, value := range values {
	//	    log.Printf("%s: %v", key, value)
	//	}
	GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)
}

// client is the concrete implementation of the Client interface
type client struct {
	transport *httpTransport
	config    *Config
	mu        sync.RWMutex
	closed    bool
}

// NewClient creates a new Birb Nest client with the provided configuration.
// If config is nil, default configuration values will be used.
//
// The client maintains a connection pool for efficient HTTP communication
// and is safe for concurrent use by multiple goroutines.
//
// Example:
//
//	// Create with default config
//	client, err := sdk.NewClient(nil)
//
//	// Create with custom config
//	config := sdk.DefaultConfig().
//	    WithBaseURL("https://cache.example.com").
//	    WithTimeout(10 * time.Second)
//	client, err := sdk.NewClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
func NewClient(config *Config) (Client, error) {
	// Use default config if nil
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create transport
	transport, err := newHTTPTransport(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	return &client{
		transport: transport,
		config:    config,
	}, nil
}

// NewExtendedClient creates a new Birb Nest client with extended functionality.
// This client includes all basic Client operations plus additional features
// like batch operations, existence checks, and options for Set operations.
//
// Example:
//
//	extClient, err := sdk.NewExtendedClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use extended features
//	ttl := 1 * time.Hour
//	err = extClient.SetWithOptions(ctx, "data:important", data, &sdk.SetOptions{
//	    TTL: &ttl,
//	})
func NewExtendedClient(config *Config) (ExtendedClient, error) {
	// Use default config if nil
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create transport
	transport, err := newHTTPTransport(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	return &client{
		transport: transport,
		config:    config,
	}, nil
}

// Set stores a value with an optional TTL
func (c *client) Set(ctx context.Context, key string, value interface{}) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	return c.SetWithOptions(ctx, key, value, nil)
}

// SetWithOptions stores a value with additional options
func (c *client) SetWithOptions(ctx context.Context, key string, value interface{}, opts *SetOptions) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Build request
	var ttl *time.Duration
	var metadata map[string]interface{}

	if opts != nil {
		ttl = opts.TTL
		metadata = opts.Metadata
	}

	req, err := buildCacheRequest(value, ttl, metadata)
	if err != nil {
		return err
	}

	// Send request
	path := fmt.Sprintf("/v1/cache/%s", key)
	var resp CacheResponse
	if err := c.transport.post(ctx, path, req, &resp); err != nil {
		return err
	}

	return nil
}

// Get retrieves a value by key
func (c *client) Get(ctx context.Context, key string, dest interface{}) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}

	// Send request
	path := fmt.Sprintf("/v1/cache/%s", key)
	var resp CacheResponse
	if err := c.transport.get(ctx, path, &resp); err != nil {
		return err
	}

	// Deserialize value
	return deserialize(resp.Value, dest)
}

// Delete removes a key from the cache
func (c *client) Delete(ctx context.Context, key string) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Send request
	path := fmt.Sprintf("/v1/cache/%s", key)
	return c.transport.delete(ctx, path)
}

// Ping checks connectivity to the server
func (c *client) Ping(ctx context.Context) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	// Send health check request
	var resp HealthResponse
	if err := c.transport.get(ctx, "/health", &resp); err != nil {
		return err
	}

	if resp.Status != "healthy" {
		return fmt.Errorf("server is not healthy: %s", resp.Status)
	}

	return nil
}

// Close closes the client and releases resources
func (c *client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.transport.close()
}

// checkClosed checks if the client is closed
func (c *client) checkClosed() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}
	return nil
}

// SetOptions contains optional parameters for the SetWithOptions operation.
// All fields are optional - omit or set to nil/empty to use defaults.
//
// Example:
//
//	ttl := 30 * time.Minute
//	opts := &sdk.SetOptions{
//	    TTL: &ttl,
//	    Metadata: map[string]interface{}{
//	        "source": "api",
//	        "version": 2,
//	    },
//	}
//	err := client.SetWithOptions(ctx, "key", value, opts)
type SetOptions struct {
	// TTL is the time-to-live for the cache entry.
	// If nil, the entry will not expire.
	TTL *time.Duration

	// Metadata is additional metadata to store with the entry.
	// This can be used for tracking, versioning, or other purposes.
	Metadata map[string]interface{}
}

// Advanced operations for future use

// GetMultiple retrieves multiple values by keys
func (c *client) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	// For now, implement as individual gets
	// TODO: Implement batch endpoint when available
	results := make(map[string]interface{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(keys))

	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			var value interface{}
			if err := c.Get(ctx, k, &value); err != nil {
				if !IsNotFound(err) {
					errChan <- err
				}
				return
			}
			mu.Lock()
			results[k] = value
			mu.Unlock()
		}(key)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		return nil, err
	}

	return results, nil
}

// Exists checks if a key exists in the cache
func (c *client) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.checkClosed(); err != nil {
		return false, err
	}

	if key == "" {
		return false, fmt.Errorf("key cannot be empty")
	}

	// Try to get the key
	var dummy interface{}
	err := c.Get(ctx, key, &dummy)
	if err == nil {
		return true, nil
	}

	if IsNotFound(err) {
		return false, nil
	}

	return false, err
}
