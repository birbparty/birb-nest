package sdk

import (
	"context"
	"fmt"
	"time"
)

// TypedClient provides type-safe wrapper around the Client interface.
// It uses Go generics to ensure compile-time type safety for cache operations,
// eliminating the need for manual type assertions and reducing runtime errors.
//
// TypedClient is particularly useful when working with:
//   - Domain-specific types (e.g., User, Product, Session)
//   - Collections of the same type
//   - Applications where type safety is critical
//
// Example with custom type:
//
//	type User struct {
//	    ID    int    `json:"id"`
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	// Create a typed client for User objects
//	userClient := sdk.NewTypedClient[User](extClient)
//
//	// Store a user - no interface{} needed
//	user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
//	err := userClient.Set(ctx, "user:1", user)
//
//	// Retrieve a user - no type assertion needed
//	retrieved, err := userClient.Get(ctx, "user:1")
//	fmt.Printf("User: %+v\n", retrieved) // retrieved is already User type
type TypedClient[T any] struct {
	client ExtendedClient
}

// NewTypedClient creates a new typed client wrapper for type T.
// The client provides type-safe operations without requiring manual
// type assertions on retrieved values.
//
// Example:
//
//	// For built-in types
//	stringClient := sdk.NewTypedClient[string](extClient)
//	intClient := sdk.NewTypedClient[int](extClient)
//
//	// For custom types
//	type Product struct {
//	    SKU   string  `json:"sku"`
//	    Name  string  `json:"name"`
//	    Price float64 `json:"price"`
//	}
//	productClient := sdk.NewTypedClient[Product](extClient)
func NewTypedClient[T any](client ExtendedClient) *TypedClient[T] {
	return &TypedClient[T]{client: client}
}

// Set stores a typed value in the cache.
// The value must be of type T as specified when creating the TypedClient.
//
// Example:
//
//	type Session struct {
//	    UserID    int       `json:"user_id"`
//	    Token     string    `json:"token"`
//	    ExpiresAt time.Time `json:"expires_at"`
//	}
//
//	sessionClient := sdk.NewTypedClient[Session](extClient)
//
//	session := Session{
//	    UserID:    123,
//	    Token:     "secret-token",
//	    ExpiresAt: time.Now().Add(24 * time.Hour),
//	}
//
//	err := sessionClient.Set(ctx, "session:abc123", session)
func (tc *TypedClient[T]) Set(ctx context.Context, key string, value T) error {
	return tc.client.Set(ctx, key, value)
}

// SetWithOptions stores a typed value with additional options like TTL.
// The value must be of type T as specified when creating the TypedClient.
//
// Example:
//
//	type CachedResult struct {
//	    Query  string    `json:"query"`
//	    Result []string  `json:"result"`
//	    Time   time.Time `json:"time"`
//	}
//
//	resultClient := sdk.NewTypedClient[CachedResult](extClient)
//
//	result := CachedResult{
//	    Query:  "SELECT * FROM products",
//	    Result: []string{"product1", "product2"},
//	    Time:   time.Now(),
//	}
//
//	// Cache for 5 minutes
//	ttl := 5 * time.Minute
//	err := resultClient.SetWithOptions(ctx, "query:hash123", result, &sdk.SetOptions{
//	    TTL: &ttl,
//	})
func (tc *TypedClient[T]) SetWithOptions(ctx context.Context, key string, value T, opts *SetOptions) error {
	return tc.client.SetWithOptions(ctx, key, value, opts)
}

// Get retrieves a typed value from the cache.
// Returns the value of type T and an error if the key doesn't exist
// or if there's a deserialization error.
//
// Example:
//
//	type Config struct {
//	    Host     string `json:"host"`
//	    Port     int    `json:"port"`
//	    Timeout  int    `json:"timeout"`
//	}
//
//	configClient := sdk.NewTypedClient[Config](extClient)
//
//	// Get returns Config directly, no type assertion needed
//	config, err := configClient.Get(ctx, "app:config")
//	if err != nil {
//	    if sdk.IsNotFound(err) {
//	        // Use default config
//	        config = Config{Host: "localhost", Port: 8080, Timeout: 30}
//	    } else {
//	        return err
//	    }
//	}
//
//	fmt.Printf("Connecting to %s:%d\n", config.Host, config.Port)
func (tc *TypedClient[T]) Get(ctx context.Context, key string) (T, error) {
	var result T
	err := tc.client.Get(ctx, key, &result)
	return result, err
}

