// Connection Resilience Example
// This example demonstrates how the Birb-Nest SDK handles network failures,
// retries, and circuit breaker patterns to maintain reliability.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/birbparty/birb-nest/sdk"
)

func main() {
	// Create a client with custom retry configuration
	config := sdk.DefaultConfig().
		WithBaseURL("http://localhost:8080").
		WithTimeout(5 * time.Second).
		WithRetries(5) // Increase retry attempts

	client, err := sdk.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: Handling temporary network failures
	fmt.Println("=== Example 1: Automatic Retry on Network Failure ===")
	handleNetworkFailure(ctx, client)

	// Example 2: Circuit breaker demonstration
	fmt.Println("\n=== Example 2: Circuit Breaker Protection ===")
	demonstrateCircuitBreaker(ctx, client)

	// Example 3: Graceful degradation
	fmt.Println("\n=== Example 3: Graceful Degradation ===")
	gracefulDegradation(ctx, client)

	// Example 4: Connection pool management
	fmt.Println("\n=== Example 4: Connection Pool Under Load ===")
	connectionPoolDemo(ctx, client)

	// Example 5: Timeout handling
	fmt.Println("\n=== Example 5: Timeout Scenarios ===")
	timeoutHandling(ctx, client)
}

func handleNetworkFailure(ctx context.Context, client sdk.Client) {
	// This simulates what happens when the network has intermittent issues
	fmt.Println("Attempting operations that might fail due to network issues...")

	// Store a value
	key := "resilience:test"
	value := "Testing automatic retry"

	err := client.Set(ctx, key, value)
	if err != nil {
		// Check if it's a retryable error
		if sdk.IsRetryable(err) {
			fmt.Printf("Operation failed with retryable error: %v\n", err)
			fmt.Println("The SDK automatically retried this operation")
		} else {
			fmt.Printf("Operation failed with non-retryable error: %v\n", err)
		}

		// Check error details
		var sdkErr *sdk.Error
		if errors.As(err, &sdkErr) {
			fmt.Printf("Error Type: %s\n", sdkErr.Type)
			fmt.Printf("Retryable: %v\n", sdkErr.IsRetryable())
			if sdkErr.Context != nil {
				fmt.Printf("Retry Count: %d\n", sdkErr.Context.RetryCount)
				fmt.Printf("Duration: %v\n", sdkErr.Context.Duration)
			}
		}
	} else {
		fmt.Println("Operation succeeded (possibly after retries)")

		// Verify the value was stored
		var retrieved string
		if err := client.Get(ctx, key, &retrieved); err == nil {
			fmt.Printf("Retrieved value: %s\n", retrieved)
		}
	}
}

func demonstrateCircuitBreaker(ctx context.Context, client sdk.Client) {
	// Circuit breaker protects the system from cascading failures
	fmt.Println("Simulating multiple failures to trigger circuit breaker...")

	// Create a client with circuit breaker configuration
	// Note: In the actual SDK, circuit breaker is built-in and automatic
	cbConfig := sdk.DefaultConfig().
		WithBaseURL("http://localhost:8080")

	cbClient, err := sdk.NewClient(cbConfig)
	if err != nil {
		log.Printf("Failed to create circuit breaker client: %v", err)
		return
	}
	defer cbClient.Close()

	// Simulate operations that might trigger the circuit breaker
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("cb:test:%d", i)
		err := cbClient.Set(ctx, key, "test value")

		if err != nil {
			if errors.Is(err, sdk.ErrCircuitOpen) {
				fmt.Printf("Attempt %d: Circuit breaker is OPEN - failing fast\n", i+1)
			} else {
				fmt.Printf("Attempt %d: Operation failed: %v\n", i+1, err)
			}
		} else {
			fmt.Printf("Attempt %d: Operation succeeded\n", i+1)
		}

		// Small delay between attempts
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\nCircuit breaker will automatically close after the timeout period")
}

