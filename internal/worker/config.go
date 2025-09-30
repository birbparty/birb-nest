package worker

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds worker configuration
type Config struct {
	// Worker identification
	WorkerID   string
	WorkerName string

	// Instance configuration
	DefaultInstanceID string

	// Processing settings
	BatchSize                 int
	BatchTimeout              time.Duration
	MaxConcurrentBatches      int
	ProcessingConcurrency     int
	RehydrationBatchSize      int
	RehydrationInterval       time.Duration
	StartupRehydrationEnabled bool

	// Retry settings
	MaxRetries      int
	RetryBackoff    time.Duration
	RetryMultiplier float64
	MaxRetryBackoff time.Duration

	// Monitoring
	MetricsInterval time.Duration
	HealthCheckPort int
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() (*Config, error) {
	batchSize, err := strconv.Atoi(getEnvOrDefault("WORKER_BATCH_SIZE", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_BATCH_SIZE: %w", err)
	}

	batchTimeout, err := parseDuration(getEnvOrDefault("WORKER_BATCH_TIMEOUT", "1s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_BATCH_TIMEOUT: %w", err)
	}

	maxConcurrentBatches, err := strconv.Atoi(getEnvOrDefault("WORKER_MAX_CONCURRENT_BATCHES", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_MAX_CONCURRENT_BATCHES: %w", err)
	}

	processingConcurrency, err := strconv.Atoi(getEnvOrDefault("WORKER_PROCESSING_CONCURRENCY", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_PROCESSING_CONCURRENCY: %w", err)
	}

	rehydrationBatchSize, err := strconv.Atoi(getEnvOrDefault("WORKER_REHYDRATION_BATCH_SIZE", "500"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_REHYDRATION_BATCH_SIZE: %w", err)
	}

	rehydrationInterval, err := parseDuration(getEnvOrDefault("WORKER_REHYDRATION_INTERVAL", "5m"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_REHYDRATION_INTERVAL: %w", err)
	}

	startupRehydration, err := strconv.ParseBool(getEnvOrDefault("WORKER_STARTUP_REHYDRATION", "true"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_STARTUP_REHYDRATION: %w", err)
	}

	maxRetries, err := strconv.Atoi(getEnvOrDefault("WORKER_MAX_RETRIES", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_MAX_RETRIES: %w", err)
	}

	retryBackoff, err := parseDuration(getEnvOrDefault("WORKER_RETRY_BACKOFF", "1s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_RETRY_BACKOFF: %w", err)
	}

	retryMultiplier, err := strconv.ParseFloat(getEnvOrDefault("WORKER_RETRY_MULTIPLIER", "2.0"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_RETRY_MULTIPLIER: %w", err)
	}

	maxRetryBackoff, err := parseDuration(getEnvOrDefault("WORKER_MAX_RETRY_BACKOFF", "30s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_MAX_RETRY_BACKOFF: %w", err)
	}

	metricsInterval, err := parseDuration(getEnvOrDefault("WORKER_METRICS_INTERVAL", "30s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_METRICS_INTERVAL: %w", err)
	}

	healthCheckPort, err := strconv.Atoi(getEnvOrDefault("WORKER_HEALTH_PORT", "8081"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_HEALTH_PORT: %w", err)
	}

	// Generate worker ID if not provided
	workerID := getEnvOrDefault("WORKER_ID", generateWorkerID())

	return &Config{
		WorkerID:                  workerID,
		WorkerName:                getEnvOrDefault("WORKER_NAME", "birb-worker-"+workerID),
		DefaultInstanceID:         getEnvOrDefault("DEFAULT_INSTANCE_ID", "global"),
		BatchSize:                 batchSize,
		BatchTimeout:              batchTimeout,
		MaxConcurrentBatches:      maxConcurrentBatches,
		ProcessingConcurrency:     processingConcurrency,
		RehydrationBatchSize:      rehydrationBatchSize,
		RehydrationInterval:       rehydrationInterval,
		StartupRehydrationEnabled: startupRehydration,
		MaxRetries:                maxRetries,
		RetryBackoff:              retryBackoff,
		RetryMultiplier:           retryMultiplier,
		MaxRetryBackoff:           maxRetryBackoff,
		MetricsInterval:           metricsInterval,
		HealthCheckPort:           healthCheckPort,
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func generateWorkerID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
}
