package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstancePerformanceOverhead tests the performance impact of instance isolation
func TestInstancePerformanceOverhead(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	registry := instance.NewRegistry(testCache)

	// Pre-create instances
	const numInstances = 10
	for i := 0; i < numInstances; i++ {
		inst := &instance.Context{
			InstanceID: fmt.Sprintf("inst_perf_%d", i),
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   make(map[string]string),
		}
		err := registry.Register(ctx, inst)
		require.NoError(t, err)
	}

	t.Run("Single instance vs multi-instance overhead", func(t *testing.T) {
		const numOperations = 10000
		testData := []byte(`{"level":10,"score":1500,"items":["sword","shield","potion"]}`)

		// Baseline: Single instance performance
		singleInstance := cache.NewInstanceCache(testCache, "inst_perf_0")

		start := time.Now()
		for i := 0; i < numOperations; i++ {
			key := fmt.Sprintf("perf:key:%d", i)
			err := singleInstance.Set(ctx, key, testData, 0)
			require.NoError(t, err)
		}
		singleWriteDuration := time.Since(start)

		start = time.Now()
		for i := 0; i < numOperations; i++ {
			key := fmt.Sprintf("perf:key:%d", i)
			_, err := singleInstance.Get(ctx, key)
			require.NoError(t, err)
		}
		singleReadDuration := time.Since(start)

		// Multi-instance: Distribute operations across instances
		start = time.Now()
		for i := 0; i < numOperations; i++ {
			instanceID := fmt.Sprintf("inst_perf_%d", i%numInstances)
			instanceCache := cache.NewInstanceCache(testCache, instanceID)
			key := fmt.Sprintf("multi:key:%d", i)
			err := instanceCache.Set(ctx, key, testData, 0)
			require.NoError(t, err)
		}
		multiWriteDuration := time.Since(start)

		start = time.Now()
		for i := 0; i < numOperations; i++ {
			instanceID := fmt.Sprintf("inst_perf_%d", i%numInstances)
			instanceCache := cache.NewInstanceCache(testCache, instanceID)
			key := fmt.Sprintf("multi:key:%d", i)
			_, err := instanceCache.Get(ctx, key)
			require.NoError(t, err)
		}
		multiReadDuration := time.Since(start)

		// Calculate overhead
		writeOverhead := float64(multiWriteDuration-singleWriteDuration) / float64(singleWriteDuration) * 100
		readOverhead := float64(multiReadDuration-singleReadDuration) / float64(singleReadDuration) * 100

		t.Logf("Performance Results:")
		t.Logf("Single Instance - Write: %v, Read: %v", singleWriteDuration, singleReadDuration)
		t.Logf("Multi Instance  - Write: %v, Read: %v", multiWriteDuration, multiReadDuration)
		t.Logf("Overhead - Write: %.2f%%, Read: %.2f%%", writeOverhead, readOverhead)

		// Assert overhead is less than 5%
		assert.Less(t, writeOverhead, 5.0, "Write overhead should be less than 5%%")
		assert.Less(t, readOverhead, 5.0, "Read overhead should be less than 5%%")
	})

	t.Run("Key generation performance", func(t *testing.T) {
		const numIterations = 1000000

		// Benchmark key generation
		kb := instance.NewKeyBuilder("inst_keygen_test")

		start := time.Now()
		for i := 0; i < numIterations; i++ {
			_ = kb.CacheKey(fmt.Sprintf("key:%d", i))
		}
		keyGenDuration := time.Since(start)

		opsPerSecond := float64(numIterations) / keyGenDuration.Seconds()
		nanosPerOp := keyGenDuration.Nanoseconds() / int64(numIterations)

		t.Logf("Key Generation Performance:")
		t.Logf("Total time: %v", keyGenDuration)
		t.Logf("Ops/second: %.0f", opsPerSecond)
		t.Logf("Nanoseconds/op: %d", nanosPerOp)

		// Assert reasonable performance
		assert.Greater(t, opsPerSecond, 1000000.0, "Should generate at least 1M keys/second")
	})

	t.Run("Context extraction overhead", func(t *testing.T) {
		// Create test contexts
		contexts := make([]*instance.Context, numInstances)
		for i := 0; i < numInstances; i++ {
			contexts[i] = &instance.Context{
				InstanceID: fmt.Sprintf("inst_ctx_%d", i),
				GameType:   "minecraft",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
				LastActive: time.Now(),
				Metadata: map[string]string{
					"test": "value",
				},
			}
		}

		// Benchmark context injection and extraction
		const numIterations = 1000000

		start := time.Now()
		for i := 0; i < numIterations; i++ {
			ctx := context.Background()
			instCtx := contexts[i%numInstances]
			newCtx := instance.InjectContext(ctx, instCtx)
			extracted, ok := instance.ExtractContext(newCtx)
			assert.True(t, ok)
			assert.Equal(t, instCtx.InstanceID, extracted.InstanceID)
		}
		duration := time.Since(start)

		opsPerSecond := float64(numIterations) / duration.Seconds()
		nanosPerOp := duration.Nanoseconds() / int64(numIterations)

		t.Logf("Context Extraction Performance:")
		t.Logf("Total time: %v", duration)
		t.Logf("Ops/second: %.0f", opsPerSecond)
		t.Logf("Nanoseconds/op: %d", nanosPerOp)

		// Assert reasonable performance
		assert.Greater(t, opsPerSecond, 1000000.0, "Should handle at least 1M context operations/second")
	})

	t.Run("Registry operations under load", func(t *testing.T) {
		const numGoroutines = 50
		const opsPerGoroutine = 1000

		var getOps, updateOps int64
		var wg sync.WaitGroup

		start := time.Now()

		// Concurrent registry operations
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				instanceID := fmt.Sprintf("inst_perf_%d", goroutineID%numInstances)

				for i := 0; i < opsPerGoroutine; i++ {
					// Alternate between get and update
					if i%2 == 0 {
						_, err := registry.Get(ctx, instanceID)
						if err == nil {
							atomic.AddInt64(&getOps, 1)
						}
					} else {
						err := registry.UpdateLastActive(ctx, instanceID)
						if err == nil {
							atomic.AddInt64(&updateOps, 1)
						}
					}
				}
			}(g)
		}

		wg.Wait()
		duration := time.Since(start)

		totalOps := getOps + updateOps
		opsPerSecond := float64(totalOps) / duration.Seconds()

		t.Logf("Registry Performance Under Load:")
		t.Logf("Total operations: %d (Get: %d, Update: %d)", totalOps, getOps, updateOps)
		t.Logf("Duration: %v", duration)
		t.Logf("Ops/second: %.0f", opsPerSecond)

		// Assert good throughput
		assert.Greater(t, opsPerSecond, 10000.0, "Registry should handle at least 10k ops/second")
	})
}

