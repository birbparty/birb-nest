package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/birbparty/birb-nest/tests/testutil"
	_ "github.com/lib/pq"
)

var (
	testContainers *testutil.TestContainers
	testDB         *database.DB
	testCache      cache.Cache
	testQueue      *queue.Client
)

// TestMain sets up and tears down test infrastructure
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start containers
	tc, err := testutil.StartContainers(ctx)
	if err != nil {
		fmt.Printf("Failed to start containers: %v\n", err)
		os.Exit(1)
	}
	testContainers = tc

	// Wait for containers to be healthy
	if err := tc.WaitForHealthy(ctx); err != nil {
		fmt.Printf("Failed to wait for healthy containers: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}

	// Initialize database
	if err := initDatabase(ctx, tc.PostgresURL); err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}

	// Parse database URL and create config
	dbConfig, err := parsePostgresURL(tc.PostgresURL)
	if err != nil {
		fmt.Printf("Failed to parse postgres URL: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}
	dbConfig.MaxConns = 10
	dbConfig.MinConns = 2
	testDB, err = database.NewDB(dbConfig)
	if err != nil {
		fmt.Printf("Failed to create database connection: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}

	// Parse Redis URL and create config
	redisConfig, err := parseRedisURL(tc.RedisURL)
	if err != nil {
		fmt.Printf("Failed to parse redis URL: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}
	redisConfig.MaxRetries = 3
	redisConfig.PoolSize = 10
	redisConfig.DefaultTTL = time.Minute * 5
	testCache, err = cache.NewRedisCache(redisConfig)
	if err != nil {
		fmt.Printf("Failed to create cache connection: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}

	// Create queue connection
	testQueue, err = queue.NewClient(&queue.Config{
		URL:                   tc.NATSURL,
		Name:                  "test-client",
		StreamName:            "TEST_CACHE",
		DLQStreamName:         "TEST_CACHE_DLQ",
		ConsumerAckWait:       30 * time.Second,
		ConsumerMaxDeliver:    5,
		ConsumerMaxAckPending: 100,
		BatchSize:             10,
		BatchTimeout:          2 * time.Second,
	})
	if err != nil {
		fmt.Printf("Failed to create queue connection: %v\n", err)
		tc.Cleanup(ctx)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	testDB.Close()
	testQueue.Close()
	tc.Cleanup(ctx)

	os.Exit(code)
}

// initDatabase creates the cache_entries table
func initDatabase(ctx context.Context, connStr string) error {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	query := `
		CREATE TABLE IF NOT EXISTS cache_entries (
			instance_id VARCHAR(255) NOT NULL DEFAULT 'global',
			key VARCHAR(255) NOT NULL,
			value JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			version INTEGER DEFAULT 1,
			ttl INTEGER,
			metadata JSONB,
			PRIMARY KEY (instance_id, key)
		);

		CREATE INDEX IF NOT EXISTS idx_cache_entries_instance_updated ON cache_entries(instance_id, updated_at);
		CREATE INDEX IF NOT EXISTS idx_cache_entries_instance_created ON cache_entries(instance_id, created_at);
	`

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// resetDatabase clears all data from the database
func resetDatabase(t *testing.T) {
	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "TRUNCATE TABLE cache_entries"); err != nil {
		t.Fatalf("Failed to reset database: %v", err)
	}
}

// resetCache clears all data from the cache
func resetCache(t *testing.T) {
	ctx := context.Background()
	// We'll use a test-specific key prefix, so we can safely flush all test keys
	// without affecting other data
	if err := testCache.Delete(ctx, "test:*"); err != nil {
		t.Logf("Warning: Failed to reset cache: %v", err)
	}
}

// resetQueue purges all messages from the queue
func resetQueue(t *testing.T) {
	// Purge the stream to remove all messages
	streamInfo, err := testQueue.StreamInfo("TEST_CACHE")
	if err != nil {
		t.Logf("Warning: Failed to get stream info: %v", err)
		return
	}
	// If stream exists, delete and recreate it
	if streamInfo != nil {
		// Note: The Client type doesn't expose stream management directly,
		// so we'll just log a warning for now
		t.Logf("Warning: Queue reset not fully implemented - stream contains %d messages", streamInfo.State.Msgs)
	}
}

// resetAll resets all test data
func resetAll(t *testing.T) {
	resetDatabase(t)
	resetCache(t)
	resetQueue(t)
}

// parsePostgresURL parses a PostgreSQL connection URL into a Config
func parsePostgresURL(connStr string) (*database.Config, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("invalid scheme: %s", u.Scheme)
	}

	port := 5432
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	password, _ := u.User.Password()

	return &database.Config{
		Host:     u.Hostname(),
		Port:     port,
		User:     u.User.Username(),
		Password: password,
		Database: strings.TrimPrefix(u.Path, "/"),
		MaxConns: 10,
		MinConns: 2,
	}, nil
}

// parseRedisURL parses a Redis connection URL into a Config
func parseRedisURL(connStr string) (*cache.Config, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.Scheme != "redis" {
		return nil, fmt.Errorf("invalid scheme: %s", u.Scheme)
	}

	port := 6379
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	password, _ := u.User.Password()

	// Parse DB from path or query params
	db := 0
	if u.Path != "" && u.Path != "/" {
		dbStr := strings.TrimPrefix(u.Path, "/")
		db, _ = strconv.Atoi(dbStr)
	}

	return &cache.Config{
		Host:     u.Hostname(),
		Port:     port,
		Password: password,
		DB:       db,
	}, nil
}
