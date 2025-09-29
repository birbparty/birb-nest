package sdk

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryStrategy defines how retries should be performed.
// Different strategies provide different behaviors for retry intervals
// and determine which errors should trigger retries.
//
// The SDK provides several built-in strategies:
//   - ExponentialBackoffStrategy: Exponentially increasing delays
//   - LinearBackoffStrategy: Linear delay increases
//   - ConstantBackoffStrategy: Fixed delay between retries
//   - NoRetryStrategy: Disables retries entirely
//
// You can also implement custom strategies:
//
//	type CustomStrategy struct{}
//
//	func (s *CustomStrategy) NextInterval(attempt int) time.Duration {
//	    // Custom logic for calculating retry delay
//	    return time.Duration(attempt*attempt) * time.Second
//	}
//
//	func (s *CustomStrategy) ShouldRetry(err error, attempt int) bool {
//	    // Custom logic for determining if error is retryable
//	    return sdk.IsRetryable(err) && attempt < 5
//	}
type RetryStrategy interface {
	// NextInterval returns the delay before the next retry attempt.
	// The attempt parameter starts at 1 for the first retry.
	// Return 0 to indicate no more retries should be attempted.
	NextInterval(attempt int) time.Duration

	// ShouldRetry determines if the error is retryable for the given attempt.
	// This method can inspect the error type and the attempt count to decide
	// whether to continue retrying.
	ShouldRetry(err error, attempt int) bool
}

// RetryBudget limits retry attempts by count and duration to prevent
// excessive retries that could overwhelm the system or delay failure detection.
//
// Example:
//
//	budget := sdk.RetryBudget{
//	    MaxAttempts: 5,                              // Max 5 retries
//	    MaxDuration: 30 * time.Second,               // Max 30s total
//	    RetryableErrors: []sdk.ErrorType{            // Only retry specific errors
//	        sdk.ErrorTypeNetwork,
//	        sdk.ErrorTypeTimeout,
//	    },
//	}
//
//	strategy := &sdk.ExponentialBackoffStrategy{
//	    InitialInterval: 100 * time.Millisecond,
//	    MaxInterval:     5 * time.Second,
//	    Multiplier:      2.0,
//	    Budget:          budget,
//	}
type RetryBudget struct {
	// MaxAttempts is the maximum number of retry attempts.
	// Set to 0 for unlimited attempts (not recommended).
	MaxAttempts int

	// MaxDuration is the maximum total time for all retries.
	// This includes the time spent in retry delays.
	// Set to 0 for no time limit.
	MaxDuration time.Duration

	// RetryableErrors is a list of error types that are retryable.
	// If empty, all retryable errors are allowed.
	// Use this to limit retries to specific error types.
	RetryableErrors []ErrorType
}

// DefaultRetryBudget returns a retry budget with sensible defaults:
//   - MaxAttempts: 3 (up to 3 retry attempts)
//   - MaxDuration: 30s (max 30 seconds total)
//   - RetryableErrors: empty (all retryable errors allowed)
//
// Example:
//
//	budget := sdk.DefaultRetryBudget()
//	strategy := sdk.DefaultExponentialBackoff()
//	strategy.Budget = budget
func DefaultRetryBudget() RetryBudget {
	return RetryBudget{
		MaxAttempts: 3,
		MaxDuration: 30 * time.Second,
	}
}

// IsExhausted checks if the retry budget is exhausted
func (rb *RetryBudget) IsExhausted(attempt int, elapsed time.Duration) bool {
	if rb.MaxAttempts > 0 && attempt >= rb.MaxAttempts {
		return true
	}
	if rb.MaxDuration > 0 && elapsed >= rb.MaxDuration {
		return true
	}
	return false
}

// IsRetryable checks if an error is allowed by the budget
func (rb *RetryBudget) IsRetryable(err error) bool {
	if !IsRetryable(err) {
		return false
	}

	// If no specific error types are configured, allow all retryable errors
	if len(rb.RetryableErrors) == 0 {
		return true
	}

	// Check if error type is in allowed list
	var enhancedErr *Error
	if errors.As(err, &enhancedErr) {
		for _, allowed := range rb.RetryableErrors {
			if enhancedErr.Type == allowed {
				return true
			}
		}
	}

	return false
}

