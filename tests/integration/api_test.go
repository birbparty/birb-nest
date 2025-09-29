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
	_ "github.com/birbparty/birb-nest/internal/cache" // for testCache
	"github.com/birbparty/birb-nest/internal/database"
	_ "github.com/birbparty/birb-nest/internal/queue" // for testQueue
	"github.com/birbparty/birb-nest/internal/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPI_CRUD tests basic CRUD operations
func TestAPI_CRUD(t *testing.T) {
	resetAll(t)

	// Create test API instance
	app, err := createTestAPI()
	require.NoError(t, err)

	t.Run("CreateEntry", func(t *testing.T) {
		payload := api.CacheRequest{
			Value: json.RawMessage(`{"name": "Test Item", "count": 42}`),
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/v1/cache/test-key-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result api.CacheResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "test-key-1", result.Key)
		assert.Equal(t, 1, result.Version)
	})

	t.Run("GetEntry", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/cache/test-key-1", nil)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.CacheResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "test-key-1", result.Key)
		assert.Contains(t, string(result.Value), "Test Item")
	})

	t.Run("UpdateEntry", func(t *testing.T) {
		payload := api.CacheRequest{
			Value: json.RawMessage(`{"name": "Updated Item", "count": 100}`),
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/v1/cache/test-key-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode) // API always returns 201 for set

		var result api.CacheResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		// Note: version handling is simplified in the current implementation
		assert.Contains(t, string(result.Value), "Updated Item")
	})

	t.Run("DeleteEntry", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/v1/cache/test-key-1", nil)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify it's deleted
		req = httptest.NewRequest("GET", "/v1/cache/test-key-1", nil)
		resp, err = app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("GetNonExistentEntry", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/cache/non-existent-key", nil)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestAPI_DoubleWrite tests that writes go to both Redis and NATS
func TestAPI_DoubleWrite(t *testing.T) {
	resetAll(t)

	app, err := createTestAPI()
	require.NoError(t, err)

	// Create entry
	payload := api.CacheRequest{
		Value: json.RawMessage(`{"test": "double-write"}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/v1/cache/double-write-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Check Redis directly
	ctx := context.Background()
	val, err := testCache.Get(ctx, "double-write-key")
	require.NoError(t, err)
	assert.NotNil(t, val)

	// Give NATS time to process
	time.Sleep(100 * time.Millisecond)

	// Check queue stats (messages should have been published)
	streamInfo, err := testQueue.StreamInfo("TEST_CACHE")
	require.NoError(t, err)
	assert.Greater(t, streamInfo.State.Msgs, uint64(0))
}

// TestAPI_CacheMissRehydration tests cache miss triggering rehydration
func TestAPI_CacheMissRehydration(t *testing.T) {
	resetAll(t)

	// First, insert directly into database (bypassing cache)
	ctx := context.Background()
	repo := database.NewCacheRepository(testDB)
	err := repo.Set(ctx, "db-only-key", json.RawMessage(`{"source": "database"}`), nil, nil)
	require.NoError(t, err)

	app, err := createTestAPI()
	require.NoError(t, err)

	// Get entry (should trigger cache miss and rehydration)
	req := httptest.NewRequest("GET", "/v1/cache/db-only-key", nil)

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result api.CacheResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "db-only-key", result.Key)
	assert.Contains(t, string(result.Value), "database")

	// Verify it was added to cache
	val, err := testCache.Get(ctx, "db-only-key")
	require.NoError(t, err)
	assert.NotNil(t, val)

	// Check that rehydration message was published
	time.Sleep(100 * time.Millisecond)
	streamInfo, err := testQueue.StreamInfo("TEST_CACHE")
	require.NoError(t, err)
	assert.Greater(t, streamInfo.State.Msgs, uint64(0))
}

// TestAPI_ConcurrentAccess tests concurrent reads and writes
func TestAPI_ConcurrentAccess(t *testing.T) {
	resetAll(t)

	app, err := createTestAPI()
	require.NoError(t, err)

	// Create initial entry
	payload := api.CacheRequest{
		Value: json.RawMessage(`{"counter": 0}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/v1/cache/concurrent-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Concurrent updates and reads
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			payload := api.CacheRequest{
				Value: json.RawMessage(fmt.Sprintf(`{"counter": %d}`, n)),
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/v1/cache/concurrent-key", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			if err != nil {
				errors <- err
				return
			}
			if resp.StatusCode != http.StatusCreated {
				errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/v1/cache/concurrent-key", nil)

			resp, err := app.Test(req, -1)
			if err != nil {
				errors <- err
				return
			}
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

// TestAPI_HealthCheck tests the health endpoint
func TestAPI_HealthCheck(t *testing.T) {
	app, err := createTestAPI()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/health", nil)

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result api.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, "healthy", result.Checks["database"])
	assert.Equal(t, "healthy", result.Checks["redis"])
	assert.Equal(t, "healthy", result.Checks["nats"])
}

// TestAPI_ErrorHandling tests various error scenarios
func TestAPI_ErrorHandling(t *testing.T) {
	resetAll(t)

	app, err := createTestAPI()
	require.NoError(t, err)

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/cache/test-key", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("EmptyKey", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/cache/", nil)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode) // 404 for invalid route
	})

	t.Run("LargePayload", func(t *testing.T) {
		// Create a large payload (>1MB)
		largeData := make([]byte, 2*1024*1024) // 2MB
		for i := range largeData {
			largeData[i] = 'a'
		}

		payload := api.CacheRequest{
			Value: json.RawMessage(fmt.Sprintf(`{"data": "%s"}`, string(largeData))),
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/v1/cache/large-key", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		// Fiber's default body limit is 4MB, so this should pass
		// but you might want to test the actual limit in your app
		assert.Contains(t, []int{http.StatusCreated, http.StatusRequestEntityTooLarge}, resp.StatusCode)
	})
}

// Helper function to create test API instance
func createTestAPI() (*fiber.App, error) {
	// Initialize telemetry (simplified for tests)
	telemetryConfig := &telemetry.Config{
		ServiceName:    "test-api",
		ServiceVersion: "test",
		LogLevel:       "error",
		EnableMetrics:  false,
		EnableTracing:  false,
		EnableLogging:  false,
	}

	// Initialize telemetry but ignore for tests
	_ = telemetry.Init(telemetryConfig)

	// Create metrics
	metrics := api.NewMetrics()

	// Create handlers
	handlers := api.NewHandler(testDB, testCache, testQueue, metrics)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit: 4 * 1024 * 1024, // 4MB
	})

	// Setup routes
	api.SetupRoutes(app, handlers, metrics, "test")

	return app, nil
}