func gracefulDegradation(ctx context.Context, client sdk.Client) {
	// Show how to implement graceful degradation when cache is unavailable
	fmt.Println("Implementing fallback behavior for cache failures...")

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	// Function to get product with fallback
	getProduct := func(productID string) (*Product, error) {
		key := fmt.Sprintf("product:%s", productID)

		// Try to get from cache first
		var cachedProduct Product
		err := client.Get(ctx, key, &cachedProduct)

		if err == nil {
			fmt.Printf("Cache HIT for product %s\n", productID)
			return &cachedProduct, nil
		}

		// Handle cache miss or error
		if sdk.IsNotFound(err) {
			fmt.Printf("Cache MISS for product %s - fetching from source\n", productID)
		} else {
			fmt.Printf("Cache ERROR for product %s: %v - falling back to source\n", productID, err)
		}

		// Fallback: fetch from primary data source (simulated)
		product := &Product{
			ID:    productID,
			Name:  fmt.Sprintf("Product %s", productID),
			Price: 99.99,
		}

		// Try to cache it for next time (non-blocking)
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			if err := client.Set(cacheCtx, key, product); err != nil {
				log.Printf("Failed to cache product %s: %v", productID, err)
			}
		}()

		return product, nil
	}

	// Use the function
	product, err := getProduct("12345")
	if err != nil {
		log.Printf("Failed to get product: %v", err)
	} else {
		fmt.Printf("Got product: %+v\n", product)
	}
}

func connectionPoolDemo(ctx context.Context, client sdk.Client) {
	// Demonstrate how the SDK handles many concurrent requests
	fmt.Println("Testing connection pool with concurrent requests...")

	const numRequests = 50
	var wg sync.WaitGroup
	results := make(chan string, numRequests)
	errors := make(chan error, numRequests)

	// Launch many concurrent requests
	start := time.Now()
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			key := fmt.Sprintf("concurrent:key:%d", n)
			value := fmt.Sprintf("value-%d", n)

			// Set value
			if err := client.Set(ctx, key, value); err != nil {
				errors <- fmt.Errorf("set %s failed: %w", key, err)
				return
			}

			// Get value
			var retrieved string
			if err := client.Get(ctx, key, &retrieved); err != nil {
				errors <- fmt.Errorf("get %s failed: %w", key, err)
				return
			}

			results <- fmt.Sprintf("Request %d completed", n)
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()
	close(results)
	close(errors)

	duration := time.Since(start)

	// Count results
	successCount := len(results)
	errorCount := len(errors)

	fmt.Printf("\nCompleted %d requests in %v\n", numRequests, duration)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", errorCount)
	fmt.Printf("Average time per request: %v\n", duration/time.Duration(numRequests))

	// Print first few errors if any
	count := 0
	for err := range errors {
		fmt.Printf("Error: %v\n", err)
		count++
		if count >= 3 {
			break
		}
	}
}

func timeoutHandling(ctx context.Context, client sdk.Client) {
	// Demonstrate different timeout scenarios
	fmt.Println("Testing timeout handling...")

	// Scenario 1: Operation timeout
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err := client.Set(shortCtx, "timeout:test", "This might timeout")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("Operation timed out as expected")
		} else {
			fmt.Printf("Operation failed with different error: %v\n", err)
		}
	} else {
		fmt.Println("Operation completed within timeout")
	}

	// Scenario 2: Handling timeout gracefully
	fmt.Println("\nImplementing timeout with fallback...")

	getValue := func(key string, timeout time.Duration) (string, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var value string
		done := make(chan error, 1)

		go func() {
			done <- client.Get(timeoutCtx, key, &value)
		}()

		select {
		case err := <-done:
			if err != nil {
				return "", err
			}
			return value, nil
		case <-timeoutCtx.Done():
			return "default-value", fmt.Errorf("operation timed out, using default")
		}
	}

	// Try with a reasonable timeout
	result, err := getValue("timeout:key", 2*time.Second)
	if err != nil {
		fmt.Printf("Get with timeout: %v\n", err)
	} else {
		fmt.Printf("Got value: %s\n", result)
	}
}