// ExponentialBackoffStrategy implements exponential backoff with jitter.
// This is the recommended retry strategy for most use cases as it:
//   - Reduces load on failing services by increasing delays
//   - Prevents thundering herd with jitter
//   - Caps maximum delay to prevent excessive waiting
//
// The delay calculation is:
//
//	base = InitialInterval * (Multiplier ^ (attempt-1))
//	delay = min(base, MaxInterval) ± jitter
//
// Example:
//
//	strategy := &sdk.ExponentialBackoffStrategy{
//	    InitialInterval: 100 * time.Millisecond, // Start with 100ms
//	    MaxInterval:     10 * time.Second,       // Cap at 10s
//	    Multiplier:      2.0,                    // Double each time
//	    Jitter:          0.3,                    // ±30% randomization
//	    Budget:          sdk.DefaultRetryBudget(),
//	}
//
//	config := sdk.DefaultConfig().
//	    WithRetryStrategy(strategy)
type ExponentialBackoffStrategy struct {
	// InitialInterval is the initial retry interval.
	// The first retry will wait approximately this long.
	InitialInterval time.Duration

	// MaxInterval is the maximum retry interval.
	// Delays will be capped at this value.
	MaxInterval time.Duration

	// Multiplier is the exponential growth factor.
	// Each retry interval is multiplied by this value.
	Multiplier float64

	// Jitter is the randomization factor (0.0 to 1.0).
	// 0.3 means ±30% randomization of the calculated interval.
	// Jitter helps prevent thundering herd problems.
	Jitter float64

	// Budget limits retry attempts by count and duration.
	Budget RetryBudget
}

// DefaultExponentialBackoff returns an exponential backoff strategy with sensible defaults:
//   - InitialInterval: 100ms
//   - MaxInterval: 5s
//   - Multiplier: 2.0 (doubles each retry)
//   - Jitter: 0.3 (±30% randomization)
//   - Budget: 3 attempts, 30s max duration
//
// This produces delays like: 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s, 5s, 5s...
// (with ±30% jitter applied to each)
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithRetryStrategy(sdk.DefaultExponentialBackoff())
func DefaultExponentialBackoff() *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
		Jitter:          0.3,
		Budget:          DefaultRetryBudget(),
	}
}

// NextInterval calculates the next retry interval
func (s *ExponentialBackoffStrategy) NextInterval(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate base interval
	interval := float64(s.InitialInterval) * math.Pow(s.Multiplier, float64(attempt-1))

	// Cap at max interval
	if interval > float64(s.MaxInterval) {
		interval = float64(s.MaxInterval)
	}

	// Apply jitter
	if s.Jitter > 0 {
		jitterRange := interval * s.Jitter
		jitter := jitterRange * (2*rand.Float64() - 1) // -jitterRange to +jitterRange
		interval += jitter
	}

	// Ensure non-negative
	if interval < 0 {
		interval = 0
	}

	return time.Duration(interval)
}

// ShouldRetry determines if the error is retryable
func (s *ExponentialBackoffStrategy) ShouldRetry(err error, attempt int) bool {
	return s.Budget.IsRetryable(err)
}

// LinearBackoffStrategy implements linear backoff with optional jitter.
// Each retry uses the same base interval, with optional randomization.
// This is simpler than exponential backoff but may not be as effective
// at reducing load on struggling services.
//
// Example:
//
//	strategy := &sdk.LinearBackoffStrategy{
//	    Interval: 1 * time.Second,   // 1s between retries
//	    Jitter:   0.1,               // ±10% randomization
//	    Budget:   sdk.DefaultRetryBudget(),
//	}
//
//	// Produces delays like: 1s±10%, 1s±10%, 1s±10%...
type LinearBackoffStrategy struct {
	// Interval is the fixed interval between retries.
	Interval time.Duration

	// Jitter is the randomization factor (0.0 to 1.0).
	// 0.1 means ±10% randomization of the interval.
	Jitter float64

	// Budget limits retry attempts.
	Budget RetryBudget
}

// DefaultLinearBackoff returns a linear backoff strategy with sensible defaults:
//   - Interval: 1s (fixed 1 second between retries)
//   - Jitter: 0.1 (±10% randomization)
//   - Budget: 3 attempts, 30s max duration
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithRetryStrategy(sdk.DefaultLinearBackoff())
func DefaultLinearBackoff() *LinearBackoffStrategy {
	return &LinearBackoffStrategy{
		Interval: 1 * time.Second,
		Jitter:   0.1,
		Budget:   DefaultRetryBudget(),
	}
}

