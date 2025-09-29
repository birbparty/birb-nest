# Basic Birb-Nest SDK Examples

This directory contains basic examples demonstrating core functionality of the Birb-Nest SDK.

## Examples

### 1. Simple Cache (`simple-cache/`)

Demonstrates basic cache operations:
- String storage and retrieval
- Struct serialization
- Error handling
- Key naming patterns

**Run it:**
```bash
cd simple-cache
go run main.go
```

### 2. Connection Resilience (`connection-resilience/`)

Shows how the SDK handles failures:
- Automatic retries
- Circuit breaker pattern
- Graceful degradation
- Timeout handling

**Run it:**
```bash
cd connection-resilience
go run main.go
```

### 3. Monitoring (`monitoring/`)

Demonstrates metrics collection and exposure:
- Custom metrics collector
- Prometheus endpoint
- Performance tracking
- Error categorization

**Run it:**
```bash
cd monitoring
go run main.go
# Then visit http://localhost:9090/metrics
```

## Prerequisites

1. Ensure Birb-Nest is running:
```bash
docker run -d -p 8080:8080 birbparty/birb-nest:latest
```

2. Install the SDK:
```bash
go get github.com/birbparty/birb-nest/sdk
```

## Common Patterns

### Error Handling
```go
err := client.Get(ctx, "key", &value)
if sdk.IsNotFound(err) {
    // Key doesn't exist - this is often normal
} else if err != nil {
    // Handle actual error
}
```

### Context Usage
```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

### Connection Management
```go
// Create once, reuse everywhere
client, err := sdk.NewClient(sdk.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

## Next Steps

Once comfortable with these basics, check out the [advanced examples](../advanced/) for:
- High availability patterns
- Load balancing strategies
- Rate limiting implementation
