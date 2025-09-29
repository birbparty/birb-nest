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
	"github.com/birbparty/birb-nest/internal/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstanceLifecycle tests complete instance lifecycle from creation to cleanup
func TestInstanceLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	// Create registry and operations
	registry := instance.NewRegistry(testCache)

	dbConfig, err := parsePostgresURL(testContainers.PostgresURL)
	require.NoError(t, err)

	db, err := database.NewPostgreSQLClient(dbConfig, "global")
	require.NoError(t, err)
	defer db.Close()

	// Create operations - note: in test environment we pass nil for DB
	// as the operations mainly use cache and the DB operations are mocked
	ops := operations.NewInstanceOperations(testCache, nil, registry)

	t.Run("Complete lifecycle flow", func(t *testing.T) {
		instanceID := "inst_lifecycle_test"

		// Phase 1: Creation
		inst := &instance.Context{
			InstanceID: instanceID,
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata: map[string]string{
				"owner":       "test_user",
				"version":     "1.0",
				"max_players": "20",
				"cur_players": "0",
			},
		}

		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Verify creation
		retrieved, err := registry.Get(ctx, instanceID)
		require.NoError(t, err)
		assert.Equal(t, inst.InstanceID, retrieved.InstanceID)
		assert.Equal(t, instance.StatusActive, retrieved.Status)

		// Phase 2: Active usage
		instanceCache := cache.NewInstanceCache(testCache, instanceID)

		// Add some data
		testData := map[string]string{
			"player:123:stats": `{"level":10,"xp":1500}`,
			"world:spawn":      `{"x":0,"y":64,"z":0}`,
			"config:settings":  `{"difficulty":"normal"}`,
			"session:abc123":   `{"player":"123","started":1234567890}`,
		}

		for key, value := range testData {
			err := instanceCache.Set(ctx, key, []byte(value), 0)
			require.NoError(t, err)
		}

		// Update activity
		err = registry.UpdateLastActive(ctx, instanceID)
		require.NoError(t, err)

		// Phase 3: Status transitions
		// Pause instance
		retrieved, err = registry.Get(ctx, instanceID)
		require.NoError(t, err)
		retrieved.Status = instance.StatusPaused
		err = registry.Update(ctx, retrieved)
		require.NoError(t, err)

		retrieved, err = registry.Get(ctx, instanceID)
		require.NoError(t, err)
		assert.Equal(t, instance.StatusPaused, retrieved.Status)

		// Resume instance
		retrieved.Status = instance.StatusActive
		retrieved.Metadata["cur_players"] = "5"
		err = registry.Update(ctx, retrieved)
		require.NoError(t, err)

		// Phase 4: Deletion preparation
		retrieved, err = registry.Get(ctx, instanceID)
		require.NoError(t, err)
		retrieved.Status = instance.StatusDeleting
		retrieved.Metadata["deletion_initiated"] = time.Now().Format(time.RFC3339)
		err = registry.Update(ctx, retrieved)
		require.NoError(t, err)

		// Verify data still exists during deletion status
		for key := range testData {
			got, err := instanceCache.Get(ctx, key)
			assert.NoError(t, err)
			assert.NotEmpty(t, got)
		}

		// Phase 5: Cleanup
		err = ops.DeleteInstance(ctx, instanceID)
		require.NoError(t, err)

		// Verify instance is gone
		_, err = registry.Get(ctx, instanceID)
		assert.Equal(t, instance.ErrInstanceNotFound, err)

		// Note: Cache cleanup verification is limited because our test implementation
		// doesn't support scanning keys for deletion
	})

	t.Run("Activity tracking accuracy", func(t *testing.T) {
		instanceID := "inst_activity_tracking"

		inst := &instance.Context{
			InstanceID: instanceID,
			GameType:   "terraria",
			Region:     "eu-west-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now().Add(-10 * time.Minute), // Start with old activity
		}

		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Get initial state
		initial, err := registry.Get(ctx, instanceID)
		require.NoError(t, err)
		initialActivity := initial.LastActive

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Update activity
		err = registry.UpdateLastActive(ctx, instanceID)
		require.NoError(t, err)

		// Verify update
		updated, err := registry.Get(ctx, instanceID)
		require.NoError(t, err)
		assert.True(t, updated.LastActive.After(initialActivity),
			"LastActive should be updated: was %v, now %v",
			initialActivity, updated.LastActive)

		// Verify idempotency - multiple updates in quick succession
		for i := 0; i < 5; i++ {
			err = registry.UpdateLastActive(ctx, instanceID)
			require.NoError(t, err)
		}

		// Clean up
		err = registry.Delete(ctx, instanceID)
		require.NoError(t, err)
	})

	t.Run("Concurrent lifecycle operations", func(t *testing.T) {
		const numInstances = 20
		var wg sync.WaitGroup
		errors := make(chan error, numInstances*3) // create, update, delete

		for i := 0; i < numInstances; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				instanceID := fmt.Sprintf("inst_concurrent_%d", idx)

				// Create
				inst := &instance.Context{
					InstanceID: instanceID,
					GameType:   "minecraft",
					Region:     "us-west-2",
					Status:     instance.StatusActive,
					CreatedAt:  time.Now(),
					LastActive: time.Now(),
				}

				if err := registry.Register(ctx, inst); err != nil {
					errors <- fmt.Errorf("create %s: %w", instanceID, err)
					return
				}

				// Random updates
				for j := 0; j < 5; j++ {
					time.Sleep(time.Millisecond * time.Duration(j))

					// Get current state
					current, err := registry.Get(ctx, instanceID)
					if err != nil {
						errors <- fmt.Errorf("get %s: %w", instanceID, err)
						continue
					}

					// Update
					current.Metadata["update_count"] = fmt.Sprintf("%d", j)
					current.LastActive = time.Now()

					if err := registry.Update(ctx, current); err != nil {
						errors <- fmt.Errorf("update %s: %w", instanceID, err)
					}
				}

				// Delete
				if err := registry.Delete(ctx, instanceID); err != nil {
					errors <- fmt.Errorf("delete %s: %w", instanceID, err)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		var errCount int
		for err := range errors {
			t.Errorf("Concurrent operation error: %v", err)
			errCount++
		}
		assert.Equal(t, 0, errCount, "Should have no errors in concurrent operations")
	})

	t.Run("Instance resurrection prevention", func(t *testing.T) {
		instanceID := "inst_resurrection"

		// Create and delete instance
		inst := &instance.Context{
			InstanceID: instanceID,
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
		}

		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Add some data
		instanceCache := cache.NewInstanceCache(testCache, instanceID)
		err = instanceCache.Set(ctx, "test:key", []byte("test:value"), 0)
		require.NoError(t, err)

		// Delete instance
		err = ops.DeleteInstance(ctx, instanceID)
		require.NoError(t, err)

		// Try to use the deleted instance
		err = registry.UpdateLastActive(ctx, instanceID)
		assert.Equal(t, instance.ErrInstanceNotFound, err)

		// Try to update the deleted instance
		deletedInst := &instance.Context{
			InstanceID: instanceID,
			Status:     instance.StatusActive,
		}
		err = registry.Update(ctx, deletedInst)
		assert.Equal(t, instance.ErrInstanceNotFound, err)

		// Note: Cache data verification is limited in test environment
	})

	t.Run("Status transition validation", func(t *testing.T) {
		instanceID := "inst_status_transitions"

		// Valid transitions map
		validTransitions := map[instance.InstanceStatus][]instance.InstanceStatus{
			instance.StatusActive:    {instance.StatusPaused, instance.StatusDeleting, instance.StatusMigrating},
			instance.StatusPaused:    {instance.StatusActive, instance.StatusDeleting},
			instance.StatusMigrating: {instance.StatusActive, instance.StatusDeleting},
			instance.StatusDeleting:  {instance.StatusInactive},
			instance.StatusInactive:  {instance.StatusActive}, // Can be reactivated
		}

		// Test each valid transition
		for fromStatus, toStatuses := range validTransitions {
			for _, toStatus := range toStatuses {
				testID := fmt.Sprintf("%s_from_%s_to_%s", instanceID, fromStatus, toStatus)

				inst := &instance.Context{
					InstanceID: testID,
					GameType:   "minecraft",
					Region:     "us-east-1",
					Status:     fromStatus,
					CreatedAt:  time.Now(),
					LastActive: time.Now(),
					Metadata:   make(map[string]string),
				}

				err := registry.Register(ctx, inst)
				require.NoError(t, err)

				// Attempt transition
				inst.Status = toStatus
				if toStatus == instance.StatusDeleting {
					inst.Metadata["deletion_initiated"] = time.Now().Format(time.RFC3339)
				}
				err = registry.Update(ctx, inst)
				assert.NoError(t, err, "Transition from %s to %s should be valid", fromStatus, toStatus)

				// Clean up
				registry.Delete(ctx, testID)
			}
		}
	})
}

