// Simple Cache Example
// This example demonstrates basic cache operations with the Birb-Nest SDK,
// including error handling and proper resource cleanup.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/birbparty/birb-nest/sdk"
)

// User represents a user in our system
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// Create a client with default configuration
	client, err := sdk.NewClient(sdk.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create a context for our operations
	ctx := context.Background()

	// Example 1: Store and retrieve a simple string
	fmt.Println("=== Example 1: Simple String ===")
	if err := simpleStringExample(ctx, client); err != nil {
		log.Printf("String example failed: %v", err)
	}

	// Example 2: Store and retrieve a struct
	fmt.Println("\n=== Example 2: Struct Storage ===")
	if err := structExample(ctx, client); err != nil {
		log.Printf("Struct example failed: %v", err)
	}

	// Example 3: Handle missing keys gracefully
	fmt.Println("\n=== Example 3: Error Handling ===")
	if err := errorHandlingExample(ctx, client); err != nil {
		log.Printf("Error handling example failed: %v", err)
	}

	// Example 4: Working with JSON data
	fmt.Println("\n=== Example 4: JSON Data ===")
	if err := jsonExample(ctx, client); err != nil {
		log.Printf("JSON example failed: %v", err)
	}

	// Example 5: Key patterns and namespacing
	fmt.Println("\n=== Example 5: Key Patterns ===")
	if err := keyPatternExample(ctx, client); err != nil {
		log.Printf("Key pattern example failed: %v", err)
	}
}

func simpleStringExample(ctx context.Context, client sdk.Client) error {
	// Store a string value
	key := "welcome:message"
	value := "Welcome to Birb-Nest!"

	if err := client.Set(ctx, key, value); err != nil {
		return fmt.Errorf("failed to set value: %w", err)
	}
	fmt.Printf("Stored: %s = %s\n", key, value)

	// Retrieve the string value
	var retrieved string
	if err := client.Get(ctx, key, &retrieved); err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}
	fmt.Printf("Retrieved: %s = %s\n", key, retrieved)

	// Delete the key
	if err := client.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	fmt.Printf("Deleted: %s\n", key)

	return nil
}

func structExample(ctx context.Context, client sdk.Client) error {
	// Create a user object
	user := User{
		ID:        123,
		Name:      "Alice Johnson",
		Email:     "alice@example.com",
		CreatedAt: time.Now(),
	}

	// Store the user
	key := fmt.Sprintf("user:%d", user.ID)
	if err := client.Set(ctx, key, user); err != nil {
		return fmt.Errorf("failed to store user: %w", err)
	}
	fmt.Printf("Stored user: %+v\n", user)

	// Retrieve the user
	var retrievedUser User
	if err := client.Get(ctx, key, &retrievedUser); err != nil {
		return fmt.Errorf("failed to retrieve user: %w", err)
	}
	fmt.Printf("Retrieved user: %+v\n", retrievedUser)

	return nil
}

func errorHandlingExample(ctx context.Context, client sdk.Client) error {
	// Try to get a non-existent key
	var value string
	err := client.Get(ctx, "non-existent-key", &value)

	if err != nil {
		if sdk.IsNotFound(err) {
			fmt.Println("Key not found (this is expected)")

			// Create a default value
			defaultValue := "default"
			if err := client.Set(ctx, "non-existent-key", defaultValue); err != nil {
				return fmt.Errorf("failed to set default: %w", err)
			}
			fmt.Printf("Created default value: %s\n", defaultValue)
		} else {
			// Handle other types of errors
			return fmt.Errorf("unexpected error: %w", err)
		}
	}

	// Clean up
	client.Delete(ctx, "non-existent-key")
	return nil
}

func jsonExample(ctx context.Context, client sdk.Client) error {
	// Working with raw JSON data
	jsonData := json.RawMessage(`{
		"type": "notification",
		"priority": "high",
		"message": "System update available",
		"metadata": {
			"version": "2.0.1",
			"size": "45MB"
		}
	}`)

	// Store JSON data
	key := "notification:latest"
	if err := client.Set(ctx, key, jsonData); err != nil {
		return fmt.Errorf("failed to store JSON: %w", err)
	}
	fmt.Println("Stored JSON notification")

	// Retrieve JSON data
	var retrieved json.RawMessage
	if err := client.Get(ctx, key, &retrieved); err != nil {
		return fmt.Errorf("failed to retrieve JSON: %w", err)
	}

	// Parse and display
	var notification map[string]interface{}
	if err := json.Unmarshal(retrieved, &notification); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	fmt.Printf("Retrieved notification: %v\n", notification)

	return nil
}

func keyPatternExample(ctx context.Context, client sdk.Client) error {
	// Demonstrate good key naming patterns
	patterns := []struct {
		pattern     string
		description string
		example     string
		value       interface{}
	}{
		{
			pattern:     "resource:id",
			description: "Simple resource by ID",
			example:     "product:12345",
			value:       "Blue Widget",
		},
		{
			pattern:     "resource:id:field",
			description: "Specific field of a resource",
			example:     "product:12345:price",
			value:       29.99,
		},
		{
			pattern:     "namespace:resource:id",
			description: "Namespaced resources",
			example:     "inventory:product:12345",
			value:       150, // quantity in stock
		},
		{
			pattern:     "cache:query:hash",
			description: "Query result caching",
			example:     "cache:search:a1b2c3d4",
			value:       []string{"result1", "result2", "result3"},
		},
		{
			pattern:     "session:token",
			description: "Session storage",
			example:     "session:xyz789",
			value:       map[string]interface{}{"user_id": 123, "expires": time.Now().Add(1 * time.Hour)},
		},
	}

	fmt.Println("Common key patterns:")
	for _, p := range patterns {
		fmt.Printf("\nPattern: %s\n", p.pattern)
		fmt.Printf("Description: %s\n", p.description)
		fmt.Printf("Example: %s\n", p.example)

		// Store the example
		if err := client.Set(ctx, p.example, p.value); err != nil {
			log.Printf("Failed to set %s: %v", p.example, err)
			continue
		}

		// Retrieve to verify
		var retrieved interface{}
		if err := client.Get(ctx, p.example, &retrieved); err != nil {
			log.Printf("Failed to get %s: %v", p.example, err)
			continue
		}
		fmt.Printf("Stored value: %v\n", retrieved)

		// Clean up
		client.Delete(ctx, p.example)
	}

	return nil
}