// NextInterval returns the next retry interval
func (s *LinearBackoffStrategy) NextInterval(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	interval := float64(s.Interval)

	// Apply jitter
	if s.Jitter > 0 {
		jitterRange := interval * s.Jitter
		jitter := jitterRange * (2*rand.Float64() - 1)
		interval += jitter
	}

	// Ensure non-negative
	if interval < 0 {
		interval = 0
	}

	return time.Duration(interval)
}

// ShouldRetry determines if the error is retryable
func (s *LinearBackoffStrategy) ShouldRetry(err error, attempt int) bool {
	return s.Budget.IsRetryable(err)
}

// ConstantBackoffStrategy implements constant interval retries.
// Every retry uses exactly the same delay with no randomization.
// This is the simplest strategy but may cause thundering herd problems.
//
// Example:
//
//	strategy := &sdk.ConstantBackoffStrategy{
//	    Interval: 500 * time.Millisecond, // Always wait 500ms
//	    Budget:   sdk.DefaultRetryBudget(),
//	}
//
//	// Produces delays like: 500ms, 500ms, 500ms...
type ConstantBackoffStrategy struct {
	// Interval is the fixed interval between retries.
	Interval time.Duration

	// Budget limits retry attempts.
	Budget RetryBudget
}

// DefaultConstantBackoff returns a constant backoff strategy with sensible defaults:
//   - Interval: 500ms (fixed 500ms between retries)
//   - Budget: 3 attempts, 30s max duration
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithRetryStrategy(sdk.DefaultConstantBackoff())
func DefaultConstantBackoff() *ConstantBackoffStrategy {
	return &ConstantBackoffStrategy{
		Interval: 500 * time.Millisecond,
		Budget:   DefaultRetryBudget(),
	}
}

// NextInterval returns the next retry interval
func (s *ConstantBackoffStrategy) NextInterval(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	return s.Interval
}

// ShouldRetry determines if the error is retryable
func (s *ConstantBackoffStrategy) ShouldRetry(err error, attempt int) bool {
	return s.Budget.IsRetryable(err)
}

// NoRetryStrategy disables retries entirely.
// Use this when you want to handle errors immediately without any retry attempts.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithRetryStrategy(&sdk.NoRetryStrategy{})
//
//	client, _ := sdk.NewClient(config)
//	// Now all operations fail immediately without retries
type NoRetryStrategy struct{}

// NextInterval always returns 0
func (s *NoRetryStrategy) NextInterval(attempt int) time.Duration {
	return 0
}

// ShouldRetry always returns false
func (s *NoRetryStrategy) ShouldRetry(err error, attempt int) bool {
	return false
}

// retryExecutor handles retry execution with a given strategy
type retryExecutor struct {
	strategy RetryStrategy
}

// newRetryExecutor creates a new retry executor
func newRetryExecutor(strategy RetryStrategy) *retryExecutor {
	if strategy == nil {
		strategy = DefaultExponentialBackoff()
	}
	return &retryExecutor{strategy: strategy}
}

// Execute runs a function with retry logic
func (re *retryExecutor) Execute(ctx context.Context, fn func() error) error {
	var lastErr error
	startTime := time.Now()

	for attempt := 0; ; attempt++ {
		// Execute the function
		err := fn()

		// Success
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !re.strategy.ShouldRetry(err, attempt+1) {
			break
		}

		// Check context
		if ctx.Err() != nil {
			return WrapError(ctx.Err(), ErrorTypeTimeout, "context canceled during retry")
		}

		// Check retry budget
		if strategy, ok := re.strategy.(*ExponentialBackoffStrategy); ok {
			elapsed := time.Since(startTime)
			if strategy.Budget.IsExhausted(attempt+1, elapsed) {
				// Return the original error when budget is exhausted
				return lastErr
			}
		}

		// Also check for other strategy types
		if strategy, ok := re.strategy.(*LinearBackoffStrategy); ok {
			elapsed := time.Since(startTime)
			if strategy.Budget.IsExhausted(attempt+1, elapsed) {
				return lastErr
			}
		}

		if strategy, ok := re.strategy.(*ConstantBackoffStrategy); ok {
			elapsed := time.Since(startTime)
			if strategy.Budget.IsExhausted(attempt+1, elapsed) {
				return lastErr
			}
		}

		// Calculate next interval
		interval := re.strategy.NextInterval(attempt + 1)
		if interval <= 0 {
			break
		}

		// Wait for next attempt
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return WrapError(ctx.Err(), ErrorTypeTimeout, "context canceled during retry wait")
		case <-timer.C:
			// Continue to next attempt
		}
	}

	return lastErr
}

