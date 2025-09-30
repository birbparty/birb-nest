package api

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the API configuration
type Config struct {
	// Server configuration
	Host string
	Port int

	// Instance mode configuration
	Mode              string // "primary" or "replica"
	InstanceID        string // Unique instance identifier
	PrimaryURL        string // URL of primary API for replicas
	DefaultInstanceID string // Default instance ID for requests without instance context

	// Async writer configuration (primary only)
	WriteQueueSize int
	WriteWorkers   int

	// API configuration
	APIKey          string
	RequestTimeout  int
	ShutdownTimeout int

	// Redis configuration
	Redis RedisConfig

	// PostgreSQL configuration
	PostgreSQL PostgreSQLConfig

	// Telemetry configuration
	TelemetryEnabled bool
	MetricsPath      string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// PostgreSQLConfig holds PostgreSQL configuration
type PostgreSQLConfig struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	port, err := strconv.Atoi(getEnvOrDefault("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	writeQueueSize, err := strconv.Atoi(getEnvOrDefault("WRITE_QUEUE_SIZE", "10000"))
	if err != nil {
		return nil, fmt.Errorf("invalid WRITE_QUEUE_SIZE: %w", err)
	}

	writeWorkers, err := strconv.Atoi(getEnvOrDefault("WRITE_WORKERS", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WRITE_WORKERS: %w", err)
	}

	requestTimeout, err := strconv.Atoi(getEnvOrDefault("REQUEST_TIMEOUT", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
	}

	shutdownTimeout, err := strconv.Atoi(getEnvOrDefault("SHUTDOWN_TIMEOUT", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}

	// Redis config
	redisPort, err := strconv.Atoi(getEnvOrDefault("REDIS_PORT", "6379"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_PORT: %w", err)
	}

	redisDB, err := strconv.Atoi(getEnvOrDefault("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	// PostgreSQL config
	postgresEnabled := getEnvOrDefault("POSTGRES_ENABLED", "true") == "true"
	postgresPort, err := strconv.Atoi(getEnvOrDefault("POSTGRES_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT: %w", err)
	}

	telemetryEnabled := getEnvOrDefault("TELEMETRY_ENABLED", "true") == "true"

	return &Config{
		Host:              getEnvOrDefault("HOST", "0.0.0.0"),
		Port:              port,
		Mode:              getEnvOrDefault("MODE", "primary"),
		InstanceID:        getEnvOrDefault("INSTANCE_ID", "primary"),
		PrimaryURL:        os.Getenv("PRIMARY_URL"),
		DefaultInstanceID: getEnvOrDefault("DEFAULT_INSTANCE_ID", "global"),
		WriteQueueSize:    writeQueueSize,
		WriteWorkers:      writeWorkers,
		APIKey:            os.Getenv("API_KEY"),
		RequestTimeout:    requestTimeout,
		ShutdownTimeout:   shutdownTimeout,
		Redis: RedisConfig{
			Host:     getEnvOrDefault("REDIS_HOST", "localhost"),
			Port:     redisPort,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		PostgreSQL: PostgreSQLConfig{
			Enabled:  postgresEnabled,
			Host:     getEnvOrDefault("POSTGRES_HOST", "localhost"),
			Port:     postgresPort,
			User:     getEnvOrDefault("POSTGRES_USER", "postgres"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: getEnvOrDefault("POSTGRES_DATABASE", "birbnest"),
			SSLMode:  getEnvOrDefault("POSTGRES_SSL_MODE", "disable"),
		},
		TelemetryEnabled: telemetryEnabled,
		MetricsPath:      getEnvOrDefault("METRICS_PATH", "/metrics"),
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// IsPrimary returns true if this is the primary instance
func (c *Config) IsPrimary() bool {
	return c.Mode == "primary"
}

// IsReplica returns true if this is a replica instance
func (c *Config) IsReplica() bool {
	return c.Mode == "replica"
}
