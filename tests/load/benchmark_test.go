package load

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"strconv"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/birbparty/birb-nest/tests/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Global test containers and clients
var (
	containers  *testutil.TestContainers
	pgPool      *pgxpool.Pool
	cacheRepo   *database.CacheRepository
	cacheClient cache.Cache
	queueClient *queue.Client
)

// TestMain sets up containers for all benchmarks
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start all containers
	var err error
	containers, err = testutil.StartContainers(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to start containers: %v", err))
	}

	// Initialize PostgreSQL
	pgPool, err = pgxpool.New(ctx, containers.PostgresURL)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Postgres: %v", err))
	}

	// Run migrations
	if err := runMigrations(ctx, pgPool); err != nil {
		panic(fmt.Sprintf("Failed to run migrations: %v", err))
	}

	// Initialize database
	dbConfig := &database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		MaxConns: 10,
		MinConns: 2,
	}
	db, err := database.NewDB(dbConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to create DB: %v", err))
	}
	cacheRepo = database.NewCacheRepository(db)

	// Parse Redis URL to get host and port
	redisHost := "localhost"
	redisPort := 6379
	if containers.RedisURL != "" {
		// Extract host and port from redis://host:port format
		parts := strings.TrimPrefix(containers.RedisURL, "redis://")
		hostPort := strings.Split(parts, ":")
		if len(hostPort) == 2 {
			redisHost = hostPort[0]
			if port, err := strconv.Atoi(hostPort[1]); err == nil {
				redisPort = port
			}
		}
	}

	// Initialize Redis cache
	cacheConfig := &cache.Config{
		Host:         redisHost,
		Port:         redisPort,
		Password:     "",
		DB:           0,
		DefaultTTL:   3600 * time.Second,
		MaxRetries:   3,
		PoolSize:     10,
		MinIdleConns: 5,
	}
	cacheClient, err = cache.NewRedisCache(cacheConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Redis cache: %v", err))
	}

	// Initialize queue client
	queueConfig := &queue.Config{
		URL:                containers.NATSURL,
		Name:               "birb-nest-bench",
		StreamName:         "CACHE_EVENTS",
		DLQStreamName:      "CACHE_DLQ",
		ConsumerMaxDeliver: 3,
		BatchSize:          100,
		BatchTimeout:       100 * time.Millisecond,
	}
	queueClient, err = queue.NewClient(queueConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to create queue client: %v", err))
	}

	// Run benchmarks
	code := m.Run()

	// Cleanup
	pgPool.Close()
	cacheClient.Close()
	queueClient.Close()
	containers.Cleanup(ctx)

	os.Exit(code)
}

// runMigrations runs the database migrations
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	query := `
		CREATE TABLE IF NOT EXISTS cache_entries (
			key VARCHAR(255) PRIMARY KEY,
			value JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			version INTEGER DEFAULT 1,
			ttl INTEGER,
			metadata JSONB DEFAULT '{}'::jsonb
		);

		CREATE INDEX IF NOT EXISTS idx_cache_entries_ttl ON cache_entries(updated_at, ttl) WHERE ttl IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_cache_entries_updated_at ON cache_entries(updated_at);
	`
	_, err := pool.Exec(ctx, query)
	return err
}

