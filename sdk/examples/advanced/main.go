package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/birbparty/birb-nest/sdk"
)

// Colors for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
	colorBold   = "\033[1m"
)

// SimulatedServer simulates various server conditions
type SimulatedServer struct {
	mu             sync.Mutex
	failureRate    float64
	latencyMin     time.Duration
	latencyMax     time.Duration
	isDown         bool
	downUntil      time.Time
	requestCount   int
	failureCount   int
	circuitBreaker sdk.CircuitBreaker
}

// NewSimulatedServer creates a new simulated server
func NewSimulatedServer() *SimulatedServer {
	return &SimulatedServer{
		failureRate: 0.1, // 10% failure rate initially
		latencyMin:  50 * time.Millisecond,
		latencyMax:  200 * time.Millisecond,
		circuitBreaker: sdk.NewCircuitBreaker(sdk.CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          10 * time.Second,
			HalfOpenRequests: 3,
		}),
	}
}

// SimulateRequest simulates a request with various conditions
func (s *SimulatedServer) SimulateRequest() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestCount++

	// Simulate latency
	latency := s.latencyMin + time.Duration(rand.Float64()*float64(s.latencyMax-s.latencyMin))
	time.Sleep(latency)

	// Check if server is down
	if s.isDown && time.Now().Before(s.downUntil) {
		s.failureCount++
		return &sdk.NetworkError{Op: "request", Err: errors.New("connection refused")}
	}

	// Random failures based on failure rate
	if rand.Float64() < s.failureRate {
		s.failureCount++
		errorTypes := []error{
			&sdk.APIError{StatusCode: 500, Message: "Internal Server Error"},
			&sdk.APIError{StatusCode: 503, Message: "Service Unavailable"},
			&sdk.TimeoutError{Op: "request"},
			&sdk.NetworkError{Op: "request", Err: errors.New("network unreachable")},
		}
		return errorTypes[rand.Intn(len(errorTypes))]
	}

	return nil
}

// SetFailureRate adjusts the failure rate
func (s *SimulatedServer) SetFailureRate(rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failureRate = rate
}

// SimulateOutage simulates a server outage
func (s *SimulatedServer) SimulateOutage(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isDown = true
	s.downUntil = time.Now().Add(duration)
}

// GetStats returns current stats
func (s *SimulatedServer) GetStats() (requests, failures int, rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rate = 0
	if s.requestCount > 0 {
		rate = float64(s.failureCount) / float64(s.requestCount)
	}
	return s.requestCount, s.failureCount, rate
}

// DemoObserver tracks operations for visualization
type DemoObserver struct {
	mu          sync.Mutex
	requests    int
	errors      int
	retries     int
	lastCircuit sdk.CircuitState
}

func (o *DemoObserver) OnRequestStart(method, path string) {
	o.mu.Lock()
	o.requests++
	o.mu.Unlock()
}

func (o *DemoObserver) OnRequestEnd(method, path string, duration time.Duration, err error) {
	if err != nil {
		o.mu.Lock()
		o.errors++
		o.mu.Unlock()
	}
}

func (o *DemoObserver) OnRetryAttempt(method, path string, attempt int, delay time.Duration, err error) {
	o.mu.Lock()
	o.retries++
	o.mu.Unlock()

	// Visualize retry attempts
	fmt.Printf("%s[RETRY]%s Attempt %d after %v delay - Error: %v\n",
		colorYellow, colorReset, attempt, delay, err)
}

func (o *DemoObserver) OnCircuitBreakerStateChange(endpoint string, oldState, newState sdk.CircuitState) {
	o.mu.Lock()
	o.lastCircuit = newState
	o.mu.Unlock()

	color := colorGreen
	icon := "✓"

	switch newState {
	case sdk.CircuitOpen:
		color = colorRed
		icon = "✗"
	case sdk.CircuitHalfOpen:
		color = colorYellow
		icon = "⚡"
	}

	fmt.Printf("\n%s%s[CIRCUIT BREAKER]%s State changed from %s to %s%s %s\n\n",
		colorBold, color, colorReset, oldState, color, newState, icon)
}

