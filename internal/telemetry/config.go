package telemetry

import (
	"os"
	"strconv"
)

// Config holds the configuration for telemetry
type Config struct {
	// Production mode - OTLP
	OTLPEndpoint   string
	ServiceName    string
	Environment    string
	ServiceVersion string

	// Testing mode - for local-otel file export
	ExportToFile    bool
	MetricsFilePath string
	TracesFilePath  string
	LogsFilePath    string

	// Common settings
	SamplingRate    float64
	LogLevel        string
	MetricsInterval int // seconds

	// Feature flags
	EnableTracing bool
	EnableMetrics bool
	EnableLogging bool
}

// NewConfigFromEnv creates a new config from environment variables
func NewConfigFromEnv() *Config {
	cfg := &Config{
		ServiceName:     getEnv("OTEL_SERVICE_NAME", "birb-nest"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		ServiceVersion:  getEnv("SERVICE_VERSION", "unknown"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		SamplingRate:    getEnvFloat("OTEL_SAMPLING_RATE", 1.0),
		MetricsInterval: getEnvInt("METRICS_INTERVAL", 10),
		EnableTracing:   getEnvBool("ENABLE_TRACING", true),
		EnableMetrics:   getEnvBool("ENABLE_METRICS", true),
		EnableLogging:   getEnvBool("ENABLE_LOGGING", true),
	}

	// Check if we're in test mode or using local-otel
	if getEnvBool("OTEL_EXPORT_TO_FILE", false) {
		cfg.ExportToFile = true
		cfg.MetricsFilePath = getEnv("OTEL_METRICS_FILE_PATH", "/tmp/otel/metrics.json")
		cfg.TracesFilePath = getEnv("OTEL_TRACES_FILE_PATH", "/tmp/otel/traces.json")
		cfg.LogsFilePath = getEnv("OTEL_LOGS_FILE_PATH", "/tmp/otel/logs.json")
	} else {
		cfg.OTLPEndpoint = getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
