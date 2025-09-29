package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/api"
	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// setupTestRedis returns the global test cache client
func setupTestRedis(t testing.TB) (cache.Cache, func()) {
	if tt, ok := t.(*testing.T); ok {
		resetCache(tt)
	}
	return testCache, func() {}
}

// setupTestPostgreSQL returns the global test database
func setupTestPostgreSQL(t testing.TB) (database.Interface, func()) {
	if tt, ok := t.(*testing.T); ok {
		resetDatabase(tt)
	}
	// Create a database interface wrapper
	dbInterface, _ := database.NewPostgreSQLClient(&database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "test",
		Password: "test",
		Database: "test",
	}, "default")
	return dbInterface, func() {}
}

// TestInstanceContextPropagation tests end-to-end context propagation
func TestInstanceContextPropagation(t *testing.T) {
	// Setup test environment
	ctx := context.Background()
	cacheClient, cleanupCache := setupTestRedis(t)
	defer cleanupCache()

	db, cleanupDB := setupTestPostgreSQL(t)
	defer cleanupDB()

	// Create registry
	registry := instance.NewRegistry(cacheClient)

	// Create test instances
	instance1 := &instance.Context{
		InstanceID: "test-instance-1",
		GameType:   "minecraft",
		Region:     "us-east-1",
		Status:     instance.StatusActive,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Metadata: map[string]string{
			"owner":       "player1",
			"max_players": "20",
		},
	}

	instance2 := &instance.Context{
		InstanceID: "test-instance-2",
		GameType:   "terraria",
		Region:     "eu-west-1",
		Status:     instance.StatusActive,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Metadata: map[string]string{
			"owner":       "player2",
			"max_players": "10",
		},
	}

	// Register instances
	require.NoError(t, registry.Register(ctx, instance1))
	require.NoError(t, registry.Register(ctx, instance2))

	// Create API server
	cfg := &api.Config{
		Mode:           "primary",
		InstanceID:     "default",
		WriteQueueSize: 100,
		WriteWorkers:   2,
		Port:           0, // Let system assign port
	}

	handlers := api.NewHandlers(cfg, cacheClient, db, registry)
	defer handlers.Shutdown()

	app := fiber.New()
	api.SetupInstanceRoutes(app, handlers, cfg, registry)

	// Start server
	go app.Listen(":0")
	time.Sleep(100 * time.Millisecond) // Let server start

	// Get actual port
	addr := "localhost:8080" // For testing, use a fixed port
	baseURL := fmt.Sprintf("http://%s", addr)

	t.Run("Happy Path - Instance Context Propagation", func(t *testing.T) {
		// Set data with instance 1
		setReq := bytes.NewBufferString("test-value-1")
		req, _ := http.NewRequest("PUT", baseURL+"/v1/cache/test-key", setReq)
		req.Header.Set("X-Instance-ID", "test-instance-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Get data with instance 1 - should succeed
		req, _ = http.NewRequest("GET", baseURL+"/v1/cache/test-key", nil)
		req.Header.Set("X-Instance-ID", "test-instance-1")

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body []byte
		body, _ = io.ReadAll(resp.Body)
		assert.Equal(t, "test-value-1", string(body))
		resp.Body.Close()

		// Get data with instance 2 - should fail
		req, _ = http.NewRequest("GET", baseURL+"/v1/cache/test-key", nil)
		req.Header.Set("X-Instance-ID", "test-instance-2")

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Data Isolation Between Instances", func(t *testing.T) {
		// Set same key for both instances
		key := "shared-key"
		value1 := "instance-1-data"
		value2 := "instance-2-data"

		// Set for instance 1
		setReq := bytes.NewBufferString(value1)
		req, _ := http.NewRequest("PUT", baseURL+"/v1/cache/"+key, setReq)
		req.Header.Set("X-Instance-ID", "test-instance-1")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()

		// Set for instance 2
		setReq = bytes.NewBufferString(value2)
		req, _ = http.NewRequest("PUT", baseURL+"/v1/cache/"+key, setReq)
		req.Header.Set("X-Instance-ID", "test-instance-2")
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()

		// Get from instance 1
		req, _ = http.NewRequest("GET", baseURL+"/v1/cache/"+key, nil)
		req.Header.Set("X-Instance-ID", "test-instance-1")
		resp, _ = http.DefaultClient.Do(req)
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, value1, string(body))
		resp.Body.Close()

		// Get from instance 2
		req, _ = http.NewRequest("GET", baseURL+"/v1/cache/"+key, nil)
		req.Header.Set("X-Instance-ID", "test-instance-2")
		resp, _ = http.DefaultClient.Do(req)
		body, _ = io.ReadAll(resp.Body)
		assert.Equal(t, value2, string(body))
		resp.Body.Close()
	})

	t.Run("Missing Instance ID", func(t *testing.T) {
		// Try to access without instance ID
		req, _ := http.NewRequest("GET", baseURL+"/v1/cache/some-key", nil)
		// No X-Instance-ID header

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		assert.Equal(t, "MISSING_INSTANCE_ID", errorResp["code"])
		resp.Body.Close()
	})

	t.Run("Invalid Instance Status", func(t *testing.T) {
		// Create inactive instance
		inactiveInstance := &instance.Context{
			InstanceID: "test-instance-inactive",
			GameType:   "minecraft",
			Region:     "us-west-2",
			Status:     instance.StatusInactive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata: map[string]string{
				"owner":       "player3",
				"max_players": "5",
			},
		}
		registry.Register(ctx, inactiveInstance)

		// Try to access with inactive instance
		req, _ := http.NewRequest("GET", baseURL+"/v1/cache/test-key", nil)
		req.Header.Set("X-Instance-ID", "test-instance-inactive")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var errorResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		assert.Equal(t, "INSTANCE_INACTIVE", errorResp["code"])
		resp.Body.Close()
	})

	t.Run("Activity Tracking", func(t *testing.T) {
		// Get initial last active time
		inst, err := registry.Get(ctx, "test-instance-1")
		require.NoError(t, err)
		initialLastActive := inst.LastActive

		// Make a request
		req, _ := http.NewRequest("GET", baseURL+"/v1/cache/activity-test", nil)
		req.Header.Set("X-Instance-ID", "test-instance-1")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()

		// Wait for async update
		time.Sleep(100 * time.Millisecond)

		// Check last active was updated
		inst, err = registry.Get(ctx, "test-instance-1")
		require.NoError(t, err)
		assert.True(t, inst.LastActive.After(initialLastActive))
	})

	t.Run("Batch Operations with Instance Context", func(t *testing.T) {
		// Set some data for instance 1
		keys := []string{"batch-1", "batch-2", "batch-3"}
		for i, key := range keys {
			value := fmt.Sprintf("value-%d", i+1)
			setReq := bytes.NewBufferString(value)
			req, _ := http.NewRequest("PUT", baseURL+"/v1/cache/"+key, setReq)
			req.Header.Set("X-Instance-ID", "test-instance-1")
			resp, _ := http.DefaultClient.Do(req)
			resp.Body.Close()
		}

		// Batch get with instance 1
		batchReq := map[string][]string{
			"keys": append(keys, "missing-key"),
		}
		body, _ := json.Marshal(batchReq)
		req, _ := http.NewRequest("POST", baseURL+"/v1/cache/batch/get", bytes.NewReader(body))
		req.Header.Set("X-Instance-ID", "test-instance-1")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var batchResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&batchResp)
		entries := batchResp["entries"].(map[string]interface{})
		missing := batchResp["missing"].([]interface{})

		assert.Len(t, entries, 3)
		assert.Len(t, missing, 1)
		assert.Equal(t, "missing-key", missing[0])
		resp.Body.Close()
	})

	t.Run("Query Parameter Instance ID", func(t *testing.T) {
		// Use query parameter instead of header
		req, _ := http.NewRequest("GET", baseURL+"/v1/cache/query-test?instance_id=test-instance-1", nil)
		// No X-Instance-ID header

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		// Should work with query parameter
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Health Check Without Instance", func(t *testing.T) {
		// Health check should work without instance ID
		req, _ := http.NewRequest("GET", baseURL+"/health", nil)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var health map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&health)
		assert.Equal(t, "healthy", health["status"])
		resp.Body.Close()
	})
}

