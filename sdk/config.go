package sdk

import (
	"time"
)

// Config holds the configuration for the Birb Nest client.
// All fields are optional and have sensible defaults.
//
// Configuration can be built using the fluent builder pattern:
//
//	config := sdk.DefaultConfig().
//	    WithBaseURL("https://cache.example.com").
//	    WithTimeout(30 * time.Second).
//	    WithRetries(5).
//	    WithCircuitBreaker(sdk.CircuitBreakerConfig{
//	        FailureThreshold: 10,
//	        Timeout: 60 * time.Second,
//	    })
//
//	client, err := sdk.NewClient(config)
type Config struct {
	// BaseURL is the base URL of the Birb Nest API.
	// Default: "http://localhost:8080"
	BaseURL string

	// Timeout is the HTTP request timeout.
	// This includes connection time, any redirects, and reading the response body.
	// Default: 30s
	Timeout time.Duration

	// RetryConfig holds retry-related settings.
	// Configures automatic retry behavior for failed requests.
	RetryConfig RetryConfig

	// TransportConfig holds HTTP transport settings.
	// Configures connection pooling and keep-alive behavior.
	TransportConfig TransportConfig

	// Headers are custom headers to include in all requests.
	// Useful for authentication tokens, correlation IDs, etc.
	// Example: {"X-API-Key": "secret", "X-Request-ID": "12345"}
	Headers map[string]string

	// CircuitBreakerConfig holds circuit breaker settings.
	// If nil, circuit breaker is disabled.
	CircuitBreakerConfig *CircuitBreakerConfig

	// RetryStrategy defines the retry strategy to use.
	// If nil, exponential backoff strategy is used.
	RetryStrategy RetryStrategy

	// Observer for monitoring operations.
	// Allows tracking of requests, responses, and errors.
	// If nil, NoopObserver is used.
	Observer Observer

	// HedgedRequestConfig for hedged requests.
	// If set, enables sending parallel requests to reduce tail latency.
	HedgedRequestConfig *HedgedRequest

	// EnablePerEndpointCircuitBreaker enables per-endpoint circuit breakers.
	// When true, each endpoint has its own circuit breaker state.
	// When false, a single circuit breaker is used for all endpoints.
	EnablePerEndpointCircuitBreaker bool
}

// RetryConfig holds retry-related configuration for automatic request retries.
// The SDK uses exponential backoff with jitter by default.
//
// Example:
//
//	config.RetryConfig = sdk.RetryConfig{
//	    MaxRetries:      5,
//	    InitialInterval: 50 * time.Millisecond,
//	    MaxInterval:     10 * time.Second,
//	    Multiplier:      1.5,
//	}
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	// Set to 0 to disable retries.
	// Default: 3
	MaxRetries int

	// InitialInterval is the initial retry interval.
	// The first retry will wait approximately this long.
	// Default: 100ms
	InitialInterval time.Duration

	// MaxInterval is the maximum retry interval.
	// Retry delays will not exceed this value.
	// Default: 5s
	MaxInterval time.Duration

	// Multiplier is the exponential backoff multiplier.
	// Each retry interval is multiplied by this value.
	// Default: 2.0
	Multiplier float64
}

