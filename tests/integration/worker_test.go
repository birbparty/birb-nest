package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/birbparty/birb-nest/internal/telemetry"
	"github.com/birbparty/birb-nest/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorker_PersistenceProcessing tests processing of persistence messages
func TestWorker_PersistenceProcessing(t *testing.T) {
	resetAll(t)

	// Create test worker
	w, err := createTestWorker()
	require.NoError(t, err)

	// Start worker in background
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(100 * time.Millisecond)

	// Publish persistence messages
	for i := 0; i < 5; i++ {
		msg := queue.NewPersistenceMessage(
			fmt.Sprintf("test-key-%d", i),
			json.RawMessage(fmt.Sprintf(`{"value": %d}`, i)),
			1,
			nil,
			nil,
		)
		err := testQueue.PublishPersistence(context.Background(), msg)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Verify all messages were persisted to database
	repo := database.NewCacheRepository(testDB)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		entry, err := repo.Get(context.Background(), key)
		require.NoError(t, err)
		assert.Equal(t, key, entry.Key)
		assert.Contains(t, string(entry.Value), fmt.Sprintf(`"value": %d`, i))
	}

	// Stop worker
	w.Stop()
	wg.Wait()
}

// TestWorker_RehydrationProcessing tests processing of rehydration messages
func TestWorker_RehydrationProcessing(t *testing.T) {
	resetAll(t)

	// First, insert some data directly into database
	ctx := context.Background()
	repo := database.NewCacheRepository(testDB)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("rehydrate-key-%d", i)
		err := repo.Set(ctx, key, json.RawMessage(fmt.Sprintf(`{"data": %d}`, i)), nil, nil)
		require.NoError(t, err)
	}

	// Create and start worker
	w, err := createTestWorker()
	require.NoError(t, err)

	workerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.Start(workerCtx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(100 * time.Millisecond)

	// Publish rehydration messages
	for i := 0; i < 3; i++ {
		msg := queue.NewRehydrationMessage(
			fmt.Sprintf("rehydrate-key-%d", i),
			queue.PriorityHigh,
		)
		err := testQueue.PublishRehydration(context.Background(), msg)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Verify all entries were rehydrated to cache
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("rehydrate-key-%d", i)
		val, err := testCache.Get(context.Background(), key)
		require.NoError(t, err)
		assert.NotNil(t, val)
		// Verify the cached data contains the database entry
		assert.Contains(t, string(val), fmt.Sprintf(`"data": %d`, i))
	}

	cancel()
	wg.Wait()
}

// TestWorker_BatchProcessing tests batch processing behavior
func TestWorker_BatchProcessing(t *testing.T) {
	resetAll(t)

	// Create worker with small batch size for testing
	cfg := &worker.Config{
		WorkerID:                  "test-batch-worker",
		WorkerName:                "Test Batch Worker",
		BatchSize:                 5,
		BatchTimeout:              1 * time.Second,
		MaxConcurrentBatches:      2,
		ProcessingConcurrency:     5,
		RehydrationBatchSize:      10,
		RehydrationInterval:       5 * time.Minute,
		StartupRehydrationEnabled: false,
		MaxRetries:                3,
		RetryBackoff:              1 * time.Second,
		RetryMultiplier:           2.0,
		MaxRetryBackoff:           30 * time.Second,
		MetricsInterval:           30 * time.Second,
		HealthCheckPort:           8081,
	}

	metrics := worker.NewMetrics()
	w := worker.NewProcessor(cfg, testDB, testCache, testQueue, metrics)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(100 * time.Millisecond)

	// Publish more messages than batch size
	for i := 0; i < 12; i++ {
		msg := queue.NewPersistenceMessage(
			fmt.Sprintf("batch-key-%d", i),
			json.RawMessage(fmt.Sprintf(`{"batch": %d}`, i)),
			1,
			nil,
			nil,
		)
		err := testQueue.PublishPersistence(context.Background(), msg)
		require.NoError(t, err)
	}

	// Wait for batch processing
	time.Sleep(3 * time.Second)

	// Verify all messages were processed
	repo := database.NewCacheRepository(testDB)
	for i := 0; i < 12; i++ {
		key := fmt.Sprintf("batch-key-%d", i)
		entry, err := repo.Get(context.Background(), key)
		require.NoError(t, err)
		assert.Equal(t, key, entry.Key)
	}

	w.Stop()
	wg.Wait()
}

