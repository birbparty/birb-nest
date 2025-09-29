package queue

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds queue configuration
type Config struct {
	// NATS connection settings
	URL      string
	Name     string
	User     string
	Password string

	// JetStream settings
	StreamName            string
	StreamMaxAge          time.Duration
	StreamMaxBytes        int64
	StreamMaxMsgs         int64
	StreamMaxMsgSize      int32
	StreamReplicas        int
	StreamRetentionPolicy string

	// Consumer settings
	ConsumerName          string
	ConsumerMaxDeliver    int
	ConsumerAckWait       time.Duration
	ConsumerMaxAckPending int

	// DLQ settings
	DLQStreamName    string
	DLQMaxRetries    int
	DLQRetryInterval time.Duration

	// Processing settings
	BatchSize         int
	BatchTimeout      time.Duration
	WorkerConcurrency int
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() (*Config, error) {
	streamMaxBytes, err := strconv.ParseInt(getEnvOrDefault("NATS_STREAM_MAX_BYTES", "1073741824"), 10, 64) // 1GB default
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_STREAM_MAX_BYTES: %w", err)
	}

	streamMaxMsgs, err := strconv.ParseInt(getEnvOrDefault("NATS_STREAM_MAX_MSGS", "1000000"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_STREAM_MAX_MSGS: %w", err)
	}

	streamMaxMsgSize, err := strconv.ParseInt(getEnvOrDefault("NATS_STREAM_MAX_MSG_SIZE", "1048576"), 10, 32) // 1MB default
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_STREAM_MAX_MSG_SIZE: %w", err)
	}

	streamReplicas, err := strconv.Atoi(getEnvOrDefault("NATS_STREAM_REPLICAS", "1"))
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_STREAM_REPLICAS: %w", err)
	}

	consumerMaxDeliver, err := strconv.Atoi(getEnvOrDefault("NATS_CONSUMER_MAX_DELIVER", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_CONSUMER_MAX_DELIVER: %w", err)
	}

	consumerMaxAckPending, err := strconv.Atoi(getEnvOrDefault("NATS_CONSUMER_MAX_ACK_PENDING", "1000"))
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_CONSUMER_MAX_ACK_PENDING: %w", err)
	}

	dlqMaxRetries, err := strconv.Atoi(getEnvOrDefault("DLQ_MAX_RETRIES", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid DLQ_MAX_RETRIES: %w", err)
	}

	batchSize, err := strconv.Atoi(getEnvOrDefault("WORKER_BATCH_SIZE", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_BATCH_SIZE: %w", err)
	}

	workerConcurrency, err := strconv.Atoi(getEnvOrDefault("WORKER_CONCURRENCY", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_CONCURRENCY: %w", err)
	}

	batchTimeout, err := parseDuration(getEnvOrDefault("WORKER_BATCH_TIMEOUT", "1s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_BATCH_TIMEOUT: %w", err)
	}

	return &Config{
		URL:                   getEnvOrDefault("NATS_URL", "nats://localhost:4222"),
		Name:                  getEnvOrDefault("NATS_NAME", "birb-nest"),
		User:                  os.Getenv("NATS_USER"),
		Password:              os.Getenv("NATS_PASSWORD"),
		StreamName:            getEnvOrDefault("NATS_STREAM_NAME", "BIRB_CACHE"),
		StreamMaxAge:          24 * time.Hour,
		StreamMaxBytes:        streamMaxBytes,
		StreamMaxMsgs:         streamMaxMsgs,
		StreamMaxMsgSize:      int32(streamMaxMsgSize),
		StreamReplicas:        streamReplicas,
		StreamRetentionPolicy: "limits",
		ConsumerName:          getEnvOrDefault("NATS_CONSUMER_NAME", "birb-worker"),
		ConsumerMaxDeliver:    consumerMaxDeliver,
		ConsumerAckWait:       30 * time.Second,
		ConsumerMaxAckPending: consumerMaxAckPending,
		DLQStreamName:         getEnvOrDefault("DLQ_STREAM_NAME", "BIRB_CACHE_DLQ"),
		DLQMaxRetries:         dlqMaxRetries,
		DLQRetryInterval:      5 * time.Minute,
		BatchSize:             batchSize,
		BatchTimeout:          batchTimeout,
		WorkerConcurrency:     workerConcurrency,
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