// TestInstanceAutoCleanup tests automatic cleanup of inactive instances
func TestInstanceAutoCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	registry := instance.NewRegistry(testCache)

	dbConfig, err := parsePostgresURL(testContainers.PostgresURL)
	require.NoError(t, err)

	db, err := database.NewPostgreSQLClient(dbConfig, "global")
	require.NoError(t, err)
	defer db.Close()

	t.Run("Manual cleanup of inactive instances", func(t *testing.T) {
		// Create test instances with different activity levels
		instances := []struct {
			id          string
			lastActive  time.Time
			shouldClean bool
		}{
			{"inst_active", time.Now(), false},
			{"inst_recent", time.Now().Add(-1 * time.Second), false},
			{"inst_inactive", time.Now().Add(-5 * time.Second), true},
			{"inst_very_old", time.Now().Add(-1 * time.Hour), true},
			{"global", time.Now().Add(-1 * time.Hour), false}, // Protected
		}

		// Register instances
		for _, inst := range instances {
			ctx := &instance.Context{
				InstanceID: inst.id,
				GameType:   "minecraft",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now().Add(-2 * time.Hour),
				LastActive: inst.lastActive,
				Metadata:   make(map[string]string),
			}

			// Mark global as permanent
			if inst.id == "global" {
				ctx.IsPermanent = true
			}

			err := registry.Register(context.Background(), ctx)
			require.NoError(t, err)

			// Add some data to each instance
			instanceCache := cache.NewInstanceCache(testCache, inst.id)
			err = instanceCache.Set(context.Background(), "test:data", []byte("value"), 0)
			require.NoError(t, err)
		}

		// Manual cleanup simulation
		inactivityThreshold := 2 * time.Second
		now := time.Now()

		// Get all instances
		allInstances, err := registry.List(context.Background(), instance.ListFilter{})
		require.NoError(t, err)

		// Identify instances to clean
		for _, inst := range allInstances {
			inactive := now.Sub(inst.LastActive) > inactivityThreshold
			if inactive && inst.CanBeAutoDeleted() {
				err := registry.Delete(context.Background(), inst.InstanceID)
				assert.NoError(t, err)
			}
		}

		// Verify cleanup results
		for _, inst := range instances {
			_, err := registry.Get(context.Background(), inst.id)
			if inst.shouldClean && inst.id != "global" {
				assert.Equal(t, instance.ErrInstanceNotFound, err,
					"Instance %s should have been cleaned up", inst.id)
			} else {
				assert.NoError(t, err,
					"Instance %s should NOT have been cleaned up", inst.id)
			}
		}
	})

	t.Run("Cleanup respects permanent flag", func(t *testing.T) {
		// Create permanent instance
		instanceID := "inst_permanent"
		inst := &instance.Context{
			InstanceID:  instanceID,
			GameType:    "minecraft",
			Region:      "us-east-1",
			Status:      instance.StatusActive,
			CreatedAt:   time.Now().Add(-48 * time.Hour), // Very old
			LastActive:  time.Now().Add(-24 * time.Hour), // Inactive
			IsPermanent: true,
			Metadata:    make(map[string]string),
		}

		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Verify it cannot be auto-deleted
		assert.False(t, inst.CanBeAutoDeleted(), "Permanent instance should not be auto-deletable")

		// Clean up
		err = registry.Delete(ctx, instanceID)
		require.NoError(t, err)
	})
}

