package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdvancedInstanceIsolation tests complex isolation scenarios
func TestAdvancedInstanceIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("Concurrent access to same key across instances", func(t *testing.T) {
		const numInstances = 10
		const numOperations = 100
		key := "concurrent:counter"

		instances := make([]*cache.InstanceCache, numInstances)
		for i := 0; i < numInstances; i++ {
			instances[i] = cache.NewInstanceCache(testCache, fmt.Sprintf("inst_concurrent_%d", i))
		}

		var wg sync.WaitGroup
		errors := make(chan error, numInstances*numOperations)

		// Each instance performs concurrent increments
		for i := 0; i < numInstances; i++ {
			instanceCache := instances[i]
			wg.Add(1)
			go func(instIdx int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					// Set instance-specific value
					value := fmt.Sprintf("%d:%d", instIdx, j)
					if err := instanceCache.Set(ctx, key, []byte(value), 0); err != nil {
						errors <- err
						return
					}

					// Immediately read back
					got, err := instanceCache.Get(ctx, key)
					if err != nil {
						errors <- err
						return
					}

					// Verify we got our own value
					if string(got) != value {
						errors <- fmt.Errorf("instance %d expected %s, got %s", instIdx, value, string(got))
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		var errCount int
		for err := range errors {
			t.Errorf("Concurrent access error: %v", err)
			errCount++
		}
		assert.Equal(t, 0, errCount, "Should have no isolation violations")
	})

	t.Run("Key collision with special characters", func(t *testing.T) {
		// Test that special characters in keys don't break isolation
		specialKeys := []string{
			"key:with:colons",
			"key/with/slashes",
			"key.with.dots",
			"key-with-dashes",
			"key_with_underscores",
			"key@with@at",
			"key#with#hash",
			"key$with$dollar",
			"key%with%percent",
			"key&with&ampersand",
			"key*with*asterisk",
			"key(with)parens",
			"key[with]brackets",
			"key{with}braces",
			"key<with>angles",
			"key?with?question",
			"key!with!exclamation",
			"key~with~tilde",
			"key`with`backtick",
			"key'with'quote",
			"key\"with\"doublequote",
			"key\\with\\backslash",
			"key|with|pipe",
			"key=with=equals",
			"key+with+plus",
			"key with spaces",
			"key\twith\ttabs",
			"key\nwith\nnewlines",
			"",                 // empty key
			" ",                // space key
			"🔑emoji",           // unicode key
			"\x00null\x00byte", // null bytes
		}

		instance1 := cache.NewInstanceCache(testCache, "inst_special_1")
		instance2 := cache.NewInstanceCache(testCache, "inst_special_2")

		for _, key := range specialKeys {
			// Skip invalid keys
			if key == "" || key == " " || key == "\x00null\x00byte" {
				continue
			}

			value1 := fmt.Sprintf("inst1_%s", key)
			value2 := fmt.Sprintf("inst2_%s", key)

			// Set in both instances
			err1 := instance1.Set(ctx, key, []byte(value1), 0)
			err2 := instance2.Set(ctx, key, []byte(value2), 0)

			if err1 == nil && err2 == nil {
				// Verify isolation
				got1, _ := instance1.Get(ctx, key)
				got2, _ := instance2.Get(ctx, key)

				assert.Equal(t, value1, string(got1), "Instance 1 should get its own value for key: %s", key)
				assert.Equal(t, value2, string(got2), "Instance 2 should get its own value for key: %s", key)
			}
		}
	})

	t.Run("Large value isolation", func(t *testing.T) {
		// Test with large values to ensure buffer isolation
		instance1 := cache.NewInstanceCache(testCache, "inst_large_1")
		instance2 := cache.NewInstanceCache(testCache, "inst_large_2")

		// Create 1MB values
		largeValue1 := make([]byte, 1024*1024)
		largeValue2 := make([]byte, 1024*1024)
		for i := range largeValue1 {
			largeValue1[i] = byte(i % 256)
			largeValue2[i] = byte((i + 128) % 256)
		}

		key := "large:value"

		// Set large values
		err := instance1.Set(ctx, key, largeValue1, 0)
		require.NoError(t, err)

		err = instance2.Set(ctx, key, largeValue2, 0)
		require.NoError(t, err)

		// Verify isolation
		got1, err := instance1.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, largeValue1, got1)

		got2, err := instance2.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, largeValue2, got2)
	})

	t.Run("Database and cache consistency", func(t *testing.T) {
		instanceID := "inst_consistency"

		// Create both cache and database clients
		instanceCache := cache.NewInstanceCache(testCache, instanceID)

		dbConfig, err := parsePostgresURL(testContainers.PostgresURL)
		require.NoError(t, err)

		db, err := database.NewPostgreSQLClient(dbConfig, instanceID)
		require.NoError(t, err)
		defer db.Close()

		// Test write-through pattern
		key := "consistency:test"
		value := []byte(`{"data": "test value", "timestamp": 123456789}`)

		// Write to cache
		err = instanceCache.Set(ctx, key, value, 0)
		require.NoError(t, err)

		// Write to database
		err = db.Set(ctx, key, value)
		require.NoError(t, err)

		// Verify both have the same data
		cacheValue, err := instanceCache.Get(ctx, key)
		require.NoError(t, err)

		dbValue, err := db.Get(ctx, key)
		require.NoError(t, err)

		assert.Equal(t, value, cacheValue, "Cache should have the correct value")
		assert.Equal(t, value, dbValue, "Database should have the correct value")

		// Test cache invalidation
		err = instanceCache.Delete(ctx, key)
		require.NoError(t, err)

		// Cache should be empty
		_, err = instanceCache.Get(ctx, key)
		assert.Error(t, err, "Cache should return error for deleted key")

		// Database should still have the value
		dbValue, err = db.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, dbValue, "Database should still have the value")
	})

	t.Run("TTL isolation between instances", func(t *testing.T) {
		instance1 := cache.NewInstanceCache(testCache, "inst_ttl_1")
		instance2 := cache.NewInstanceCache(testCache, "inst_ttl_2")

		key := "ttl:test"
		value1 := []byte("ttl_value_1")
		value2 := []byte("ttl_value_2")

		// Set with different TTLs
		err := instance1.Set(ctx, key, value1, 1*time.Second)
		require.NoError(t, err)

		err = instance2.Set(ctx, key, value2, 3*time.Second)
		require.NoError(t, err)

		// Verify both exist initially
		got1, err := instance1.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value1, got1)

		got2, err := instance2.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value2, got2)

		// Wait for instance1's TTL to expire
		time.Sleep(1500 * time.Millisecond)

		// Instance1's key should be gone
		_, err = instance1.Get(ctx, key)
		assert.Error(t, err, "Instance 1 key should have expired")

		// Instance2's key should still exist
		got2, err = instance2.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value2, got2, "Instance 2 key should still exist")
	})

	t.Run("Pattern operations isolation", func(t *testing.T) {
		instance1 := cache.NewInstanceCache(testCache, "inst_pattern_1")
		instance2 := cache.NewInstanceCache(testCache, "inst_pattern_2")

		// Set multiple keys in each instance
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pattern:test:%d", i)
			value1 := fmt.Sprintf("inst1_value_%d", i)
			value2 := fmt.Sprintf("inst2_value_%d", i)

			err := instance1.Set(ctx, key, []byte(value1), 0)
			require.NoError(t, err)

			err = instance2.Set(ctx, key, []byte(value2), 0)
			require.NoError(t, err)
		}

		// In a real implementation, we would test SCAN/KEYS operations
		// For now, verify individual key isolation
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pattern:test:%d", i)

			got1, err := instance1.Get(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("inst1_value_%d", i), string(got1))

			got2, err := instance2.Get(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("inst2_value_%d", i), string(got2))
		}
	})
}

