package cleanup

import (
	"os"
	"strconv"
	"time"

	"github.com/birbparty/birb-nest/internal/storage"
)

// LoadCleanupConfig loads cleanup configuration from environment variables
func LoadCleanupConfig() CleanupConfig {
	return CleanupConfig{
		InactivityTimeout:   getEnvDuration("CLEANUP_INACTIVITY_TIMEOUT", 30*time.Minute),
		MinimumAge:          getEnvDuration("CLEANUP_MINIMUM_AGE", 30*time.Minute),
		CleanupInterval:     getEnvDuration("CLEANUP_INTERVAL", 5*time.Minute),
		DryRun:              getEnvBool("CLEANUP_DRY_RUN", false),
		ArchiveBeforeDelete: getEnvBool("CLEANUP_ARCHIVE", true),
	}
}

// LoadSpacesConfig loads Digital Ocean Spaces configuration from environment variables
func LoadSpacesConfig() storage.SpacesConfig {
	return storage.SpacesConfig{
		Endpoint:  getEnvString("DO_SPACES_ENDPOINT", "nyc3.digitaloceanspaces.com"),
		Region:    getEnvString("DO_SPACES_REGION", "nyc3"),
		Bucket:    getEnvString("DO_SPACES_BUCKET", "birbnest-archives"),
		AccessKey: getEnvString("DO_SPACES_ACCESS_KEY", ""),
		SecretKey: getEnvString("DO_SPACES_SECRET_KEY", ""),
	}
}

// getEnvString gets a string value from environment or returns default
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets a boolean value from environment or returns default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvDuration gets a duration value from environment or returns default
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}