// TestInstanceResourceTracking tests accurate tracking of instance resources
func TestInstanceResourceTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	registry := instance.NewRegistry(testCache)

	t.Run("Track cache keys per instance", func(t *testing.T) {
		instanceID := "inst_resource_tracking"

		err := registry.Register(ctx, &instance.Context{
			InstanceID: instanceID,
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   make(map[string]string),
		})
		require.NoError(t, err)

		instanceCache := cache.NewInstanceCache(testCache, instanceID)

		// Add various types of data
		dataTypes := map[string]int{
			"player":  50,
			"world":   100,
			"config":  10,
			"session": 25,
			"temp":    15,
		}

		totalKeys := 0
		for prefix, count := range dataTypes {
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("%s:%d", prefix, i)
				value := fmt.Sprintf("value_%s_%d", prefix, i)
				err := instanceCache.Set(ctx, key, []byte(value), 0)
				require.NoError(t, err)
				totalKeys++
			}
		}

		// In a real implementation, we would query the instance's key count
		// For now, we verify by attempting to get all keys
		foundKeys := 0
		for prefix, count := range dataTypes {
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("%s:%d", prefix, i)
				_, err := instanceCache.Get(ctx, key)
				if err == nil {
					foundKeys++
				}
			}
		}

		assert.Equal(t, totalKeys, foundKeys, "Should track all keys accurately")
	})

	t.Run("Track memory usage estimation", func(t *testing.T) {
		instanceID := "inst_memory_tracking"

		err := registry.Register(ctx, &instance.Context{
			InstanceID: instanceID,
			GameType:   "terraria",
			Region:     "eu-west-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   make(map[string]string),
		})
		require.NoError(t, err)

		instanceCache := cache.NewInstanceCache(testCache, instanceID)

		// Add data of known sizes
		testData := []struct {
			key   string
			value []byte
		}{
			{"small", make([]byte, 100)},       // 100 bytes
			{"medium", make([]byte, 10*1024)},  // 10 KB
			{"large", make([]byte, 100*1024)},  // 100 KB
			{"xlarge", make([]byte, 500*1024)}, // 500 KB
		}

		expectedSize := 0
		for _, td := range testData {
			// Fill with pattern for verification
			for i := range td.value {
				td.value[i] = byte(i % 256)
			}

			err := instanceCache.Set(ctx, td.key, td.value, 0)
			require.NoError(t, err)

			expectedSize += len(td.key) + len(td.value)
		}

		// Verify data integrity
		for _, td := range testData {
			got, err := instanceCache.Get(ctx, td.key)
			require.NoError(t, err)
			assert.Equal(t, len(td.value), len(got), "Size should match for key %s", td.key)
		}

		// In a real implementation, we would track actual memory usage
		t.Logf("Expected minimum memory usage: %d bytes", expectedSize)
	})

	t.Run("Resource quota validation", func(t *testing.T) {
		instanceID := "inst_resource_quota"

		// Create instance with resource quota
		inst := &instance.Context{
			InstanceID: instanceID,
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			ResourceQuota: &instance.ResourceQuota{
				MaxMemoryMB:   1024, // 1GB
				MaxStorageGB:  10,   // 10GB
				MaxCPUCores:   2,
				MaxConcurrent: 100,
			},
			Metadata: make(map[string]string),
		}

		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Verify quota was stored
		retrieved, err := registry.Get(ctx, instanceID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.ResourceQuota)
		assert.Equal(t, int64(1024), retrieved.ResourceQuota.MaxMemoryMB)
		assert.Equal(t, int64(10), retrieved.ResourceQuota.MaxStorageGB)
	})
}