// TestInstanceScalability tests the system's ability to scale with many instances
func TestInstanceScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	registry := instance.NewRegistry(testCache)

	t.Run("Scale to 100+ instances", func(t *testing.T) {
		const targetInstances = 100
		instanceIDs := make([]string, targetInstances)

		// Create instances
		start := time.Now()
		for i := 0; i < targetInstances; i++ {
			instanceIDs[i] = fmt.Sprintf("inst_scale_%d", i)
			inst := &instance.Context{
				InstanceID: instanceIDs[i],
				GameType:   "minecraft",
				Region:     fmt.Sprintf("region-%d", i%5),
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
				LastActive: time.Now(),
				Metadata: map[string]string{
					"index": fmt.Sprintf("%d", i),
				},
			}
			err := registry.Register(ctx, inst)
			require.NoError(t, err)
		}
		createDuration := time.Since(start)

		t.Logf("Created %d instances in %v", targetInstances, createDuration)

		// Concurrent operations on all instances
		const opsPerInstance = 100
		var totalOps int64
		var wg sync.WaitGroup

		start = time.Now()

		for _, instanceID := range instanceIDs {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				instanceCache := cache.NewInstanceCache(testCache, id)

				for i := 0; i < opsPerInstance; i++ {
					key := fmt.Sprintf("scale:key:%d", i)
					value := fmt.Sprintf("value_%d", i)

					// Write
					err := instanceCache.Set(ctx, key, []byte(value), 0)
					if err == nil {
						atomic.AddInt64(&totalOps, 1)
					}

					// Read
					_, err = instanceCache.Get(ctx, key)
					if err == nil {
						atomic.AddInt64(&totalOps, 1)
					}
				}
			}(instanceID)
		}

		wg.Wait()
		operationsDuration := time.Since(start)

		opsPerSecond := float64(totalOps) / operationsDuration.Seconds()

		t.Logf("Scalability Results:")
		t.Logf("Total operations: %d across %d instances", totalOps, targetInstances)
		t.Logf("Duration: %v", operationsDuration)
		t.Logf("Ops/second: %.0f", opsPerSecond)
		t.Logf("Ops/second/instance: %.0f", opsPerSecond/float64(targetInstances))

		// Verify linear scalability
		assert.Greater(t, opsPerSecond, 10000.0, "Should maintain high throughput with 100+ instances")
	})

	t.Run("Memory usage with many instances", func(t *testing.T) {
		// Get initial registry stats
		initialStats := registry.Stats()
		initialCacheSize := initialStats["memory_cache_size"].(int)

		// Create many instances with data
		const numInstances = 50
		const keysPerInstance = 100

		for i := 0; i < numInstances; i++ {
			instanceID := fmt.Sprintf("inst_mem_%d", i)
			inst := &instance.Context{
				InstanceID: instanceID,
				GameType:   "minecraft",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
				LastActive: time.Now(),
				Metadata:   make(map[string]string),
			}
			err := registry.Register(ctx, inst)
			require.NoError(t, err)

			// Add data to instance
			instanceCache := cache.NewInstanceCache(testCache, instanceID)
			for j := 0; j < keysPerInstance; j++ {
				key := fmt.Sprintf("mem:key:%d", j)
				value := make([]byte, 1024) // 1KB per key
				err := instanceCache.Set(ctx, key, value, 0)
				require.NoError(t, err)
			}
		}

		// Check final registry stats
		finalStats := registry.Stats()
		finalCacheSize := finalStats["memory_cache_size"].(int)

		t.Logf("Memory Usage Results:")
		t.Logf("Initial cache entries: %d", initialCacheSize)
		t.Logf("Final cache entries: %d", finalCacheSize)
		t.Logf("Growth: %d entries", finalCacheSize-initialCacheSize)

		// Check that the registry cache grew by the number of instances created
		growth := finalCacheSize - initialCacheSize
		assert.Equal(t, numInstances, growth, "Registry cache should grow by exactly the number of instances created")
	})
}