// GetMultiple retrieves multiple typed values from the cache in a single operation.
// Returns a map of key to value for all found keys. Missing keys are not included.
// All values in the returned map are guaranteed to be of type T.
//
// Example:
//
//	type Score struct {
//	    UserID int `json:"user_id"`
//	    Points int `json:"points"`
//	    Level  int `json:"level"`
//	}
//
//	scoreClient := sdk.NewTypedClient[Score](extClient)
//
//	// Get multiple user scores
//	keys := []string{"score:user1", "score:user2", "score:user3"}
//	scores, err := scoreClient.GetMultiple(ctx, keys)
//	if err != nil {
//	    return err
//	}
//
//	// All values are already Score type
//	for key, score := range scores {
//	    fmt.Printf("%s: Level %d, Points %d\n", key, score.Level, score.Points)
//	}
func (tc *TypedClient[T]) GetMultiple(ctx context.Context, keys []string) (map[string]T, error) {
	if len(keys) == 0 {
		return make(map[string]T), nil
	}

	// Get raw results
	rawResults, err := tc.client.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple values: %w", err)
	}

	// Convert to typed results
	results := make(map[string]T)
	for key, rawValue := range rawResults {
		// Use serialization to convert
		serialized, err := serialize(rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize value for key %s: %w", key, err)
		}

		var typedValue T
		if err := deserialize(serialized, &typedValue); err != nil {
			return nil, fmt.Errorf("failed to deserialize value for key %s: %w", key, err)
		}

		results[key] = typedValue
	}

	return results, nil
}

// Standalone generic functions for convenience
//
// These functions provide type-safe operations without creating a TypedClient.
// They're useful for one-off operations or when you don't want to create
// a dedicated typed client.

// SetTyped stores a typed value using the provided client.
// This is a convenience function for one-off typed operations
// without creating a TypedClient.
//
// Example:
//
//	type Event struct {
//	    Type      string    `json:"type"`
//	    Timestamp time.Time `json:"timestamp"`
//	    Data      any       `json:"data"`
//	}
//
//	event := Event{
//	    Type:      "user_login",
//	    Timestamp: time.Now(),
//	    Data:      map[string]string{"user_id": "123"},
//	}
//
//	err := sdk.SetTyped(ctx, client, "event:latest", event)
func SetTyped[T any](ctx context.Context, client Client, key string, value T) error {
	return client.Set(ctx, key, value)
}

// SetTypedWithOptions stores a typed value with options using the provided client.
// This is a convenience function for one-off typed operations with custom options.
//
// Example:
//
//	type TempData struct {
//	    Value   string    `json:"value"`
//	    Created time.Time `json:"created"`
//	}
//
//	data := TempData{
//	    Value:   "temporary",
//	    Created: time.Now(),
//	}
//
//	ttl := 5 * time.Minute
//	err := sdk.SetTypedWithOptions(ctx, extClient, "temp:data", data, &sdk.SetOptions{
//	    TTL: &ttl,
//	})
func SetTypedWithOptions[T any](ctx context.Context, client ExtendedClient, key string, value T, opts *SetOptions) error {
	return client.SetWithOptions(ctx, key, value, opts)
}

// GetTyped retrieves a typed value using the provided client.
// This is a convenience function for one-off typed operations
// without creating a TypedClient.
//
// Example:
//
//	type Settings struct {
//	    Theme    string   `json:"theme"`
//	    Language string   `json:"language"`
//	    Features []string `json:"features"`
//	}
//
//	// Get settings with type safety
//	settings, err := sdk.GetTyped[Settings](ctx, client, "user:settings")
//	if err != nil {
//	    // Handle error
//	}
//
//	fmt.Printf("Theme: %s\n", settings.Theme)
func GetTyped[T any](ctx context.Context, client Client, key string) (T, error) {
	var result T
	err := client.Get(ctx, key, &result)
	return result, err
}

// GetTypedMultiple retrieves multiple typed values using the provided client.
// This is a convenience function for batch operations without creating a TypedClient.
//
// Example:
//
//	type Product struct {
//	    ID    int     `json:"id"`
//	    Name  string  `json:"name"`
//	    Price float64 `json:"price"`
//	}
//
//	keys := []string{"product:1", "product:2", "product:3"}
//	products, err := sdk.GetTypedMultiple[Product](ctx, extClient, keys)
//	if err != nil {
//	    return err
//	}
//
//	for key, product := range products {
//	    fmt.Printf("%s: %s ($%.2f)\n", key, product.Name, product.Price)
//	}
func GetTypedMultiple[T any](ctx context.Context, client ExtendedClient, keys []string) (map[string]T, error) {
	if len(keys) == 0 {
		return make(map[string]T), nil
	}

	// Get raw results
	rawResults, err := client.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple values: %w", err)
	}

	// Convert to typed results
	results := make(map[string]T)
	for key, rawValue := range rawResults {
		// Use serialization to convert
		serialized, err := serialize(rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize value for key %s: %w", key, err)
		}

		var typedValue T
		if err := deserialize(serialized, &typedValue); err != nil {
			return nil, fmt.Errorf("failed to deserialize value for key %s: %w", key, err)
		}

		results[key] = typedValue
	}

	return results, nil
}

// Common type aliases for convenience
//
// These aliases provide ready-to-use typed clients for common types,
// making it easier to work with basic data types in a type-safe manner.