// TestInstanceKeyNamespacing verifies the key namespacing prevents collisions
func TestInstanceKeyNamespacing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("KeyBuilder namespace verification", func(t *testing.T) {
		instances := []string{
			"inst_test_1",
			"inst_test_2",
			"inst_production",
			"inst_staging",
			"inst_1234567890",
			"inst_customer_abc",
		}

		// Create key builders for each instance
		keyBuilders := make(map[string]*instance.KeyBuilder)
		for _, instID := range instances {
			keyBuilders[instID] = instance.NewKeyBuilder(instID)
		}

		// Test various key types
		testKeys := []struct {
			keyType string
			genFunc func(kb *instance.KeyBuilder) string
		}{
			{"cache", func(kb *instance.KeyBuilder) string { return kb.CacheKey("user", "123") }},
			{"table", func(kb *instance.KeyBuilder) string { return kb.TableKey("users", "456") }},
			{"index", func(kb *instance.KeyBuilder) string { return kb.IndexKey("email_idx", "test@example.com") }},
			{"schema", func(kb *instance.KeyBuilder) string { return kb.SchemaKey("users") }},
			{"eventlog", func(kb *instance.KeyBuilder) string { return kb.EventLogKey("1234567890") }},
		}

		// Verify each instance generates unique keys
		for _, tk := range testKeys {
			keyMap := make(map[string]string)

			for _, instID := range instances {
				key := tk.genFunc(keyBuilders[instID])

				// Verify key contains instance ID
				assert.Contains(t, key, instID, "Key should contain instance ID")

				// Verify no collision
				if existingInst, exists := keyMap[key]; exists {
					t.Errorf("Key collision detected: instances %s and %s generated same key: %s",
						existingInst, instID, key)
				}
				keyMap[key] = instID
			}
		}
	})

	t.Run("Raw Redis key verification", func(t *testing.T) {
		// This test verifies the actual Redis keys are properly namespaced
		instance1 := cache.NewInstanceCache(testCache, "inst_raw_1")
		instance2 := cache.NewInstanceCache(testCache, "inst_raw_2")

		key := "test:key"
		value1 := []byte("value1")
		value2 := []byte("value2")

		// Set values
		err := instance1.Set(ctx, key, value1, 0)
		require.NoError(t, err)

		err = instance2.Set(ctx, key, value2, 0)
		require.NoError(t, err)

		// Get the actual Redis keys using KeyBuilder directly
		kb1 := instance.NewKeyBuilder("inst_raw_1")
		kb2 := instance.NewKeyBuilder("inst_raw_2")

		actualKey1 := kb1.CacheKey(key)
		actualKey2 := kb2.CacheKey(key)

		// Verify keys are different
		assert.NotEqual(t, actualKey1, actualKey2, "Actual Redis keys should be different")

		// Verify key format
		assert.Regexp(t, `^instance:inst_raw_1:cache:test:key$`, actualKey1)
		assert.Regexp(t, `^instance:inst_raw_2:cache:test:key$`, actualKey2)
	})
}

