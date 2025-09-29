package sdk

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test default values
	if config.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %v, want %v", config.BaseURL, "http://localhost:8080")
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 30*time.Second)
	}

	if config.RetryConfig.MaxRetries != 3 {
		t.Errorf("MaxRetries = %v, want %v", config.RetryConfig.MaxRetries, 3)
	}

	if config.RetryConfig.InitialInterval != 100*time.Millisecond {
		t.Errorf("InitialInterval = %v, want %v", config.RetryConfig.InitialInterval, 100*time.Millisecond)
	}

	if config.RetryConfig.MaxInterval != 5*time.Second {
		t.Errorf("MaxInterval = %v, want %v", config.RetryConfig.MaxInterval, 5*time.Second)
	}

	if config.RetryConfig.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want %v", config.RetryConfig.Multiplier, 2.0)
	}

	if config.TransportConfig.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %v, want %v", config.TransportConfig.MaxIdleConns, 100)
	}

	if config.TransportConfig.MaxConnsPerHost != 10 {
		t.Errorf("MaxConnsPerHost = %v, want %v", config.TransportConfig.MaxConnsPerHost, 10)
	}

	if config.TransportConfig.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want %v", config.TransportConfig.IdleConnTimeout, 90*time.Second)
	}

	if config.Headers == nil {
		t.Error("Headers should not be nil")
	}
}

func TestConfig_WithBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantURL string
	}{
		{
			name:    "http URL",
			url:     "http://api.example.com",
			wantURL: "http://api.example.com",
		},
		{
			name:    "https URL",
			url:     "https://api.example.com",
			wantURL: "https://api.example.com",
		},
		{
			name:    "URL with port",
			url:     "http://localhost:9090",
			wantURL: "http://localhost:9090",
		},
		{
			name:    "URL with path",
			url:     "http://api.example.com/v2",
			wantURL: "http://api.example.com/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().WithBaseURL(tt.url)
			if config.BaseURL != tt.wantURL {
				t.Errorf("BaseURL = %v, want %v", config.BaseURL, tt.wantURL)
			}
		})
	}
}

func TestConfig_WithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		wantTimeout time.Duration
	}{
		{
			name:        "10 seconds",
			timeout:     10 * time.Second,
			wantTimeout: 10 * time.Second,
		},
		{
			name:        "1 minute",
			timeout:     time.Minute,
			wantTimeout: time.Minute,
		},
		{
			name:        "500 milliseconds",
			timeout:     500 * time.Millisecond,
			wantTimeout: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().WithTimeout(tt.timeout)
			if config.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %v, want %v", config.Timeout, tt.wantTimeout)
			}
		})
	}
}

func TestConfig_WithRetries(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		want       int
	}{
		{
			name:       "zero retries",
			maxRetries: 0,
			want:       0,
		},
		{
			name:       "5 retries",
			maxRetries: 5,
			want:       5,
		},
		{
			name:       "negative retries (should be allowed)",
			maxRetries: -1,
			want:       -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().WithRetries(tt.maxRetries)
			if config.RetryConfig.MaxRetries != tt.want {
				t.Errorf("MaxRetries = %v, want %v", config.RetryConfig.MaxRetries, tt.want)
			}
		})
	}
}