// Benchmark data generators
func generateTestData(size int) map[string]interface{} {
	data := make(map[string]interface{})
	data["id"] = fmt.Sprintf("bench-%d-%d", time.Now().Unix(), rand.Intn(10000))
	data["timestamp"] = time.Now().Format(time.RFC3339)
	data["type"] = "benchmark"

	// Add fields to reach approximate size
	fieldCount := size / 100 // Rough estimate
	for i := 0; i < fieldCount; i++ {
		data[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d_%s", i, randString(50))
	}

	return data
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Redis Cache Benchmarks
func BenchmarkRedisSet(b *testing.B) {
	ctx := context.Background()

	benchmarks := []struct {
		name string
		size int
	}{
		{"Small_100B", 100},
		{"Medium_1KB", 1024},
		{"Large_10KB", 10240},
		{"XLarge_100KB", 102400},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			data := generateTestData(bm.size)
			value, _ := json.Marshal(data)

			b.ResetTimer()
			b.SetBytes(int64(len(value)))

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("bench-key-%d", i)
				err := cacheClient.Set(ctx, key, value, 3600*time.Second)
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkRedisGet(b *testing.B) {
	ctx := context.Background()

	// Pre-populate cache
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("bench-get-%d", i)
		data := generateTestData(1024)
		value, _ := json.Marshal(data)
		cacheClient.Set(ctx, key, value, 3600*time.Second)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-get-%d", i%numKeys)
		value, err := cacheClient.Get(ctx, key)
		if err != nil && err != cache.ErrKeyNotFound {
			b.Fatal(err)
		}
		if value != nil {
			b.SetBytes(int64(len(value)))
		}
	}
}

// PostgreSQL Benchmarks
func BenchmarkPostgresInsert(b *testing.B) {
	ctx := context.Background()

	benchmarks := []struct {
		name string
		size int
	}{
		{"Small_100B", 100},
		{"Medium_1KB", 1024},
		{"Large_10KB", 10240},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			data := generateTestData(bm.size)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("bench-pg-%d-%d", time.Now().UnixNano(), i)
				value, _ := json.Marshal(data)
				ttl := 3600 // 1 hour in seconds
				err := cacheRepo.Set(ctx, key, value, &ttl, nil)
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkPostgresBatchInsert(b *testing.B) {
	ctx := context.Background()

	batchSizes := []int{10, 50, 100, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Use individual inserts for now since BatchSet doesn't exist
				for j := 0; j < batchSize; j++ {
					key := fmt.Sprintf("batch-%d-%d-%d", time.Now().UnixNano(), i, j)
					value, _ := json.Marshal(generateTestData(1024))
					ttl := 3600
					err := cacheRepo.Set(ctx, key, value, &ttl, nil)
					require.NoError(b, err)
				}
			}
		})
	}
}

// NATS Queue Benchmarks
func BenchmarkNATSPublish(b *testing.B) {
	ctx := context.Background()

	sizes := []int{100, 1024, 10240}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dB", size), func(b *testing.B) {
			data := generateTestData(size)
			value, _ := json.Marshal(data)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				msg := queue.NewPersistenceMessage(
					fmt.Sprintf("bench-nats-%d-%d", time.Now().UnixNano(), i),
					value,
					1,
					nil,
					nil,
				)
				err := queueClient.PublishPersistence(ctx, msg)
				require.NoError(b, err)
			}
		})
	}
}

// Concurrent Access Benchmarks
func BenchmarkConcurrentCacheAccess(b *testing.B) {
	ctx := context.Background()

	// Pre-populate cache
	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("concurrent-%d", i)
		data := generateTestData(1024)
		value, _ := json.Marshal(data)
		cacheClient.Set(ctx, key, value, 3600*time.Second)
	}

	concurrencyLevels := []int{10, 50, 100, 500}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					key := fmt.Sprintf("concurrent-%d", rand.Intn(numKeys))

					// 70% reads, 30% writes
					if rand.Float32() < 0.7 {
						cacheClient.Get(ctx, key)
					} else {
						data := generateTestData(1024)
						value, _ := json.Marshal(data)
						cacheClient.Set(ctx, key, value, 3600*time.Second)
					}
				}
			})
		})
	}
}

// End-to-End Benchmarks
func BenchmarkEndToEndCacheOperation(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("e2e-bench-%d-%d", time.Now().UnixNano(), i)
		data := generateTestData(1024)
		value, _ := json.Marshal(data)

		// Write to Redis
		err := cacheClient.Set(ctx, key, value, 3600*time.Second)
		require.NoError(b, err)

		// Publish to queue
		msg := queue.NewPersistenceMessage(key, value, 1, nil, nil)
		err = queueClient.PublishPersistence(ctx, msg)
		require.NoError(b, err)

		// Read from Redis
		readValue, err := cacheClient.Get(ctx, key)
		require.NoError(b, err)
		require.NotNil(b, readValue)
	}
}

// Memory allocation benchmarks
func BenchmarkMemoryAllocation(b *testing.B) {
	sizes := []int{100, 1024, 10240, 102400}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dB", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				data := generateTestData(size)
				value, _ := json.Marshal(data)

				// Force usage to prevent optimization
				if len(value) == 0 {
					b.Fatal("Empty value")
				}
			}
		})
	}
}

// Benchmark helper to measure operation latencies
func BenchmarkLatencyDistribution(b *testing.B) {
	ctx := context.Background()

	// Pre-populate cache
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("latency-%d", i)
		data := generateTestData(1024)
		value, _ := json.Marshal(data)
		cacheClient.Set(ctx, key, value, 3600*time.Second)
	}

	latencies := make([]time.Duration, 0, b.N)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("latency-%d", rand.Intn(numKeys))

		start := time.Now()
		_, err := cacheClient.Get(ctx, key)
		latency := time.Since(start)

		if err != nil && err != cache.ErrKeyNotFound {
			b.Fatal(err)
		}

		latencies = append(latencies, latency)
	}

	// Calculate percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]

	b.Logf("Latency - P50: %v, P95: %v, P99: %v", p50, p95, p99)
}