// TestWorker_InitialRehydration tests cache warming on startup
func TestWorker_InitialRehydration(t *testing.T) {
	resetAll(t)

	// Insert old data directly into database
	ctx := context.Background()

	// Create entries with different ages
	now := time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("old-key-%d", i)
		entry := &database.CacheEntry{
			Key:       key,
			Value:     json.RawMessage(fmt.Sprintf(`{"old": %d}`, i)),
			Version:   1,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
			UpdatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
		// Use direct SQL to insert with specific timestamps
		_, err := testDB.Exec(ctx,
			`INSERT INTO cache_entries (key, value, version, created_at, updated_at) 
			 VALUES ($1, $2, $3, $4, $5)`,
			entry.Key, entry.Value, entry.Version, entry.CreatedAt, entry.UpdatedAt,
		)
		require.NoError(t, err)
	}

	// Create worker with rehydration enabled
	w, err := createTestWorker()
	require.NoError(t, err)

	// Run startup rehydration
	err = w.PerformStartupRehydration(ctx)
	require.NoError(t, err)

	// Give it time to process
	time.Sleep(2 * time.Second)

	// Verify recent entries were rehydrated
	// With the default implementation, all entries might be rehydrated
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("old-key-%d", i)
		val, err := testCache.Get(ctx, key)
		// We expect at least the recent entries to be rehydrated
		if i < 3 { // Less than 3 hours old
			require.NoError(t, err)
			assert.NotNil(t, val)
		}
	}
}

// TestWorker_ErrorHandling tests error handling and DLQ behavior
func TestWorker_ErrorHandling(t *testing.T) {
	resetAll(t)

	// For this test, we'll publish a message with invalid data that will cause processing to fail
	// The actual message format needs to be a valid persistence message but with data that
	// will fail during processing

	// Create and start worker
	w, err := createTestWorker()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(100 * time.Millisecond)

	// Publish a message that will fail processing
	// For example, a message with a very long key that exceeds database limits
	longKey := fmt.Sprintf("key-%s", string(make([]byte, 300))) // Exceeds VARCHAR(255) limit
	msg := queue.NewPersistenceMessage(
		longKey,
		json.RawMessage(`{"test": "error"}`),
		1,
		nil,
		nil,
	)
	err = testQueue.PublishPersistence(context.Background(), msg)
	require.NoError(t, err)

	// Wait for processing attempts
	time.Sleep(2 * time.Second)

	// Check that the message was NOT persisted to database
	repo := database.NewCacheRepository(testDB)
	_, err = repo.Get(context.Background(), longKey)
	assert.Error(t, err)

	w.Stop()
	wg.Wait()
}

// TestWorker_GracefulShutdown tests graceful shutdown behavior
func TestWorker_GracefulShutdown(t *testing.T) {
	resetAll(t)

	w, err := createTestWorker()
	require.NoError(t, err)

	// Start worker
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	startedChan := make(chan struct{})
	go func() {
		defer wg.Done()
		close(startedChan)
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Wait for worker to start
	<-startedChan
	time.Sleep(100 * time.Millisecond)

	// Publish some messages
	for i := 0; i < 3; i++ {
		msg := queue.NewPersistenceMessage(
			fmt.Sprintf("shutdown-key-%d", i),
			json.RawMessage(fmt.Sprintf(`{"shutdown": %d}`, i)),
			1,
			nil,
			nil,
		)
		err := testQueue.PublishPersistence(context.Background(), msg)
		require.NoError(t, err)
	}

	// Give it a moment to start processing
	time.Sleep(500 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Wait for graceful shutdown
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-time.After(15 * time.Second):
		t.Fatal("Worker shutdown timeout")
	}

	// Verify messages were processed before shutdown
	repo := database.NewCacheRepository(testDB)
	processedCount := 0
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("shutdown-key-%d", i)
		_, err := repo.Get(context.Background(), key)
		if err == nil {
			processedCount++
		}
	}
	// At least some messages should have been processed
	assert.Greater(t, processedCount, 0)
}

// Helper function to create test worker
func createTestWorker() (*worker.Processor, error) {
	// Initialize telemetry (simplified for tests)
	telemetryConfig := &telemetry.Config{
		ServiceName:    "test-worker",
		ServiceVersion: "test",
		LogLevel:       "error",
		EnableMetrics:  false,
		EnableTracing:  false,
		EnableLogging:  false,
	}
	_ = telemetry.Init(telemetryConfig)

	// Create worker config
	cfg := &worker.Config{
		WorkerID:                  "test-worker",
		WorkerName:                "Test Worker",
		BatchSize:                 10,
		BatchTimeout:              2 * time.Second,
		MaxConcurrentBatches:      5,
		ProcessingConcurrency:     10,
		RehydrationBatchSize:      50,
		RehydrationInterval:       5 * time.Minute,
		StartupRehydrationEnabled: true,
		MaxRetries:                3,
		RetryBackoff:              1 * time.Second,
		RetryMultiplier:           2.0,
		MaxRetryBackoff:           30 * time.Second,
		MetricsInterval:           30 * time.Second,
		HealthCheckPort:           8081,
	}

	// Create worker metrics (simplified for tests)
	metrics := worker.NewMetrics()

	return worker.NewProcessor(cfg, testDB, testCache, testQueue, metrics), nil
}
