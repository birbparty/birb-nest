package cache

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds cache configuration
type Config struct {
	// Redis connection settings
	Host     string
	Port     int
	Password string
	DB       int

	// Connection pool settings
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int
	MinIdleConns    int
	MaxIdleTime     time.Duration

	// Default TTL for cache entries
	DefaultTTL time.Duration
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() (*Config, error) {
	port, err := strconv.Atoi(getEnvOrDefault("REDIS_PORT", "6379"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_PORT: %w", err)
	}

	db, err := strconv.Atoi(getEnvOrDefault("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	poolSize, err := strconv.Atoi(getEnvOrDefault("REDIS_POOL_SIZE", "50"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_POOL_SIZE: %w", err)
	}

	minIdleConns, err := strconv.Atoi(getEnvOrDefault("REDIS_MIN_IDLE_CONNS", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_MIN_IDLE_CONNS: %w", err)
	}

	defaultTTL, err := parseDuration(getEnvOrDefault("CACHE_DEFAULT_TTL", "1h"))
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_DEFAULT_TTL: %w", err)
	}

	return &Config{
		Host:            getEnvOrDefault("REDIS_HOST", "localhost"),
		Port:            port,
		Password:        os.Getenv("REDIS_PASSWORD"),
		DB:              db,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        poolSize,
		MinIdleConns:    minIdleConns,
		MaxIdleTime:     5 * time.Minute,
		DefaultTTL:      defaultTTL,
	}, nil
}

// Address returns the Redis server address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) (time.Duration, error) {
	// Try parsing as a duration string (e.g., "1h30m")
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Try parsing as seconds
	if seconds, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	return 0, fmt.Errorf("invalid duration format: %s", s)
}