func TestConfig_WithHeader(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		setupFunc  func() *Config
		wantLen    int
		wantHeader string
	}{
		{
			name:       "add to empty headers",
			key:        "X-API-Key",
			value:      "test-key",
			setupFunc:  func() *Config { return &Config{} },
			wantLen:    1,
			wantHeader: "test-key",
		},
		{
			name:       "add to existing headers",
			key:        "X-Request-ID",
			value:      "12345",
			setupFunc:  func() *Config { return DefaultConfig().WithHeader("X-API-Key", "test-key") },
			wantLen:    2,
			wantHeader: "12345",
		},
		{
			name:       "overwrite existing header",
			key:        "X-API-Key",
			value:      "new-key",
			setupFunc:  func() *Config { return DefaultConfig().WithHeader("X-API-Key", "old-key") },
			wantLen:    1,
			wantHeader: "new-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupFunc().WithHeader(tt.key, tt.value)

			if len(config.Headers) != tt.wantLen {
				t.Errorf("Headers length = %v, want %v", len(config.Headers), tt.wantLen)
			}

			if got := config.Headers[tt.key]; got != tt.wantHeader {
				t.Errorf("Header[%s] = %v, want %v", tt.key, got, tt.wantHeader)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		checkFunc func(t *testing.T, c *Config)
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty base URL",
			config: &Config{
				BaseURL: "",
			},
			wantErr: true,
		},
		{
			name: "zero timeout (should be set to default)",
			config: &Config{
				BaseURL: "http://localhost:8080",
				Timeout: 0,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.Timeout != 30*time.Second {
					t.Errorf("Timeout = %v, want %v", c.Timeout, 30*time.Second)
				}
			},
		},
		{
			name: "negative timeout (should be set to default)",
			config: &Config{
				BaseURL: "http://localhost:8080",
				Timeout: -1 * time.Second,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.Timeout != 30*time.Second {
					t.Errorf("Timeout = %v, want %v", c.Timeout, 30*time.Second)
				}
			},
		},
		{
			name: "negative max retries (should be set to 0)",
			config: &Config{
				BaseURL: "http://localhost:8080",
				RetryConfig: RetryConfig{
					MaxRetries: -5,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.RetryConfig.MaxRetries != 0 {
					t.Errorf("MaxRetries = %v, want %v", c.RetryConfig.MaxRetries, 0)
				}
			},
		},
		{
			name: "zero initial interval (should be set to default)",
			config: &Config{
				BaseURL: "http://localhost:8080",
				RetryConfig: RetryConfig{
					InitialInterval: 0,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.RetryConfig.InitialInterval != 100*time.Millisecond {
					t.Errorf("InitialInterval = %v, want %v", c.RetryConfig.InitialInterval, 100*time.Millisecond)
				}
			},
		},
		{
			name: "zero max interval (should be set to default)",
			config: &Config{
				BaseURL: "http://localhost:8080",
				RetryConfig: RetryConfig{
					MaxInterval: 0,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.RetryConfig.MaxInterval != 5*time.Second {
					t.Errorf("MaxInterval = %v, want %v", c.RetryConfig.MaxInterval, 5*time.Second)
				}
			},
		},
		{
			name: "multiplier <= 1 (should be set to default)",
			config: &Config{
				BaseURL: "http://localhost:8080",
				RetryConfig: RetryConfig{
					Multiplier: 0.5,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.RetryConfig.Multiplier != 2.0 {
					t.Errorf("Multiplier = %v, want %v", c.RetryConfig.Multiplier, 2.0)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkFunc != nil && err == nil {
				tt.checkFunc(t, tt.config)
			}
		})
	}
}

func TestConfig_Chaining(t *testing.T) {
	config := DefaultConfig().
		WithBaseURL("https://api.example.com").
		WithTimeout(10*time.Second).
		WithRetries(5).
		WithHeader("X-API-Key", "secret").
		WithHeader("X-Request-ID", "12345")

	// Verify all values were set
	if config.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %v, want %v", config.BaseURL, "https://api.example.com")
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 10*time.Second)
	}

	if config.RetryConfig.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want %v", config.RetryConfig.MaxRetries, 5)
	}

	if config.Headers["X-API-Key"] != "secret" {
		t.Errorf("Header[X-API-Key] = %v, want %v", config.Headers["X-API-Key"], "secret")
	}

	if config.Headers["X-Request-ID"] != "12345" {
		t.Errorf("Header[X-Request-ID] = %v, want %v", config.Headers["X-Request-ID"], "12345")
	}
}

func TestConfig_Copy(t *testing.T) {
	// Create original config
	original := DefaultConfig().
		WithBaseURL("https://api.example.com").
		WithTimeout(10*time.Second).
		WithHeader("X-API-Key", "secret")

	// Modify original after creating it
	original.RetryConfig.MaxRetries = 10
	original.Headers["X-New-Header"] = "new-value"

	// Create a new config to verify modifications work independently
	newConfig := DefaultConfig()

	// Verify they are independent
	if newConfig.BaseURL == original.BaseURL {
		t.Error("New config should have different BaseURL")
	}

	if newConfig.Timeout == original.Timeout {
		t.Error("New config should have different Timeout")
	}

	if len(newConfig.Headers) == len(original.Headers) {
		t.Error("New config should have different Headers")
	}
}

// Benchmark configuration operations
func BenchmarkDefaultConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig()
	}
}

func BenchmarkConfig_WithMethods(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig().
			WithBaseURL("https://api.example.com").
			WithTimeout(10*time.Second).
			WithRetries(5).
			WithHeader("X-API-Key", "secret").
			WithHeader("X-Request-ID", "12345")
	}
}

func BenchmarkConfig_Validate(b *testing.B) {
	config := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}
