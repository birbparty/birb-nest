# Birb Nest Go SDK

A high-performance Go client library for the Birb Nest persistent cache service. This SDK provides a simple and efficient way to interact with the Birb Nest API with zero external dependencies.

## Features

- ✅ **Zero external dependencies** - Uses only Go standard library
- ✅ **Thread-safe** operations
- ✅ **Connection pooling** for efficient HTTP communication
- ✅ **Automatic retries** with exponential backoff
- ✅ **Context support** for cancellation and timeouts
- ✅ **Type-safe operations** with JSON serialization
- ✅ **WASM compatible** (with build tags)
- ✅ **Comprehensive error handling**
- ✅ **Circuit breaker** for fault tolerance
- ✅ **Observability** with metrics and logging

## Installation

```bash
go get github.com/birbparty/birb-nest/sdk
```

## 5-Minute Quick Start

### 1. Basic Setup (1 minute)

Create a new Go file `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/birbparty/birb-nest/sdk"
)

func main() {
    // Create client with default settings
    client, err := sdk.NewClient(sdk.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    ctx := context.Background()
    
    // Your code here...
}
```

### 2. Store and Retrieve Data (2 minutes)

Add this to your main function:

```go
// Store a simple string
err = client.Set(ctx, "greeting", "Hello, Birb Nest!")
if err != nil {
    log.Printf("Failed to set: %v", err)
}

// Retrieve the string
var greeting string
err = client.Get(ctx, "greeting", &greeting)
if err != nil {
    log.Printf("Failed to get: %v", err)
} else {
    fmt.Println(greeting) // Output: Hello, Birb Nest!
}

// Store a struct
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
err = client.Set(ctx, "user:1", user)
if err != nil {
    log.Printf("Failed to set user: %v", err)
}

// Retrieve the struct
var retrievedUser User
err = client.Get(ctx, "user:1", &retrievedUser)
if err != nil {
    log.Printf("Failed to get user: %v", err)
} else {
    fmt.Printf("User: %+v\n", retrievedUser)
}
```

### 3. Handle Errors Gracefully (1 minute)

Add error handling:

```go
// Check if key exists
var data string
err = client.Get(ctx, "nonexistent", &data)
if sdk.IsNotFound(err) {
    fmt.Println("Key not found, creating default...")
    client.Set(ctx, "nonexistent", "default value")
} else if err != nil {
    log.Printf("Unexpected error: %v", err)
}

// Delete a key
err = client.Delete(ctx, "greeting")
if err != nil {
    log.Printf("Failed to delete: %v", err)
}
```

### 4. Run Your Application (1 minute)

```bash
# Ensure Birb Nest is running (via Docker)
docker run -d -p 8080:8080 birbparty/birb-nest:latest

# Run your application
go run main.go
```

### Complete Example

Here's the full working example:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/birbparty/birb-nest/sdk"
)