func (o *DemoObserver) OnCacheHit(key string)  {}
func (o *DemoObserver) OnCacheMiss(key string) {}

// AdvancedDemo demonstrates advanced error handling and resilience features
type AdvancedDemo struct {
	client   sdk.Client
	server   *SimulatedServer
	observer *DemoObserver
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewAdvancedDemo creates a new demo instance
func NewAdvancedDemo() *AdvancedDemo {
	observer := &DemoObserver{}
	server := NewSimulatedServer()

	// Create config with advanced features
	config := sdk.DefaultConfig().
		WithBaseURL("http://localhost:8080").
		WithTimeout(5 * time.Second).
		WithRetries(3).
		WithCircuitBreaker(sdk.CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          10 * time.Second,
			HalfOpenRequests: 3,
		}).
		WithRetryStrategy(&sdk.ExponentialBackoffStrategy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     2 * time.Second,
			Multiplier:      2.0,
			Jitter:          0.3,
			Budget: sdk.RetryBudget{
				MaxAttempts: 3,
				MaxDuration: 10 * time.Second,
			},
		}).
		WithObserver(observer)

	// For demo purposes, we'll simulate the transport behavior
	// In real usage, the SDK handles all of this internally
	client, _ := sdk.NewClient(config)

	ctx, cancel := context.WithCancel(context.Background())

	return &AdvancedDemo{
		client:   client,
		server:   server,
		observer: observer,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Run executes the demo scenarios
func (d *AdvancedDemo) Run() {
	fmt.Printf("%s%s🦜 Birb Nest SDK - Advanced Error Handling Demo%s\n\n", colorBold, colorCyan, colorReset)

	scenarios := []struct {
		name string
		fn   func()
	}{
		{"Normal Operations", d.scenarioNormalOps},
		{"Increasing Failure Rate", d.scenarioIncreasingFailures},
		{"Server Outage", d.scenarioServerOutage},
		{"Recovery Demonstration", d.scenarioRecovery},
		{"Different Error Types", d.scenarioErrorTypes},
		{"Retry Budget Exhaustion", d.scenarioRetryBudget},
	}

	for i, scenario := range scenarios {
		fmt.Printf("\n%s%s=== Scenario %d: %s ===%s\n\n", colorBold, colorBlue, i+1, scenario.name, colorReset)
		scenario.fn()

		// Show stats after each scenario
		d.showStats()

		if i < len(scenarios)-1 {
			fmt.Printf("\nPress Enter to continue to next scenario...")
			fmt.Scanln()
		}
	}

	fmt.Printf("\n%s%s✨ Demo Complete! ✨%s\n", colorBold, colorGreen, colorReset)
}

// scenarioNormalOps demonstrates normal operations
func (d *AdvancedDemo) scenarioNormalOps() {
	fmt.Println("Demonstrating normal operations with occasional failures...")
	d.server.SetFailureRate(0.1) // 10% failure rate

	for i := 0; i < 10; i++ {
		d.performOperation(fmt.Sprintf("normal-op-%d", i))
		time.Sleep(200 * time.Millisecond)
	}
}

// scenarioIncreasingFailures demonstrates circuit breaker activation
func (d *AdvancedDemo) scenarioIncreasingFailures() {
	fmt.Println("Gradually increasing failure rate to trigger circuit breaker...")

	rates := []float64{0.3, 0.5, 0.7, 0.9, 1.0}
	for _, rate := range rates {
		d.server.SetFailureRate(rate)
		fmt.Printf("\n%sFailure rate: %.0f%%%s\n", colorGray, rate*100, colorReset)

		for i := 0; i < 5; i++ {
			d.performOperation(fmt.Sprintf("high-failure-%d", i))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// scenarioServerOutage simulates complete server outage
func (d *AdvancedDemo) scenarioServerOutage() {
	fmt.Println("Simulating complete server outage for 5 seconds...")
	d.server.SimulateOutage(5 * time.Second)

	// Reset failure rate for clear demonstration
	d.server.SetFailureRate(0)

	start := time.Now()
	for time.Since(start) < 7*time.Second {
		d.performOperation("outage-test")
		time.Sleep(500 * time.Millisecond)
	}
}

// scenarioRecovery demonstrates recovery after failures
func (d *AdvancedDemo) scenarioRecovery() {
	fmt.Println("Demonstrating recovery: failures -> circuit open -> recovery -> circuit closed...")

	// Phase 1: Cause failures to open circuit
	fmt.Printf("\n%sPhase 1: Causing failures%s\n", colorGray, colorReset)
	d.server.SetFailureRate(1.0)
	for i := 0; i < 10; i++ {
		d.performOperation("recovery-fail")
		time.Sleep(100 * time.Millisecond)
	}

	// Phase 2: Wait for timeout
	fmt.Printf("\n%sPhase 2: Waiting for circuit timeout (10 seconds)...%s\n", colorGray, colorReset)
	time.Sleep(10 * time.Second)

	// Phase 3: Successful recovery
	fmt.Printf("\n%sPhase 3: Server recovered, testing half-open state%s\n", colorGray, colorReset)
	d.server.SetFailureRate(0) // Server is healthy now
	for i := 0; i < 5; i++ {
		d.performOperation("recovery-success")
		time.Sleep(200 * time.Millisecond)
	}
}

// scenarioErrorTypes demonstrates different error types
func (d *AdvancedDemo) scenarioErrorTypes() {
	fmt.Println("Demonstrating different error types and their handling...")

	// This would normally interact with a real server
	// For demo, we'll show error type detection
	operations := []struct {
		name string
		err  error
	}{
		{"timeout", &sdk.TimeoutError{Op: "request"}},
		{"network", &sdk.NetworkError{Op: "connect", Err: errors.New("connection refused")}},
		{"server-500", &sdk.APIError{StatusCode: 500, Message: "Internal Server Error"}},
		{"server-503", &sdk.APIError{StatusCode: 503, Message: "Service Unavailable"}},
		{"rate-limit", &sdk.APIError{StatusCode: 429, Message: "Too Many Requests"}},
		{"not-found", &sdk.APIError{StatusCode: 404, Message: "Not Found"}},
	}

	for _, op := range operations {
		d.handleError(op.name, op.err)
		time.Sleep(500 * time.Millisecond)
	}
}

// scenarioRetryBudget demonstrates retry budget exhaustion
func (d *AdvancedDemo) scenarioRetryBudget() {
	fmt.Println("Demonstrating retry budget exhaustion...")

	// Set very high failure rate
	d.server.SetFailureRate(1.0)

	// Create a client with limited retry budget
	config := sdk.DefaultConfig().
		WithRetryStrategy(&sdk.ExponentialBackoffStrategy{
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     500 * time.Millisecond,
			Multiplier:      2.0,
			Budget: sdk.RetryBudget{
				MaxAttempts: 3,
				MaxDuration: 2 * time.Second,
			},
		}).
		WithObserver(d.observer)

	client, _ := sdk.NewClient(config)

	start := time.Now()
	err := client.Set(d.ctx, "budget-test", "value")
	duration := time.Since(start)

	fmt.Printf("\nOperation failed after %v (budget exhausted)\n", duration)
	if err != nil {
		d.handleError("budget-exhaustion", err)
	}

	// Reset for next scenario
	d.server.SetFailureRate(0.1)
}

// performOperation simulates a cache operation
func (d *AdvancedDemo) performOperation(key string) {
	// Simulate the operation with our mock server
	err := d.server.circuitBreaker.Execute(func() error {
		return d.server.SimulateRequest()
	})

	if err != nil {
		d.handleError(key, err)
	} else {
		fmt.Printf("%s[SUCCESS]%s Set key: %s%s ✓%s\n",
			colorGreen, colorReset, colorGray, key, colorReset)
	}
}

// handleError demonstrates error type detection and handling
func (d *AdvancedDemo) handleError(operation string, err error) {
	if err == nil {
		return
	}

	// Determine error type and appropriate handling
	color := colorRed
	icon := "✗"
	action := "Failed"

	// Check for specific error types
	if errors.Is(err, sdk.ErrCircuitOpen) {
		color = colorYellow
		icon = "⚡"
		action = "Circuit Open"
	} else if sdk.IsRetryable(err) {
		color = colorYellow
		icon = "↻"
		action = "Retryable"
	}

	// Get detailed error information
	var details string
	var sdkErr *sdk.Error
	if errors.As(err, &sdkErr) {
		details = fmt.Sprintf(" [Type: %s, Retryable: %v]", sdkErr.Type, sdkErr.IsRetryable())
	}

	fmt.Printf("%s[%s]%s %s: %v%s %s\n",
		color, action, colorReset, operation, err, details, icon)
}

// showStats displays current statistics
func (d *AdvancedDemo) showStats() {
	requests, failures, rate := d.server.GetStats()

	fmt.Printf("\n%s--- Statistics ---%s\n", colorGray, colorReset)
	fmt.Printf("Total Requests: %d\n", requests)
	fmt.Printf("Total Failures: %d\n", failures)
	fmt.Printf("Failure Rate: %.1f%%\n", rate*100)
	fmt.Printf("Circuit State: %s\n", d.server.circuitBreaker.State())
	fmt.Printf("SDK Retries: %d\n", d.observer.retries)
}

// InteractiveMode allows manual testing
func (d *AdvancedDemo) InteractiveMode() {
	fmt.Printf("\n%s%s🎮 Interactive Mode%s\n", colorBold, colorPurple, colorReset)
	fmt.Println("\nCommands:")
	fmt.Println("  set <key> <value> - Store a value")
	fmt.Println("  get <key>         - Retrieve a value")
	fmt.Println("  fail <rate>       - Set failure rate (0.0-1.0)")
	fmt.Println("  outage <seconds>  - Simulate outage")
	fmt.Println("  stats             - Show statistics")
	fmt.Println("  reset             - Reset circuit breaker")
	fmt.Println("  quit              - Exit")

	for {
		fmt.Printf("\n> ")
		var cmd string
		fmt.Scanln(&cmd)

		switch cmd {
		case "quit", "exit":
			return

		case "stats":
			d.showStats()

		case "reset":
			d.server.circuitBreaker.Reset()
			fmt.Printf("%sCircuit breaker reset%s\n", colorGreen, colorReset)

		default:
			var arg1, arg2 string
			fmt.Sscanf(cmd, "%s %s %s", &cmd, &arg1, &arg2)

			switch cmd {
			case "set":
				d.performOperation(arg1)

			case "get":
				d.performOperation(arg1)

			case "fail":
				var rate float64
				fmt.Sscanf(arg1, "%f", &rate)
				d.server.SetFailureRate(rate)
				fmt.Printf("Failure rate set to %.1f%%\n", rate*100)

			case "outage":
				var seconds int
				fmt.Sscanf(arg1, "%d", &seconds)
				d.server.SimulateOutage(time.Duration(seconds) * time.Second)
				fmt.Printf("Simulating %d second outage\n", seconds)

			default:
				fmt.Println("Unknown command")
			}
		}
	}
}

// Cleanup releases resources
func (d *AdvancedDemo) Cleanup() {
	d.cancel()
	d.client.Close()
}

func main() {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Create and run demo
	demo := NewAdvancedDemo()
	defer demo.Cleanup()

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Printf("\n\n%sInterrupted. Cleaning up...%s\n", colorYellow, colorReset)
		demo.Cleanup()
		os.Exit(0)
	}()

	// Check for interactive mode
	if len(os.Args) > 1 && os.Args[1] == "--interactive" {
		demo.InteractiveMode()
	} else {
		demo.Run()

		fmt.Printf("\n\nWould you like to try interactive mode? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" {
			demo.InteractiveMode()
		}
	}

	fmt.Printf("\n%s👋 Thanks for trying the Birb Nest SDK!%s\n", colorCyan, colorReset)
}