// TestInstanceRegistryOperations tests registry CRUD operations
func TestInstanceRegistryOperations(t *testing.T) {
	ctx := context.Background()
	cacheClient, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := instance.NewRegistry(cacheClient)

	t.Run("Register and Get Instance", func(t *testing.T) {
		inst := &instance.Context{
			InstanceID: "reg-test-1",
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata: map[string]string{
				"owner":       "test-user",
				"max_players": "10",
			},
		}

		// Register
		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Get
		retrieved, err := registry.Get(ctx, "reg-test-1")
		require.NoError(t, err)
		assert.Equal(t, inst.InstanceID, retrieved.InstanceID)
		assert.Equal(t, inst.GameType, retrieved.GameType)
		assert.Equal(t, inst.Region, retrieved.Region)
	})

	t.Run("Update Instance", func(t *testing.T) {
		// Get current instance
		inst, err := registry.Get(ctx, "reg-test-1")
		require.NoError(t, err)

		// Update status
		inst.Status = instance.StatusPaused
		inst.Metadata["current_players"] = "5"
		err = registry.Update(ctx, inst)
		require.NoError(t, err)

		// Verify update
		updated, err := registry.Get(ctx, "reg-test-1")
		require.NoError(t, err)
		assert.Equal(t, instance.StatusPaused, updated.Status)
		assert.Equal(t, "5", updated.Metadata["current_players"])
	})

	t.Run("List Instances with Filters", func(t *testing.T) {
		// Create more instances
		instances := []*instance.Context{
			{
				InstanceID: "list-test-1",
				GameType:   "minecraft",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
			},
			{
				InstanceID: "list-test-2",
				GameType:   "terraria",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
			},
			{
				InstanceID: "list-test-3",
				GameType:   "minecraft",
				Region:     "eu-west-1",
				Status:     instance.StatusInactive,
				CreatedAt:  time.Now(),
			},
		}

		for _, inst := range instances {
			registry.Register(ctx, inst)
		}

		// List all
		all, err := registry.List(ctx, instance.ListFilter{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 3)

		// List by status
		active, err := registry.List(ctx, instance.ListFilter{Status: instance.StatusActive})
		require.NoError(t, err)
		for _, inst := range active {
			assert.Equal(t, instance.StatusActive, inst.Status)
		}

		// List by game type
		minecraft, err := registry.List(ctx, instance.ListFilter{GameType: "minecraft"})
		require.NoError(t, err)
		for _, inst := range minecraft {
			assert.Equal(t, "minecraft", inst.GameType)
		}

		// List by region
		usEast, err := registry.List(ctx, instance.ListFilter{Region: "us-east-1"})
		require.NoError(t, err)
		for _, inst := range usEast {
			assert.Equal(t, "us-east-1", inst.Region)
		}
	})

	t.Run("Delete Instance", func(t *testing.T) {
		// Delete
		err := registry.Delete(ctx, "reg-test-1")
		require.NoError(t, err)

		// Verify deleted
		_, err = registry.Get(ctx, "reg-test-1")
		assert.Equal(t, instance.ErrInstanceNotFound, err)
	})

	t.Run("GetOrCreate Instance", func(t *testing.T) {
		// Get non-existent instance - should create
		inst, err := registry.GetOrCreate(ctx, "new-instance-1")
		require.NoError(t, err)
		assert.Equal(t, "new-instance-1", inst.InstanceID)
		assert.Equal(t, instance.StatusActive, inst.Status)
		assert.Equal(t, "default", inst.GameType)

		// Get existing instance - should not create
		inst2, err := registry.GetOrCreate(ctx, "new-instance-1")
		require.NoError(t, err)
		assert.Equal(t, inst.CreatedAt, inst2.CreatedAt) // Same creation time
	})
}

// BenchmarkInstanceContextPropagation benchmarks performance overhead
func BenchmarkInstanceContextPropagation(b *testing.B) {
	// Setup
	ctx := context.Background()
	cacheClient, cleanupCache := setupTestRedis(b)
	defer cleanupCache()

	registry := instance.NewRegistry(cacheClient)

	// Register test instance
	inst := &instance.Context{
		InstanceID: "bench-instance",
		GameType:   "minecraft",
		Region:     "us-east-1",
		Status:     instance.StatusActive,
		CreatedAt:  time.Now(),
	}
	registry.Register(ctx, inst)

	// Create handlers
	cfg := &api.Config{
		Mode:           "primary",
		InstanceID:     "default",
		WriteQueueSize: 1000,
		WriteWorkers:   4,
	}
	handlers := api.NewHandlers(cfg, cacheClient, nil, registry)
	defer handlers.Shutdown()

	app := fiber.New()
	api.SetupInstanceRoutes(app, handlers, cfg, registry)

	// Benchmark different operations
	b.Run("Set with Instance Context", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().Header.Set("X-Instance-ID", "bench-instance")
			c.Request().SetRequestURI(fmt.Sprintf("/v1/cache/bench-key-%d", i))
			c.Request().Header.SetMethod("PUT")
			c.Request().SetBody([]byte("benchmark-value"))

			handlers.Set(c)
			app.ReleaseCtx(c)
		}
	})

	b.Run("Get with Instance Context", func(b *testing.B) {
		// Pre-populate some data
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("preload-%d", i)
			kb := instance.NewKeyBuilder("bench-instance")
			cacheClient.Set(ctx, kb.CacheKey(key), []byte("value"), 0)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().Header.Set("X-Instance-ID", "bench-instance")
			c.Request().SetRequestURI(fmt.Sprintf("/v1/cache/preload-%d", i%100))
			c.Request().Header.SetMethod("GET")

			handlers.Get(c)
			app.ReleaseCtx(c)
		}
	})

	b.Run("Registry Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry.Get(ctx, "bench-instance")
		}
	})
}
