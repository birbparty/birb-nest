package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	// Test instance isolation between two instances
	t.Run("Data isolation between instances", func(t *testing.T) {
		// Create two instance caches with different instance IDs
		instance1Cache := cache.NewInstanceCache(testCache, "inst_game1")
		instance2Cache := cache.NewInstanceCache(testCache, "inst_game2")

		// Set same key in both instances
		key := "player:123:position"
		value1 := []byte(`{"x":100,"y":200}`)
		value2 := []byte(`{"x":500,"y":600}`)

		err := instance1Cache.Set(ctx, key, value1, 0)
		require.NoError(t, err)

		err = instance2Cache.Set(ctx, key, value2, 0)
		require.NoError(t, err)

		// Verify each instance gets its own data
		got1, err := instance1Cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value1, got1)

		got2, err := instance2Cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value2, got2)

		// Delete from instance1 shouldn't affect instance2
		err = instance1Cache.Delete(ctx, key)
		require.NoError(t, err)

		_, err = instance1Cache.Get(ctx, key)
		assert.Error(t, err) // Should be gone

		got2Again, err := instance2Cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value2, got2Again) // Should still exist
	})

	t.Run("Database isolation between instances", func(t *testing.T) {
		// Create database config from test environment
		dbConfig, err := parsePostgresURL(testContainers.PostgresURL)
		require.NoError(t, err)

		// Create database clients with different instance IDs
		db1, err := database.NewPostgreSQLClient(dbConfig, "inst_game1")
		require.NoError(t, err)
		defer db1.Close()

		db2, err := database.NewPostgreSQLClient(dbConfig, "inst_game2")
		require.NoError(t, err)
		defer db2.Close()

		// Set same key in both instances
		key := "game:state"
		value1 := []byte(`{"level":1,"score":100}`)
		value2 := []byte(`{"level":5,"score":5000}`)

		err = db1.Set(ctx, key, value1)
		require.NoError(t, err)

		err = db2.Set(ctx, key, value2)
		require.NoError(t, err)

		// Verify isolation
		got1, err := db1.Get(ctx, key)
		require.NoError(t, err)
		assert.JSONEq(t, string(value1), string(got1))

		got2, err := db2.Get(ctx, key)
		require.NoError(t, err)
		assert.JSONEq(t, string(value2), string(got2))
	})

	t.Run("Cross-instance queries fail appropriately", func(t *testing.T) {
		// Create instance cache with specific ID
		instanceCache := cache.NewInstanceCache(testCache, "inst_secure")

		// Set data in instance
		key := "secret:data"
		value := []byte(`{"secret":"confidential"}`)
		err := instanceCache.Set(ctx, key, value, 0)
		require.NoError(t, err)

		// Try to access with different instance
		differentInstanceCache := cache.NewInstanceCache(testCache, "inst_attacker")
		_, err = differentInstanceCache.Get(ctx, key)
		assert.Error(t, err) // Should not be able to access
	})
}

func TestInstanceCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("Can clean up instance data", func(t *testing.T) {
		instanceID := "inst_cleanup_test"

		// Create instance cache and add some data
		instanceCache := cache.NewInstanceCache(testCache, instanceID)

		// Add multiple keys
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("test:key:%d", i)
			value := fmt.Sprintf("value_%d", i)
			err := instanceCache.Set(ctx, key, []byte(value), 0)
			require.NoError(t, err)
		}

		// Verify data exists
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("test:key:%d", i)
			_, err := instanceCache.Get(ctx, key)
			require.NoError(t, err)
		}

		// Clean up instance data (would use SCAN in real implementation)
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("test:key:%d", i)
			err := instanceCache.Delete(ctx, key)
			require.NoError(t, err)
		}

		// Verify data is gone
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("test:key:%d", i)
			_, err := instanceCache.Get(ctx, key)
			assert.Error(t, err)
		}
	})

	t.Run("Database instance cleanup", func(t *testing.T) {
		instanceID := "inst_db_cleanup"

		// Create database config from test environment
		dbConfig, err := parsePostgresURL(testContainers.PostgresURL)
		require.NoError(t, err)

		db, err := database.NewPostgreSQLClient(dbConfig, instanceID)
		require.NoError(t, err)
		defer db.Close()

		// Add some data
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("db:key:%d", i)
			value := fmt.Sprintf(`{"data": "value_%d"}`, i)
			err := db.Set(ctx, key, []byte(value))
			require.NoError(t, err)
		}

		// In a real implementation, we would have a DeleteByInstance method
		// For now, clean up manually
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("db:key:%d", i)
			err := db.Delete(ctx, key)
			require.NoError(t, err)
		}
	})
}

func TestWebAPIInstanceFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("WebAPI instance ID format works correctly", func(t *testing.T) {
		// Test with realistic WebAPI instance IDs
		webAPIInstances := []string{
			"inst_1719432000_abc12345",
			"inst_1719432100_xyz98765",
			"inst_1719432200_def45678",
		}

		for _, instanceID := range webAPIInstances {
			instanceCache := cache.NewInstanceCache(testCache, instanceID)

			// Set and get data
			key := "player:p123:stats"
			value := []byte(`{"level":10,"health":100}`)

			err := instanceCache.Set(ctx, key, value, 0)
			require.NoError(t, err)

			got, err := instanceCache.Get(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, value, got)

			// Verify the actual Redis key format
			kb := instance.NewKeyBuilder(instanceID)
			actualKey := kb.CacheKey(key)
			expectedKey := fmt.Sprintf("instance:%s:cache:%s", instanceID, key)
			assert.Equal(t, expectedKey, actualKey)
		}
	})

	t.Run("Parent and child instance relationship", func(t *testing.T) {
		// Simulate overworld and dungeon instances
		overworldID := "inst_1719432000_overworld"
		dungeonID := "inst_1719432100_dungeon01"

		overworldCache := cache.NewInstanceCache(testCache, overworldID)
		dungeonCache := cache.NewInstanceCache(testCache, dungeonID)

		// Player enters dungeon - copy some data
		playerKey := "player:p123:position"
		overworldPos := []byte(`{"x":100,"y":200,"z":50,"world":"overworld"}`)
		dungeonPos := []byte(`{"x":0,"y":0,"z":0,"world":"dungeon"}`)

		err := overworldCache.Set(ctx, playerKey, overworldPos, 0)
		require.NoError(t, err)

		err = dungeonCache.Set(ctx, playerKey, dungeonPos, 0)
		require.NoError(t, err)

		// Verify isolation
		gotOverworld, err := overworldCache.Get(ctx, playerKey)
		require.NoError(t, err)
		assert.Equal(t, overworldPos, gotOverworld)

		gotDungeon, err := dungeonCache.Get(ctx, playerKey)
		require.NoError(t, err)
		assert.Equal(t, dungeonPos, gotDungeon)
	})
}

func TestInstanceKeyFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	resetAll(t)

	t.Run("Key format consistency", func(t *testing.T) {
		instanceID := "inst_test_format"
		kb := instance.NewKeyBuilder(instanceID)

		// Test various key formats
		testCases := []struct {
			name     string
			keyFunc  func() string
			expected string
		}{
			{
				name:     "Table key",
				keyFunc:  func() string { return kb.TableKey("users", "123") },
				expected: "instance:inst_test_format:table:users:row:123",
			},
			{
				name:     "Index key",
				keyFunc:  func() string { return kb.IndexKey("users_by_email", "test@example.com") },
				expected: "instance:inst_test_format:index:users_by_email:test@example.com",
			},
			{
				name:     "Cache key",
				keyFunc:  func() string { return kb.CacheKey("session", "abc123") },
				expected: "instance:inst_test_format:cache:session:abc123",
			},
			{
				name:     "Schema key",
				keyFunc:  func() string { return kb.SchemaKey("users") },
				expected: "instance:inst_test_format:schema:users",
			},
			{
				name:     "Event log key",
				keyFunc:  func() string { return kb.EventLogKey("1234567890") },
				expected: "instance:inst_test_format:eventlog:1234567890",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				actualKey := tc.keyFunc()
				assert.Equal(t, tc.expected, actualKey)
			})
		}
	})
}
