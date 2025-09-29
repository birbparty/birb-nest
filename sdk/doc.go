// Package sdk provides a high-performance Go client library for the Birb Nest
// persistent cache service. This SDK offers a simple, efficient, and type-safe
// way to interact with the Birb Nest API with zero external dependencies.
//
// # Features
//
// The SDK provides:
//   - Zero external dependencies (uses only Go standard library)
//   - Thread-safe operations with connection pooling
//   - Automatic retries with exponential backoff
//   - Circuit breaker pattern for fault tolerance
//   - Context support for cancellation and timeouts
//   - Type-safe operations with automatic JSON serialization
//   - WASM compatibility for browser-based applications
//   - Comprehensive error handling with retryable error detection
//
// # Basic Usage
//
// Create a client and perform basic cache operations:
//
//	package main
//
//	import (
//	    "context"
//	    "log"
//	    "github.com/birbparty/birb-nest/sdk"
//	)
//
//	func main() {
//	    // Create client with default configuration
//	    client, err := sdk.NewClient(sdk.DefaultConfig())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer client.Close()
//
//	    ctx := context.Background()
//
//	    // Store a value
//	    err = client.Set(ctx, "user:123", map[string]string{
//	        "name": "Alice",
//	        "email": "alice@example.com",
//	    })
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Retrieve a value
//	    var user map[string]string
//	    err = client.Get(ctx, "user:123", &user)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// # Configuration
//
// The SDK can be configured using a fluent builder pattern:
//
//	config := sdk.DefaultConfig().
//	    WithBaseURL("https://cache.example.com").
//	    WithTimeout(10 * time.Second).
//	    WithRetries(5).
//	    WithCircuitBreaker(sdk.CircuitBreakerConfig{
//	        FailureThreshold: 5,
//	        Timeout: 30 * time.Second,
//	    })
//
//	client, err := sdk.NewClient(config)
//
// # Extended Client
//
// For advanced features, use the ExtendedClient interface:
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
//	// Check existence
//	exists, err := extClient.Exists(ctx, "session:abc")
//
//	// Get multiple keys
//	values, err := extClient.GetMultiple(ctx, []string{"key1", "key2", "key3"})
//
// # Error Handling
//
// The SDK provides rich error information and helper functions:
//
//	err := client.Get(ctx, "missing-key", &value)
//	if sdk.IsNotFound(err) {
//	    // Handle missing key
//	    return nil
//	}
//
//	if sdk.IsRetryable(err) {
//	    // Error is transient, can retry
//	}
//
//	// Access detailed error information
//	var sdkErr *sdk.Error
//	if errors.As(err, &sdkErr) {
//	    log.Printf("Error type: %s, Retryable: %v", sdkErr.Type, sdkErr.IsRetryable())
//	}
//
// # Circuit Breaker
//
// The SDK includes circuit breaker functionality to prevent cascading failures:
//
//	config := sdk.DefaultConfig().WithCircuitBreaker(sdk.CircuitBreakerConfig{
//	    FailureThreshold: 5,      // Open after 5 failures
//	    SuccessThreshold: 2,      // Close after 2 successes in half-open
//	    Timeout: 30 * time.Second, // Try half-open after 30s
//	})
//
// Circuit states:
//   - Closed: Normal operation, requests pass through
//   - Open: Requests fail immediately without calling the server
//   - Half-Open: Limited requests allowed to test recovery
//
// # Retry Strategies
//
// The SDK supports various retry strategies:
//
//	// Exponential backoff (default)
//	config.WithRetryStrategy(sdk.NewExponentialBackoffStrategy(
//	    100*time.Millisecond, // initial interval
//	    5*time.Second,        // max interval
//	    2.0,                  // multiplier
//	))
//
//	// Fixed interval
//	config.WithRetryStrategy(sdk.NewFixedIntervalStrategy(
//	    1*time.Second, // fixed interval
//	))
//
//	// Custom strategy
//	config.WithRetryStrategy(sdk.RetryStrategyFunc(
//	    func(attempt int) time.Duration {
//	        return time.Duration(attempt) * time.Second
//	    },
//	))
//
// # Observability
//
// Monitor SDK operations using the Observer interface:
//
//	type MyObserver struct{}
//
//	func (o *MyObserver) OnRequest(ctx context.Context, method, url string) {
//	    log.Printf("Request: %s %s", method, url)
//	}
//
//	func (o *MyObserver) OnResponse(ctx context.Context, method, url string,
//	                                statusCode int, duration time.Duration) {
//	    log.Printf("Response: %s %s - %d (%v)", method, url, statusCode, duration)
//	}
//
//	func (o *MyObserver) OnError(ctx context.Context, method, url string,
//	                             err error, duration time.Duration) {
//	    log.Printf("Error: %s %s - %v (%v)", method, url, err, duration)
//	}
//
//	config.WithObserver(&MyObserver{})
//
// # Type-Safe Operations
//
// The SDK provides type-safe wrappers for common types:
//
//	// Create a typed client for User structs
//	userClient := sdk.NewTypedClient[User](client)
//
//	user := User{ID: 1, Name: "Bob", Email: "bob@example.com"}
//	err := userClient.Set(ctx, "user:1", user)
//
//	retrieved, err := userClient.Get(ctx, "user:1")
//	fmt.Printf("User: %+v\n", retrieved)
//
// # Context Support
//
// All operations support context for cancellation and timeouts:
//
//	// With timeout
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	err := client.Set(ctx, "key", "value")
//
//	// With cancellation
//	ctx, cancel := context.WithCancel(context.Background())
//	go func() {
//	    time.Sleep(100 * time.Millisecond)
//	    cancel() // Cancel the operation
//	}()
//	err := client.Get(ctx, "key", &value)
//
// # Thread Safety
//
// The client is thread-safe and designed for concurrent use:
//
//	var wg sync.WaitGroup
//	for i := 0; i < 100; i++ {
//	    wg.Add(1)
//	    go func(n int) {
//	        defer wg.Done()
//	        client.Set(ctx, fmt.Sprintf("key-%d", n), n)
//	    }(i)
//	}
//	wg.Wait()
//
// # WASM Support
//
// The SDK supports WebAssembly for browser-based applications:
//
//	// Build for WASM
//	GOOS=js GOARCH=wasm go build -tags wasm -o main.wasm
//
// See examples/wasm for a complete browser-based example.
package sdk