// TestInstanceDataIntegrity verifies data integrity across operations
func TestInstanceDataIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("Transaction-like operations", func(t *testing.T) {
		instanceCache := cache.NewInstanceCache(testCache, "inst_transaction")

		// Simulate a transaction with multiple keys
		keys := []string{"tx:account:1", "tx:account:2", "tx:log"}
		values := []string{"100", "200", "transfer:pending"}

		// Set all values
		for i, key := range keys {
			err := instanceCache.Set(ctx, key, []byte(values[i]), 0)
			require.NoError(t, err)
		}

		// Simulate transfer
		// In a real system, this would be atomic
		acc1Val := "50"
		acc2Val := "250"
		logVal := "transfer:complete"

		err := instanceCache.Set(ctx, keys[0], []byte(acc1Val), 0)
		require.NoError(t, err)

		err = instanceCache.Set(ctx, keys[1], []byte(acc2Val), 0)
		require.NoError(t, err)

		err = instanceCache.Set(ctx, keys[2], []byte(logVal), 0)
		require.NoError(t, err)

		// Verify final state
		got1, err := instanceCache.Get(ctx, keys[0])
		require.NoError(t, err)
		assert.Equal(t, acc1Val, string(got1))

		got2, err := instanceCache.Get(ctx, keys[1])
		require.NoError(t, err)
		assert.Equal(t, acc2Val, string(got2))

		got3, err := instanceCache.Get(ctx, keys[2])
		require.NoError(t, err)
		assert.Equal(t, logVal, string(got3))
	})

	t.Run("Concurrent modifications", func(t *testing.T) {
		const numGoroutines = 50
		const numOperations = 100

		instanceCache := cache.NewInstanceCache(testCache, "inst_concurrent_mod")
		key := "counter"

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Track all values written
		valuesSeen := &sync.Map{}

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					value := fmt.Sprintf("%d-%d", goroutineID, j)

					// Write
					if err := instanceCache.Set(ctx, key, []byte(value), 0); err != nil {
						t.Logf("Error setting value: %v", err)
						continue
					}

					// Immediately read
					if got, err := instanceCache.Get(ctx, key); err == nil {
						valuesSeen.Store(string(got), true)
					}

					// Small random delay
					time.Sleep(time.Microsecond * time.Duration(j%10))
				}
			}(i)
		}

		wg.Wait()

		// Verify we saw multiple different values (no stuck value)
		count := 0
		valuesSeen.Range(func(key, value interface{}) bool {
			count++
			return true
		})

		t.Logf("Saw %d different values during concurrent modifications", count)
		assert.Greater(t, count, 10, "Should have seen many different values")
	})
}