// HedgedRequest represents a hedged request configuration.
// Hedged requests send multiple identical requests with a delay between them,
// using the first successful response. This reduces tail latency at the cost
// of additional load.
//
// Hedging is most effective when:
//   - Latency variance is high
//   - The service can handle additional load
//   - Fast responses are more important than resource efficiency
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithHedgedRequests(sdk.HedgedRequest{
//	        MaxRequests: 2,                      // Send up to 2 requests
//	        Delay:       50 * time.Millisecond,  // Wait 50ms before 2nd request
//	    })
//
//	client, _ := sdk.NewClient(config)
//	// Now Get operations may send a second request after 50ms
//	// if the first hasn't completed yet
type HedgedRequest struct {
	// MaxRequests is the maximum number of concurrent requests.
	// Set to 1 to disable hedging.
	MaxRequests int

	// Delay is the delay between successive requests.
	// The second request starts this long after the first, and so on.
	Delay time.Duration
}

// DefaultHedgedRequest returns hedged request configuration with sensible defaults:
//   - MaxRequests: 2 (send up to 2 concurrent requests)
//   - Delay: 50ms (wait 50ms before sending second request)
//
// This configuration balances latency reduction with resource usage.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithHedgedRequests(sdk.DefaultHedgedRequest())
func DefaultHedgedRequest() HedgedRequest {
	return HedgedRequest{
		MaxRequests: 2,
		Delay:       50 * time.Millisecond,
	}
}

// hedgedExecutor handles hedged request execution
type hedgedExecutor struct {
	config HedgedRequest
}

// newHedgedExecutor creates a new hedged executor
func newHedgedExecutor(config HedgedRequest) *hedgedExecutor {
	return &hedgedExecutor{config: config}
}

// Execute runs a function with hedging
func (he *hedgedExecutor) Execute(ctx context.Context, fn func() error) error {
	if he.config.MaxRequests <= 1 {
		// No hedging, just execute once
		return fn()
	}

	// Channel for results
	type result struct {
		err   error
		index int
	}
	resultChan := make(chan result, he.config.MaxRequests)

	// Context for canceling other requests
	hedgeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Track first error and results
	var firstErr error
	resultsReceived := 0

	// Launch first request immediately
	go func() {
		err := fn()
		select {
		case resultChan <- result{err: err, index: 0}:
		case <-hedgeCtx.Done():
			// Context canceled, discard result
		}
	}()

	// Launch additional requests with delay
	for i := 1; i < he.config.MaxRequests; i++ {
		// Wait for delay or check for early result
		timer := time.NewTimer(he.config.Delay)
		select {
		case res := <-resultChan:
			timer.Stop()
			resultsReceived++
			if res.err == nil {
				// Success! Cancel other requests
				cancel()
				return nil
			}
			if firstErr == nil {
				firstErr = res.err
			}
			// Got a failed result, but continue launching next request

		case <-timer.C:
			// Timer expired, launch next request

		case <-ctx.Done():
			timer.Stop()
			cancel()
			return WrapError(ctx.Err(), ErrorTypeTimeout, "context canceled during hedged request")
		}

		// Launch next request
		go func(idx int) {
			err := fn()
			select {
			case resultChan <- result{err: err, index: idx}:
			case <-hedgeCtx.Done():
				// Context canceled, discard result
			}
		}(i)
	}

	// Wait for remaining results
	for resultsReceived < he.config.MaxRequests {
		select {
		case res := <-resultChan:
			resultsReceived++
			if res.err == nil {
				// Success! Cancel other requests
				cancel()
				return nil
			}
			if firstErr == nil {
				firstErr = res.err
			}

		case <-ctx.Done():
			// Original context canceled
			cancel()
			return WrapError(ctx.Err(), ErrorTypeTimeout, "context canceled during hedged request")
		}
	}

	// All requests failed
	return firstErr
}