// TransportConfig holds HTTP transport configuration for connection pooling.
// These settings control how the SDK manages HTTP connections.
//
// Example:
//
//	config.TransportConfig = sdk.TransportConfig{
//	    MaxIdleConns:    200,
//	    MaxConnsPerHost: 50,
//	    IdleConnTimeout: 120 * time.Second,
//	}
type TransportConfig struct {
	// MaxIdleConns controls the maximum number of idle connections
	// across all hosts. Zero means no limit.
	// Default: 100
	MaxIdleConns int

	// MaxConnsPerHost controls the maximum connections per host.
	// This includes connections in the dialing, active, and idle states.
	// Default: 10
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum time an idle connection will remain idle
	// before closing itself. Zero means no limit.
	// Default: 90s
	IdleConnTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults suitable for most use cases.
// The default configuration includes:
//   - Base URL: http://localhost:8080
//   - Timeout: 30 seconds
//   - Retries: 3 attempts with exponential backoff
//   - Connection pooling: 100 idle connections, 10 per host
//
// Example:
//
//	config := sdk.DefaultConfig()
//	client, err := sdk.NewClient(config)
func DefaultConfig() *Config {
	return &Config{
		BaseURL: "http://localhost:8080",
		Timeout: 30 * time.Second,
		RetryConfig: RetryConfig{
			MaxRetries:      3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     5 * time.Second,
			Multiplier:      2.0,
		},
		TransportConfig: TransportConfig{
			MaxIdleConns:    100,
			MaxConnsPerHost: 10,
			IdleConnTimeout: 90 * time.Second,
		},
		Headers:  make(map[string]string),
		Observer: &NoopObserver{},
	}
}

// WithBaseURL sets the base URL for the Birb Nest API.
// The URL should include the protocol (http/https) but not trailing slashes.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithBaseURL("https://cache.example.com")
func (c *Config) WithBaseURL(url string) *Config {
	c.BaseURL = url
	return c
}

// WithTimeout sets the request timeout for all operations.
// This includes connection time, redirects, and reading the response.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithTimeout(10 * time.Second)
func (c *Config) WithTimeout(timeout time.Duration) *Config {
	c.Timeout = timeout
	return c
}

// WithRetries sets the maximum number of retry attempts for failed requests.
// Set to 0 to disable automatic retries.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithRetries(5) // Retry up to 5 times
func (c *Config) WithRetries(maxRetries int) *Config {
	c.RetryConfig.MaxRetries = maxRetries
	return c
}

// WithHeader adds a custom header to be sent with all requests.
// Useful for authentication, request tracking, or custom metadata.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithHeader("X-API-Key", "your-api-key").
//	    WithHeader("X-Tenant-ID", "tenant-123")
func (c *Config) WithHeader(key, value string) *Config {
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	c.Headers[key] = value
	return c
}

// WithCircuitBreaker enables and configures circuit breaker protection.
// Circuit breaker prevents cascading failures by failing fast when
// the service is experiencing issues.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithCircuitBreaker(sdk.CircuitBreakerConfig{
//	        FailureThreshold: 5,
//	        SuccessThreshold: 2,
//	        Timeout: 30 * time.Second,
//	    })
func (c *Config) WithCircuitBreaker(config CircuitBreakerConfig) *Config {
	c.CircuitBreakerConfig = &config
	return c
}

// WithRetryStrategy sets a custom retry strategy for determining retry delays.
// By default, exponential backoff with jitter is used.
//
// Example:
//
//	// Use fixed interval retries
//	config := sdk.DefaultConfig().
//	    WithRetryStrategy(sdk.NewFixedIntervalStrategy(1 * time.Second))
//
//	// Use custom strategy
//	config.WithRetryStrategy(sdk.RetryStrategyFunc(
//	    func(attempt int) time.Duration {
//	        return time.Duration(attempt*attempt) * time.Second
//	    }
//	))
func (c *Config) WithRetryStrategy(strategy RetryStrategy) *Config {
	c.RetryStrategy = strategy
	return c
}

// WithObserver sets a custom observer for monitoring SDK operations.
// Observers can track requests, responses, errors, and performance metrics.
//
// Example:
//
//	type LogObserver struct{}
//
//	func (o *LogObserver) OnRequest(ctx context.Context, method, url string) {
//	    log.Printf("[%s] %s", method, url)
//	}
//
//	config := sdk.DefaultConfig().
//	    WithObserver(&LogObserver{})
func (c *Config) WithObserver(observer Observer) *Config {
	c.Observer = observer
	return c
}

// WithHedgedRequests enables hedged requests to reduce tail latency.
// When enabled, the SDK sends parallel requests after a delay and uses
// the first successful response.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithHedgedRequests(sdk.HedgedRequest{
//	        Delay:       50 * time.Millisecond,
//	        MaxAttempts: 2,
//	    })
func (c *Config) WithHedgedRequests(config HedgedRequest) *Config {
	c.HedgedRequestConfig = &config
	return c
}

// WithPerEndpointCircuitBreaker enables per-endpoint circuit breakers.
// When enabled, each API endpoint maintains its own circuit breaker state.
// This prevents issues with one endpoint from affecting others.
//
// Example:
//
//	config := sdk.DefaultConfig().
//	    WithCircuitBreaker(sdk.DefaultCircuitBreakerConfig()).
//	    WithPerEndpointCircuitBreaker()
func (c *Config) WithPerEndpointCircuitBreaker() *Config {
	c.EnablePerEndpointCircuitBreaker = true
	return c
}

// Validate validates the configuration and sets defaults for missing values.
// This is called automatically by NewClient and NewExtendedClient.
//
// Returns an error if the configuration is invalid (e.g., missing base URL).
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return ErrInvalidConfig
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if c.RetryConfig.MaxRetries < 0 {
		c.RetryConfig.MaxRetries = 0
	}
	if c.RetryConfig.InitialInterval <= 0 {
		c.RetryConfig.InitialInterval = 100 * time.Millisecond
	}
	if c.RetryConfig.MaxInterval <= 0 {
		c.RetryConfig.MaxInterval = 5 * time.Second
	}
	if c.RetryConfig.Multiplier <= 1 {
		c.RetryConfig.Multiplier = 2.0
	}
	if c.Observer == nil {
		c.Observer = &NoopObserver{}
	}
	// Validate circuit breaker config if present
	if c.CircuitBreakerConfig != nil {
		if c.CircuitBreakerConfig.FailureThreshold <= 0 {
			c.CircuitBreakerConfig.FailureThreshold = 5
		}
		if c.CircuitBreakerConfig.SuccessThreshold <= 0 {
			c.CircuitBreakerConfig.SuccessThreshold = 2
		}
		if c.CircuitBreakerConfig.Timeout <= 0 {
			c.CircuitBreakerConfig.Timeout = 30 * time.Second
		}
		if c.CircuitBreakerConfig.HalfOpenRequests <= 0 {
			c.CircuitBreakerConfig.HalfOpenRequests = 3
		}
	}
	return nil
}