// BenchmarkInstanceLifecycle benchmarks lifecycle operations
func BenchmarkInstanceLifecycle(b *testing.B) {
	ctx := context.Background()
	registry := instance.NewRegistry(testCache)

	b.Run("Instance Creation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			inst := &instance.Context{
				InstanceID: fmt.Sprintf("bench_create_%d", i),
				GameType:   "minecraft",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
				LastActive: time.Now(),
				Metadata:   make(map[string]string),
			}
			registry.Register(ctx, inst)
		}
	})

	b.Run("Activity Updates", func(b *testing.B) {
		// Pre-create instance
		instanceID := "bench_activity"
		inst := &instance.Context{
			InstanceID: instanceID,
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   make(map[string]string),
		}
		registry.Register(ctx, inst)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry.UpdateLastActive(ctx, instanceID)
		}
	})

	b.Run("Status Updates", func(b *testing.B) {
		// Pre-create instance
		instanceID := "bench_status"
		inst := &instance.Context{
			InstanceID: instanceID,
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   make(map[string]string),
		}
		registry.Register(ctx, inst)

		statuses := []instance.InstanceStatus{
			instance.StatusActive,
			instance.StatusPaused,
			instance.StatusActive,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Get current instance
			current, _ := registry.Get(ctx, instanceID)
			if current != nil {
				status := statuses[i%len(statuses)]
				current.Status = status
				registry.Update(ctx, current)
			}
		}
	})
}