func main() {
    // Create client
    client, err := sdk.NewClient(sdk.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    ctx := context.Background()
    
    // Store and retrieve string
    client.Set(ctx, "greeting", "Hello, Birb Nest!")
    
    var greeting string
    client.Get(ctx, "greeting", &greeting)
    fmt.Println(greeting)
    
    // Store and retrieve struct
    type User struct {
        ID    int    `json:"id"`
        Name  string `json:"name"`
    }
    
    user := User{ID: 1, Name: "Alice"}
    client.Set(ctx, "user:1", user)
    
    var retrievedUser User
    client.Get(ctx, "user:1", &retrievedUser)
    fmt.Printf("User: %+v\n", retrievedUser)
    
    // Cleanup
    client.Delete(ctx, "greeting")
    client.Delete(ctx, "user:1")
}
```

## Configuration

```go
config := sdk.DefaultConfig().
    WithBaseURL("http://localhost:8080").
    WithTimeout(10 * time.Second).
    WithRetries(3).
    WithHeader("X-API-Key", "your-api-key")

client, err := sdk.NewClient(config)
```

### Configuration Options

- `BaseURL`: The base URL of the Birb Nest API (default: `http://localhost:8080`)
- `Timeout`: HTTP request timeout (default: `30s`)
- `RetryConfig`:
  - `MaxRetries`: Maximum number of retry attempts (default: `3`)
  - `InitialInterval`: Initial retry interval (default: `100ms`)
  - `MaxInterval`: Maximum retry interval (default: `5s`)
  - `Multiplier`: Exponential backoff multiplier (default: `2.0`)
- `TransportConfig`:
  - `MaxIdleConns`: Maximum idle connections (default: `100`)
  - `MaxConnsPerHost`: Maximum connections per host (default: `10`)
  - `IdleConnTimeout`: Idle connection timeout (default: `90s`)

## Extended Client

For advanced features, use the `ExtendedClient`:

```go
client, err := sdk.NewExtendedClient(config)

// Set with TTL
ttl := 30 * time.Second
opts := &sdk.SetOptions{
    TTL: &ttl,
}
err = client.SetWithOptions(ctx, "temp-key", "temp-value", opts)

// Check if key exists
exists, err := client.Exists(ctx, "my-key")

// Get multiple keys
keys := []string{"key1", "key2", "key3"}
values, err := client.GetMultiple(ctx, keys)
```

## Data Types

The SDK supports storing any JSON-serializable data:

```go
// Strings
client.Set(ctx, "name", "Birb McFly")

// Numbers
client.Set(ctx, "count", 42)

// Structs
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

user := User{ID: 1, Name: "Test", Email: "test@example.com"}
client.Set(ctx, "user:1", user)

// Raw JSON
jsonData := json.RawMessage(`{"type": "bird", "species": "parrot"}`)
client.Set(ctx, "bird-data", jsonData)
```

## Error Handling

```go
err := client.Get(ctx, "non-existent", &value)
if sdk.IsNotFound(err) {
    // Handle not found
} else if err != nil {
    // Handle other errors
}

// Check if error is retryable
if sdk.IsRetryable(err) {
    // Error is transient and operation can be retried
}
```

## Migration from Other Cache Clients

### From Redis Client

```go
// Redis client
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
rdb.Set(ctx, "key", "value", 0)

// Birb-Nest SDK (similar API)
client, _ := sdk.NewClient(sdk.DefaultConfig())
client.Set(ctx, "key", "value")
```

### From Memcached

```go
// Memcached
mc := memcache.New("localhost:11211")
mc.Set(&memcache.Item{Key: "key", Value: []byte("value")})

// Birb-Nest SDK
client, _ := sdk.NewClient(sdk.DefaultConfig())
client.Set(ctx, "key", "value")
```

## Performance Best Practices

### 1. Reuse Client Instances

```go
// ❌ Don't create a new client for each request
func handleRequest() {
    client, _ := sdk.NewClient(sdk.DefaultConfig())
    defer client.Close()
    // ...
}

// ✅ Create once and reuse
var client *sdk.Client

func init() {
    client, _ = sdk.NewClient(sdk.DefaultConfig())
}

func handleRequest() {
    // Use the global client
    client.Get(ctx, "key", &value)
}
```

### 2. Use Context for Timeouts

```go
// Set operation timeout
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

err := client.Get(ctx, "key", &value)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("Operation timed out")
    }
}
```

### 3. Batch Operations When Possible

```go
// Use ExtendedClient for batch operations
extClient, _ := sdk.NewExtendedClient(config)

// Get multiple keys at once
keys := []string{"key1", "key2", "key3"}
values, err := extClient.GetMultiple(ctx, keys)
```

### 4. Monitor Cache Hit Rate

```go
// Implement observer for metrics
type MetricsObserver struct {
    hits   int64
    misses int64
}

func (m *MetricsObserver) OnCacheHit(key string) {
    atomic.AddInt64(&m.hits, 1)
}

func (m *MetricsObserver) OnCacheMiss(key string) {
    atomic.AddInt64(&m.misses, 1)
}

// Attach observer
observer := &MetricsObserver{}
client.SetObserver(observer)
```

## Connection Management

The client maintains a connection pool for efficient HTTP communication:

```go
// Client reuses connections automatically
for i := 0; i < 1000; i++ {
    client.Set(ctx, fmt.Sprintf("key-%d", i), "value")
}

// Always close the client when done
defer client.Close()
```

## Context Support

All operations support context for cancellation and timeouts:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
err := client.Set(ctx, "key", "value")

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(100 * time.Millisecond)
    cancel() // Cancel the operation
}()
err := client.Get(ctx, "key", &value)
```

## Thread Safety

The client is thread-safe and can be used concurrently:

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        client.Set(ctx, fmt.Sprintf("key-%d", n), n)
    }(i)
}
wg.Wait()
```

## Examples

See the [examples](examples/) directory for more detailed examples:

- [Basic usage](examples/basic/) - Simple CRUD operations
- [Advanced usage](examples/advanced/) - Connection resilience, monitoring, and advanced patterns
- [WASM usage](examples/wasm/) - Browser-based WASM example

## Testing

Run tests:

```bash
cd sdk
go test -v
```

Run benchmarks:

```bash
cd sdk
go test -bench=. -benchmem
```

## Performance

The SDK is designed for high performance:

- Connection pooling reduces latency
- Minimal allocations in hot paths
- Efficient JSON serialization
- Smart retry logic with jitter

Benchmark results on Apple M4 Max:
```
BenchmarkClient_Set-16    26433    42168 ns/op    10719 B/op    136 allocs/op
BenchmarkClient_Get-16    29257    39687 ns/op     8516 B/op    105 allocs/op
```

## Operational Documentation

For deployment and operations teams:

- [Operations Guide](docs/OPERATIONS.md) - Deployment and configuration
- [Troubleshooting Guide](docs/TROUBLESHOOTING.md) - Error dictionary and solutions
- [On-Call Runbook](docs/ON_CALL_RUNBOOK.md) - Emergency procedures
- [Monitoring Guide](docs/MONITORING.md) - Metrics and alerting
- [Logging Guide](docs/LOGGING.md) - Log management and analysis

## License

Same as the parent project - See LICENSE file in the root directory.