// TestInstanceLoadPatterns tests different load patterns
func TestInstanceLoadPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load pattern test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	registry := instance.NewRegistry(testCache)

	t.Run("Burst traffic pattern", func(t *testing.T) {
		// Create a few instances
		const numInstances = 5
		for i := 0; i < numInstances; i++ {
			inst := &instance.Context{
				InstanceID: fmt.Sprintf("inst_burst_%d", i),
				GameType:   "minecraft",
				Region:     "us-east-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
				LastActive: time.Now(),
				Metadata:   make(map[string]string),
			}
			err := registry.Register(ctx, inst)
			require.NoError(t, err)
		}

		// Simulate burst traffic
		const burstSize = 10000
		const numBursts = 3

		for burst := 0; burst < numBursts; burst++ {
			var wg sync.WaitGroup
			var successCount int64

			start := time.Now()

			// Send burst of requests
			for i := 0; i < burstSize; i++ {
				wg.Add(1)
				go func(reqID int) {
					defer wg.Done()

					instanceID := fmt.Sprintf("inst_burst_%d", reqID%numInstances)
					instanceCache := cache.NewInstanceCache(testCache, instanceID)

					key := fmt.Sprintf("burst:%d:%d", burst, reqID)
					value := fmt.Sprintf("value_%d", reqID)

					err := instanceCache.Set(ctx, key, []byte(value), 0)
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(start)

			successRate := float64(successCount) / float64(burstSize) * 100
			opsPerSecond := float64(successCount) / duration.Seconds()

			t.Logf("Burst %d Results:", burst+1)
			t.Logf("Success rate: %.2f%% (%d/%d)", successRate, successCount, burstSize)
			t.Logf("Duration: %v", duration)
			t.Logf("Ops/second: %.0f", opsPerSecond)

			// Should handle bursts well
			assert.Greater(t, successRate, 99.0, "Should handle burst traffic with >99%% success")

			// Brief pause between bursts
			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("Sustained load pattern", func(t *testing.T) {
		// Create instances for sustained load
		const numInstances = 10
		for i := 0; i < numInstances; i++ {
			inst := &instance.Context{
				InstanceID: fmt.Sprintf("inst_sustained_%d", i),
				GameType:   "terraria",
				Region:     "eu-west-1",
				Status:     instance.StatusActive,
				CreatedAt:  time.Now(),
				LastActive: time.Now(),
				Metadata:   make(map[string]string),
			}
			err := registry.Register(ctx, inst)
			require.NoError(t, err)
		}

		// Sustained load for a period
		const duration = 5 * time.Second
		const targetOpsPerSecond = 1000

		var totalOps int64
		done := make(chan struct{})

		// Start load generators
		for i := 0; i < numInstances; i++ {
			go func(workerID int) {
				instanceID := fmt.Sprintf("inst_sustained_%d", workerID)
				instanceCache := cache.NewInstanceCache(testCache, instanceID)

				ticker := time.NewTicker(time.Second / time.Duration(targetOpsPerSecond/numInstances))
				defer ticker.Stop()

				opCount := 0
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						key := fmt.Sprintf("sustained:%d:%d", workerID, opCount)
						value := fmt.Sprintf("value_%d", opCount)

						err := instanceCache.Set(ctx, key, []byte(value), 0)
						if err == nil {
							atomic.AddInt64(&totalOps, 1)
						}

						// Also do a read
						_, err = instanceCache.Get(ctx, key)
						if err == nil {
							atomic.AddInt64(&totalOps, 1)
						}

						opCount++
					}
				}
			}(i)
		}

		// Run for duration
		time.Sleep(duration)
		close(done)

		// Give goroutines time to finish
		time.Sleep(100 * time.Millisecond)

		actualOpsPerSecond := float64(totalOps) / duration.Seconds()

		t.Logf("Sustained Load Results:")
		t.Logf("Target ops/second: %d", targetOpsPerSecond*2) // *2 because we do read and write
		t.Logf("Actual ops/second: %.0f", actualOpsPerSecond)
		t.Logf("Total operations: %d", totalOps)

		// Should maintain target throughput
		targetTotal := float64(targetOpsPerSecond * 2)
		assert.Greater(t, actualOpsPerSecond, targetTotal*0.9, "Should maintain at least 90%% of target throughput")
	})
}

// BenchmarkInstanceOperations provides detailed performance benchmarks
func BenchmarkInstanceOperations(b *testing.B) {
	ctx := context.Background()
	registry := instance.NewRegistry(testCache)

	// Pre-create instances
	const numInstances = 10
	for i := 0; i < numInstances; i++ {
		inst := &instance.Context{
			InstanceID: fmt.Sprintf("bench_inst_%d", i),
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Metadata:   make(map[string]string),
		}
		registry.Register(ctx, inst)
	}

	b.Run("Cache Set", func(b *testing.B) {
		instanceCache := cache.NewInstanceCache(testCache, "bench_inst_0")
		value := []byte(`{"test":"data"}`)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench:key:%d", i)
			instanceCache.Set(ctx, key, value, 0)
		}
	})

	b.Run("Cache Get", func(b *testing.B) {
		instanceCache := cache.NewInstanceCache(testCache, "bench_inst_1")

		// Pre-populate data
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("bench:get:%d", i)
			value := fmt.Sprintf("value_%d", i)
			instanceCache.Set(ctx, key, []byte(value), 0)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench:get:%d", i%1000)
			instanceCache.Get(ctx, key)
		}
	})

	b.Run("Parallel Operations", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			threadID := 0
			for pb.Next() {
				instanceID := fmt.Sprintf("bench_inst_%d", threadID%numInstances)
				instanceCache := cache.NewInstanceCache(testCache, instanceID)

				key := fmt.Sprintf("parallel:key:%d", threadID)
				value := fmt.Sprintf("value_%d", threadID)

				instanceCache.Set(ctx, key, []byte(value), 0)
				instanceCache.Get(ctx, key)

				threadID++
			}
		})
	})

	b.Run("Registry Operations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			instanceID := fmt.Sprintf("bench_inst_%d", i%numInstances)
			registry.Get(ctx, instanceID)
		}
	})
}