// StringClient provides type-safe operations for string values.
//
// Example:
//
//	stringClient := sdk.NewStringClient(extClient)
//	err := stringClient.Set(ctx, "message", "Hello, World!")
//	msg, err := stringClient.Get(ctx, "message")
type StringClient = TypedClient[string]

// IntClient provides type-safe operations for int values.
//
// Example:
//
//	intClient := sdk.NewIntClient(extClient)
//	err := intClient.Set(ctx, "counter", 42)
//	count, err := intClient.Get(ctx, "counter")
type IntClient = TypedClient[int]

// BoolClient provides type-safe operations for bool values.
//
// Example:
//
//	boolClient := sdk.NewBoolClient(extClient)
//	err := boolClient.Set(ctx, "feature:enabled", true)
//	enabled, err := boolClient.Get(ctx, "feature:enabled")
type BoolClient = TypedClient[bool]

// TimeClient provides type-safe operations for time.Time values.
//
// Example:
//
//	timeClient := sdk.NewTimeClient(extClient)
//	err := timeClient.Set(ctx, "last:update", time.Now())
//	lastUpdate, err := timeClient.Get(ctx, "last:update")
type TimeClient = TypedClient[time.Time]

// BytesClient provides type-safe operations for []byte values.
//
// Example:
//
//	bytesClient := sdk.NewBytesClient(extClient)
//	data := []byte("binary data")
//	err := bytesClient.Set(ctx, "file:content", data)
//	content, err := bytesClient.Get(ctx, "file:content")
type BytesClient = TypedClient[[]byte]

// MapClient provides type-safe operations for map[string]interface{} values.
//
// Example:
//
//	mapClient := sdk.NewMapClient(extClient)
//	data := map[string]interface{}{
//	    "name": "John",
//	    "age":  30,
//	    "tags": []string{"user", "active"},
//	}
//	err := mapClient.Set(ctx, "user:data", data)
//	retrieved, err := mapClient.Get(ctx, "user:data")
type MapClient = TypedClient[map[string]interface{}]

// Helper constructors for common types
//
// These constructors provide a convenient way to create typed clients
// for common data types without using generics syntax directly.

// NewStringClient creates a client for string values.
// Useful for caching text data like messages, tokens, or identifiers.
//
// Example:
//
//	client := sdk.NewStringClient(extClient)
//	err := client.Set(ctx, "greeting", "Welcome!")
//	greeting, err := client.Get(ctx, "greeting")
func NewStringClient(client ExtendedClient) *StringClient {
	return NewTypedClient[string](client)
}

// NewIntClient creates a client for int values.
// Useful for caching counters, IDs, or numeric values.
//
// Example:
//
//	client := sdk.NewIntClient(extClient)
//	err := client.Set(ctx, "user:count", 1000)
//	count, err := client.Get(ctx, "user:count")
func NewIntClient(client ExtendedClient) *IntClient {
	return NewTypedClient[int](client)
}

// NewBoolClient creates a client for bool values.
// Useful for caching flags, toggles, or binary states.
//
// Example:
//
//	client := sdk.NewBoolClient(extClient)
//	err := client.Set(ctx, "maintenance:mode", false)
//	isMaintenanceMode, err := client.Get(ctx, "maintenance:mode")
func NewBoolClient(client ExtendedClient) *BoolClient {
	return NewTypedClient[bool](client)
}

// NewTimeClient creates a client for time.Time values.
// Useful for caching timestamps, schedules, or time-based data.
//
// Example:
//
//	client := sdk.NewTimeClient(extClient)
//	err := client.Set(ctx, "last:backup", time.Now())
//	lastBackup, err := client.Get(ctx, "last:backup")
//	if time.Since(lastBackup) > 24*time.Hour {
//	    // Time for another backup
//	}
func NewTimeClient(client ExtendedClient) *TimeClient {
	return NewTypedClient[time.Time](client)
}

// NewBytesClient creates a client for []byte values.
// Useful for caching binary data, files, or pre-serialized content.
//
// Example:
//
//	client := sdk.NewBytesClient(extClient)
//	imageData := []byte{/* image bytes */}
//	err := client.Set(ctx, "image:thumbnail", imageData)
//	thumbnail, err := client.Get(ctx, "image:thumbnail")
func NewBytesClient(client ExtendedClient) *BytesClient {
	return NewTypedClient[[]byte](client)
}

// NewMapClient creates a client for map[string]interface{} values.
// Useful for caching flexible JSON-like structures or dynamic data.
//
// Example:
//
//	client := sdk.NewMapClient(extClient)
//	metadata := map[string]interface{}{
//	    "version": "1.0",
//	    "author": "John Doe",
//	    "tags": []string{"important", "reviewed"},
//	}
//	err := client.Set(ctx, "doc:metadata", metadata)
//	meta, err := client.Get(ctx, "doc:metadata")
func NewMapClient(client ExtendedClient) *MapClient {
	return NewTypedClient[map[string]interface{}](client)
}
