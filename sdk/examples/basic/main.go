package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/birbparty/birb-nest/sdk"
)

func main() {
	// Create an extended client with default configuration
	config := sdk.DefaultConfig().
		WithBaseURL("http://localhost:8080").
		WithTimeout(10 * time.Second).
		WithRetries(3)

	client, err := sdk.NewExtendedClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test connectivity
	fmt.Println("Testing connectivity...")
	if err := client.Ping(ctx); err != nil {
		log.Printf("Warning: Server ping failed: %v", err)
		log.Println("Make sure the Birb Nest server is running on http://localhost:8080")
	} else {
		fmt.Println("✓ Successfully connected to Birb Nest server")
	}

	// Example 1: Store a simple string
	fmt.Println("\n--- Example 1: Simple String ---")
	key1 := "greeting"
	value1 := "Hello, Birb Nest!"

	if err := client.Set(ctx, key1, value1); err != nil {
		log.Fatalf("Failed to set value: %v", err)
	}
	fmt.Printf("✓ Set key '%s' with value '%s'\n", key1, value1)

	// Retrieve the string
	var retrieved1 string
	if err := client.Get(ctx, key1, &retrieved1); err != nil {
		log.Fatalf("Failed to get value: %v", err)
	}
	fmt.Printf("✓ Retrieved: %s\n", retrieved1)

	// Example 2: Store a struct
	fmt.Println("\n--- Example 2: Struct ---")
	type User struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}

	user := User{
		ID:        123,
		Name:      "Birb McFly",
		Email:     "birb@example.com",
		CreatedAt: time.Now(),
	}

	key2 := "user:123"
	if err := client.Set(ctx, key2, user); err != nil {
		log.Fatalf("Failed to set user: %v", err)
	}
	fmt.Printf("✓ Stored user: %+v\n", user)

	// Retrieve the struct
	var retrieved2 User
	if err := client.Get(ctx, key2, &retrieved2); err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("✓ Retrieved user: %+v\n", retrieved2)

	// Example 3: Set with TTL
	fmt.Println("\n--- Example 3: TTL ---")
	key3 := "temporary"
	value3 := map[string]interface{}{
		"message":   "This will expire in 30 seconds",
		"timestamp": time.Now().Unix(),
	}

	ttl := 30 * time.Second
	opts := &sdk.SetOptions{
		TTL: &ttl,
	}

	// Use SetWithOptions directly (available on ExtendedClient)
	if err := client.SetWithOptions(ctx, key3, value3, opts); err != nil {
		log.Fatalf("Failed to set with TTL: %v", err)
	}
	fmt.Printf("✓ Set key '%s' with TTL of %v\n", key3, ttl)

	// Example 4: Check if key exists
	fmt.Println("\n--- Example 4: Existence Check ---")

	// Use Exists method directly (available on ExtendedClient)
	exists, err := client.Exists(ctx, key1)
	if err != nil {
		log.Printf("Failed to check existence: %v", err)
	} else {
		fmt.Printf("✓ Key '%s' exists: %v\n", key1, exists)
	}

	exists, err = client.Exists(ctx, "nonexistent-key")
	if err != nil {
		log.Printf("Failed to check existence: %v", err)
	} else {
		fmt.Printf("✓ Key 'nonexistent-key' exists: %v\n", exists)
	}

	// Example 5: Delete a key
	fmt.Println("\n--- Example 5: Delete ---")
	if err := client.Delete(ctx, key1); err != nil {
		log.Fatalf("Failed to delete key: %v", err)
	}
	fmt.Printf("✓ Deleted key '%s'\n", key1)

	// Verify deletion
	var checkDeleted string
	err = client.Get(ctx, key1, &checkDeleted)
	if sdk.IsNotFound(err) {
		fmt.Println("✓ Confirmed: key no longer exists")
	} else if err != nil {
		log.Printf("Unexpected error: %v", err)
	}

	// Example 6: Error handling
	fmt.Println("\n--- Example 6: Error Handling ---")
	err = client.Get(ctx, "non-existent-key", &checkDeleted)
	if err != nil {
		if sdk.IsNotFound(err) {
			fmt.Println("✓ Correctly identified not found error")
		} else {
			log.Printf("Other error: %v", err)
		}
	}

	// Example 7: Raw JSON
	fmt.Println("\n--- Example 7: Raw JSON ---")
	jsonData := json.RawMessage(`{"type": "bird", "species": "parrot", "colors": ["red", "blue", "green"]}`)
	key4 := "bird-data"

	if err := client.Set(ctx, key4, jsonData); err != nil {
		log.Fatalf("Failed to set JSON: %v", err)
	}
	fmt.Println("✓ Stored raw JSON data")

	var retrievedJSON json.RawMessage
	if err := client.Get(ctx, key4, &retrievedJSON); err != nil {
		log.Fatalf("Failed to get JSON: %v", err)
	}
	fmt.Printf("✓ Retrieved JSON: %s\n", string(retrievedJSON))

	// Example 8: Type-safe operations
	fmt.Println("\n--- Example 8: Type-Safe Operations ---")
	demoTypeSafeOperations(ctx, client)

	fmt.Println("\n✅ All examples completed successfully!")
}

