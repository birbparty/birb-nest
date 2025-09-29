# 🦜 Birb Nest SDK - Advanced Error Handling Demo

This interactive demo showcases the advanced error handling and resilience features of the Birb Nest SDK, including circuit breakers, retry strategies, and comprehensive error handling.

## 🎯 Features Demonstrated

- **Circuit Breaker Pattern**: Automatic failure detection and recovery
- **Retry Strategies**: Exponential backoff with jitter and retry budgets
- **Error Classification**: Different error types and their handling
- **Visual Feedback**: Color-coded terminal output for easy understanding
- **Interactive Mode**: Manual testing and experimentation

## 🚀 Running the Demo

### Automated Scenarios
```bash
go run main.go
```

This runs through 6 pre-programmed scenarios that demonstrate:
1. **Normal Operations** - Regular operations with 10% failure rate
2. **Increasing Failure Rate** - Gradual failure increase to trigger circuit breaker
3. **Server Outage** - Complete server unavailability simulation
4. **Recovery Demonstration** - Full cycle from failures to recovery
5. **Different Error Types** - Various error conditions and their handling
6. **Retry Budget Exhaustion** - What happens when retry limits are reached

### Interactive Mode
```bash
go run main.go --interactive
```

Or select interactive mode after the automated demo completes.

## 🎮 Interactive Commands

| Command | Description | Example |
|---------|-------------|---------|
| `set <key> <value>` | Store a value | `set user:123 data` |
| `get <key>` | Retrieve a value | `get user:123` |
| `fail <rate>` | Set failure rate (0.0-1.0) | `fail 0.5` |
| `outage <seconds>` | Simulate server outage | `outage 10` |
| `stats` | Show current statistics | `stats` |
| `reset` | Reset circuit breaker | `reset` |
| `quit` | Exit interactive mode | `quit` |

## 🎨 Visual Indicators

The demo uses color-coded output for clarity:

- 🟢 **Green** `[SUCCESS]` - Successful operations
- 🔴 **Red** `[Failed]` - Failed operations
- 🟡 **Yellow** `[RETRY]` - Retry attempts
- ⚡ **Yellow** `[Circuit Open]` - Circuit breaker is open
- 🔵 **Blue** - Scenario headers
- 🟣 **Purple** - Interactive mode
- 🔗 **Cyan** - Welcome/goodbye messages
- ⚪ **Gray** - Informational text

## 📊 Understanding Circuit Breaker States

The circuit breaker has three states:

1. **Closed** ✓ - Normal operation, requests pass through
2. **Open** ✗ - Too many failures, requests are blocked
3. **Half-Open** ⚡ - Testing if service has recovered

State transitions:
```
Closed → Open (after failure threshold reached)
Open → Half-Open (after timeout period)
Half-Open → Closed (if test requests succeed)
Half-Open → Open (if test requests fail)
```

## 🔧 Configuration

The demo uses these default settings:

```go
// Circuit Breaker
FailureThreshold: 5      // Opens after 5 failures
SuccessThreshold: 2      // Closes after 2 successes in half-open
Timeout: 10 seconds      // Time before trying half-open
HalfOpenRequests: 3      // Max requests in half-open state

// Retry Strategy
InitialInterval: 100ms   // First retry delay
MaxInterval: 2s          // Maximum retry delay
Multiplier: 2.0          // Exponential growth factor
Jitter: 0.3              // ±30% randomization
MaxAttempts: 3           // Maximum retry attempts
MaxDuration: 10s         // Maximum total retry time
```

## 📈 Metrics Displayed

The demo tracks and displays:
- **Total Requests**: Number of operations attempted
- **Total Failures**: Number of failed operations
- **Failure Rate**: Percentage of failures
- **Circuit State**: Current circuit breaker state
- **SDK Retries**: Total retry attempts made

## 🎯 Learning Objectives

This demo helps you understand:

1. **When Circuit Breakers Activate**: See exactly when and why the circuit opens
2. **Retry Behavior**: Watch exponential backoff with jitter in action
3. **Error Types**: Learn which errors are retryable vs permanent
4. **Recovery Patterns**: Observe how the system recovers from failures
5. **Budget Management**: See what happens when retry budgets are exhausted

## 💡 Tips for Exploration

- Try different failure rates to see how the circuit breaker responds
- Simulate outages of varying lengths
- Watch the retry delays increase with each attempt
- Notice how the circuit breaker prevents cascading failures
- Experiment with the stats command to track metrics

## 🐛 Troubleshooting

If you see compilation errors:
```bash
go mod tidy
```

If colors don't display correctly on Windows:
- Use Windows Terminal or enable ANSI escape sequences
- Or run in WSL/WSL2 for full color support

## 📚 Related Examples

- [Basic Example](../basic/) - Simple usage patterns
- [WASM Example](../wasm/) - Browser-based demo
- [Integration Tests](../../integration_test.go) - Real server testing

## 🦜 Have Fun!

This demo is designed to be interactive and educational. Play around with different scenarios, break things, and watch how the SDK handles various failure conditions. The visual feedback makes it easy to understand what's happening under the hood!
