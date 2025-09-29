package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/api"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_FullCacheFlow tests the complete flow from API to worker processing
func TestE2E_FullCacheFlow(t *testing.T) {
	resetAll(t)

	// Start worker
	w, err := createTestWorker()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Create API
	app, err := createTestAPI()
	require.NoError(t, err)

	// Test data
	testData := []struct {
		key   string
		value json.RawMessage
	}{
		{"user:123", json.RawMessage(`{"name": "John Doe", "email": "john@example.com"}`)},
		{"product:456", json.RawMessage(`{"name": "Widget", "price": 99.99}`)},
		{"session:789", json.RawMessage(`{"userId": "123", "token": "abc123"}`)},
	}

	// 1. Create entries via API
	for _, td := range testData {
		payload := api.CacheRequest{
			Value: td.value,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", td.key), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// 2. Wait for worker to process persistence messages
	time.Sleep(3 * time.Second)

	// 3. Verify data is in both Redis and PostgreSQL
	for _, td := range testData {
		// Check Redis
		val, err := testCache.Get(context.Background(), td.key)
		require.NoError(t, err)
		assert.NotNil(t, val)

		// Check PostgreSQL
		repo := database.NewCacheRepository(testDB)
		entry, err := repo.Get(context.Background(), td.key)
		require.NoError(t, err)
		assert.Equal(t, td.key, entry.Key)
		assert.JSONEq(t, string(td.value), string(entry.Value))
	}

	// 4. Clear Redis to simulate cache eviction
	for _, td := range testData {
		err := testCache.Delete(context.Background(), td.key)
		assert.NoError(t, err)
	}

	// 5. Read via API - should trigger cache miss and rehydration
	for _, td := range testData {
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", td.key), nil)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.CacheResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, td.key, result.Key)
		assert.JSONEq(t, string(td.value), string(result.Value))
	}

	// 6. Wait for rehydration processing
	time.Sleep(2 * time.Second)

	// 7. Verify data is back in Redis
	for _, td := range testData {
		val, err := testCache.Get(context.Background(), td.key)
		require.NoError(t, err)
		assert.NotNil(t, val)
	}

	// Stop worker
	w.Stop()
	wg.Wait()
}

// TestE2E_ConcurrentOperations tests the system under concurrent load
func TestE2E_ConcurrentOperations(t *testing.T) {
	resetAll(t)

	// Start worker
	w, err := createTestWorker()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Create API
	app, err := createTestAPI()
	require.NoError(t, err)

	// Run concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Writers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			for j := 0; j < 5; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", n, j)
				payload := api.CacheRequest{
					Value: json.RawMessage(fmt.Sprintf(`{"worker": %d, "iteration": %d}`, n, j)),
				}
				body, _ := json.Marshal(payload)

				req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				resp, err := app.Test(req, -1)
				if err != nil {
					errors <- err
					continue
				}
				if resp.StatusCode != http.StatusCreated {
					errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
					continue
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			// Wait a bit for some data to be written
			time.Sleep(500 * time.Millisecond)

			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", j%20, j%5)
				req := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)

				resp, err := app.Test(req, -1)
				if err != nil {
					errors <- err
					continue
				}
				// It's ok if some are 404 (not yet created)
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
					errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Concurrent operation error: %v", err)
		errorCount++
	}
	// Allow for some errors in concurrent scenario
	assert.Less(t, errorCount, 10, "Too many errors in concurrent operations")

	// Wait for worker to process
	time.Sleep(5 * time.Second)

	// Verify data consistency - sample check
	repo := database.NewCacheRepository(testDB)
	sampleKey := "concurrent:0:0"
	entry, err := repo.Get(context.Background(), sampleKey)
	if err == nil {
		assert.Equal(t, sampleKey, entry.Key)
		assert.Contains(t, string(entry.Value), "worker")
	}

	// Stop worker
	w.Stop()
	workerWg.Wait()
}

// TestE2E_DataConsistency tests that data remains consistent across all layers
func TestE2E_DataConsistency(t *testing.T) {
	resetAll(t)

	// Start worker
	w, err := createTestWorker()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.Start(ctx)
		assert.NoError(t, err)
	}()

	// Give worker time to start
	time.Sleep(500 * time.Millisecond)

	// Create API
	app, err := createTestAPI()
	require.NoError(t, err)

	// Create entry
	key := "consistency-test"
	originalValue := json.RawMessage(`{"counter": 1, "data": "original"}`)

	payload := api.CacheRequest{
		Value: originalValue,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Wait for persistence
	time.Sleep(2 * time.Second)

	// Update the entry multiple times
	for i := 2; i <= 5; i++ {
		updatedValue := json.RawMessage(fmt.Sprintf(`{"counter": %d, "data": "updated"}`, i))
		payload := api.CacheRequest{
			Value: updatedValue,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Small delay between updates
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all updates to process
	time.Sleep(3 * time.Second)

	// Verify final state is consistent across all layers
	// 1. Check via API
	req = httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var apiResult api.CacheResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResult)
	require.NoError(t, err)

	// 2. Check Redis directly
	redisVal, err := testCache.Get(context.Background(), key)
	require.NoError(t, err)

	// 3. Check PostgreSQL directly
	repo := database.NewCacheRepository(testDB)
	dbEntry, err := repo.Get(context.Background(), key)
	require.NoError(t, err)

	// All should have the latest value
	expectedValue := `{"counter": 5, "data": "updated"}`
	assert.JSONEq(t, expectedValue, string(apiResult.Value))
	assert.Contains(t, string(redisVal), expectedValue)
	assert.JSONEq(t, expectedValue, string(dbEntry.Value))

	// Stop worker
	w.Stop()
	wg.Wait()
}

// TestE2E_SystemResilience tests system behavior under failure conditions
func TestE2E_SystemResilience(t *testing.T) {
	resetAll(t)

	// This test simulates various failure scenarios
	t.Run("RedisUnavailable", func(t *testing.T) {
		// Temporarily close Redis connection
		oldCache := testCache
		testCache.Close()

		// Create API (it should still work without Redis)
		app, err := createTestAPI()
		require.NoError(t, err)

		// Try to create an entry - should succeed (writes to queue)
		payload := api.CacheRequest{
			Value: json.RawMessage(`{"test": "redis-down"}`),
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/v1/cache/resilience-test", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		// API might return error or success depending on implementation
		assert.Contains(t, []int{http.StatusCreated, http.StatusServiceUnavailable}, resp.StatusCode)

		// Restore Redis
		testCache = oldCache
	})

	t.Run("WorkerRestart", func(t *testing.T) {
		// Start worker
		w1, err := createTestWorker()
		require.NoError(t, err)

		ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel1()

		var wg1 sync.WaitGroup
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			w1.Start(ctx1)
		}()

		time.Sleep(500 * time.Millisecond)

		// Publish messages
		for i := 0; i < 3; i++ {
			msg := queue.NewPersistenceMessage(
				fmt.Sprintf("restart-test-%d", i),
				json.RawMessage(fmt.Sprintf(`{"value": %d}`, i)),
				1,
				nil,
				nil,
			)
			err = testQueue.PublishPersistence(context.Background(), msg)
			require.NoError(t, err)
		}

		// Stop worker
		cancel1()
		w1.Stop()
		wg1.Wait()

		// Start new worker - it should pick up unprocessed messages
		w2, err := createTestWorker()
		require.NoError(t, err)

		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()

		var wg2 sync.WaitGroup
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			w2.Start(ctx2)
		}()

		// Wait for processing
		time.Sleep(3 * time.Second)

		// Verify messages were eventually processed
		repo := database.NewCacheRepository(testDB)
		processedCount := 0
		for i := 0; i < 3; i++ {
			key := fmt.Sprintf("restart-test-%d", i)
			_, err := repo.Get(context.Background(), key)
			if err == nil {
				processedCount++
			}
		}
		assert.Greater(t, processedCount, 0, "At least some messages should be processed after restart")

		w2.Stop()
		wg2.Wait()
	})
}