// demoTypeSafeOperations demonstrates the type-safe wrapper functionality
func demoTypeSafeOperations(ctx context.Context, client sdk.ExtendedClient) {
	// Create typed clients for different types
	stringClient := sdk.NewStringClient(client)
	intClient := sdk.NewIntClient(client)

	// Example 8a: Type-safe string operations
	fmt.Println("\n--- Example 8a: Type-Safe String Operations ---")

	// Set a string value
	if err := stringClient.Set(ctx, "typed:message", "Type safety rocks!"); err != nil {
		log.Printf("Failed to set typed string: %v", err)
		return
	}
	fmt.Println("✓ Set typed string value")

	// Get the string value - no need to pass a pointer
	message, err := stringClient.Get(ctx, "typed:message")
	if err != nil {
		log.Printf("Failed to get typed string: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved typed string: %s\n", message)

	// Example 8b: Type-safe integer operations
	fmt.Println("\n--- Example 8b: Type-Safe Integer Operations ---")

	// Set an integer value
	if err := intClient.Set(ctx, "typed:counter", 42); err != nil {
		log.Printf("Failed to set typed int: %v", err)
		return
	}
	fmt.Println("✓ Set typed integer value")

	// Get the integer value
	counter, err := intClient.Get(ctx, "typed:counter")
	if err != nil {
		log.Printf("Failed to get typed int: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved typed integer: %d\n", counter)

	// Example 8c: Using standalone generic functions
	fmt.Println("\n--- Example 8c: Standalone Generic Functions ---")

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	product := Product{
		ID:    "PROD-001",
		Name:  "Birb Seed Premium",
		Price: 19.99,
	}

	// Set using generic function
	if err := sdk.SetTyped(ctx, client, "typed:product", product); err != nil {
		log.Printf("Failed to set typed product: %v", err)
		return
	}
	fmt.Println("✓ Set typed product using generic function")

	// Get using generic function
	retrievedProduct, err := sdk.GetTyped[Product](ctx, client, "typed:product")
	if err != nil {
		log.Printf("Failed to get typed product: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved typed product: %+v\n", retrievedProduct)

	// Example 8d: Type-safe multiple get
	fmt.Println("\n--- Example 8d: Type-Safe Multiple Get ---")

	// Store multiple string values
	stringKeys := []string{"typed:str1", "typed:str2", "typed:str3"}
	for i, key := range stringKeys {
		if err := stringClient.Set(ctx, key, fmt.Sprintf("Value %d", i+1)); err != nil {
			log.Printf("Failed to set %s: %v", key, err)
			return
		}
	}
	fmt.Println("✓ Set multiple string values")

	// Get multiple values with type safety
	results, err := stringClient.GetMultiple(ctx, stringKeys)
	if err != nil {
		log.Printf("Failed to get multiple typed strings: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved %d typed string values\n", len(results))
	for key, value := range results {
		fmt.Printf("  - %s: %s\n", key, value)
	}

	// Example 8e: Custom type with TypedClient
	fmt.Println("\n--- Example 8e: Custom Type with TypedClient ---")

	type Config struct {
		Theme    string   `json:"theme"`
		Language string   `json:"language"`
		Features []string `json:"features"`
	}

	// Create a typed client for Config
	configClient := sdk.NewTypedClient[Config](client)

	config := Config{
		Theme:    "dark",
		Language: "en",
		Features: []string{"cache", "queue", "analytics"},
	}

	// Set config
	if err := configClient.Set(ctx, "typed:config", config); err != nil {
		log.Printf("Failed to set typed config: %v", err)
		return
	}
	fmt.Println("✓ Set typed config")

	// Get config with type safety
	retrievedConfig, err := configClient.Get(ctx, "typed:config")
	if err != nil {
		log.Printf("Failed to get typed config: %v", err)
		return
	}
	fmt.Printf("✓ Retrieved typed config: %+v\n", retrievedConfig)

	// Clean up typed examples
	typedKeys := append(stringKeys, "typed:message", "typed:counter", "typed:product", "typed:config")
	for _, key := range typedKeys {
		client.Delete(ctx, key)
	}
}
