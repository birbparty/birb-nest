package database

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds database configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() (*Config, error) {
	port, err := strconv.Atoi(getEnvOrDefault("POSTGRES_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT: %w", err)
	}

	maxConns, err := strconv.ParseInt(getEnvOrDefault("POSTGRES_MAX_CONNS", "25"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_MAX_CONNS: %w", err)
	}

	minConns, err := strconv.ParseInt(getEnvOrDefault("POSTGRES_MIN_CONNS", "5"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_MIN_CONNS: %w", err)
	}

	return &Config{
		Host:            getEnvOrDefault("POSTGRES_HOST", "localhost"),
		Port:            port,
		User:            getEnvOrDefault("POSTGRES_USER", "birb"),
		Password:        getEnvOrDefault("POSTGRES_PASSWORD", "birbpass"),
		Database:        getEnvOrDefault("POSTGRES_DB", "birbcache"),
		MaxConns:        int32(maxConns),
		MinConns:        int32(minConns),
		MaxConnLifetime: 1 * time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}, nil
}

// ConnectionString returns a PostgreSQL connection string
func (c *Config) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
