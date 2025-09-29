package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelper provides utilities for integration tests
type TestHelper struct {
	URL     string
	Client  *http.Client
	Cleanup func()
	ClearFn func()
	StopFn  func()
}

func (h *TestHelper) SetKey(key, value string) error {
	resp, err := h.Client.Post(
		h.URL+"/v1/cache/"+key,
		"application/octet-stream",
		bytes.NewReader([]byte(value)),
	)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (h *TestHelper) ClearRedis() {
	if h.ClearFn != nil {
		h.ClearFn()
	}
}

func (h *TestHelper) Stop() {
	if h.StopFn != nil {
		h.StopFn()
	}
}

func TestPrimaryMode(t *testing.T) {
	// Start primary instance
	primary := startTestInstance(t, map[string]string{
		"MODE":             "primary",
		"POSTGRES_ENABLED": "true",
	})
	defer primary.Cleanup()

	// Write data
	resp, err := http.Post(
		primary.URL+"/v1/cache/test-key",
		"application/octet-stream",
		bytes.NewReader([]byte("test-value")),
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify Redis has it immediately
	resp, err = http.Get(primary.URL + "/v1/cache/test-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "test-value", string(body))

	// Wait for async PostgreSQL write
	time.Sleep(200 * time.Millisecond)

	// Clear Redis to force PostgreSQL read
	primary.ClearRedis()

	// Should still get value from PostgreSQL
	resp, err = http.Get(primary.URL + "/v1/cache/test-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "test-value", string(body))
}

func TestReplicaMode(t *testing.T) {
	// Start primary
	primary := startTestInstance(t, map[string]string{
		"MODE": "primary",
	})
	defer primary.Cleanup()

	// Start replica
	replica := startTestInstance(t, map[string]string{
		"MODE":        "replica",
		"INSTANCE_ID": "game123",
		"PRIMARY_URL": primary.URL,
	})
	defer replica.Cleanup()

	// Write to replica
	resp, err := http.Post(
		replica.URL+"/v1/cache/test-key",
		"application/octet-stream",
		bytes.NewReader([]byte("replica-value")),
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Replica should have it in local Redis
	resp, err = http.Get(replica.URL + "/v1/cache/test-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "replica-value", string(body))

	// Wait for async forward to primary
	time.Sleep(200 * time.Millisecond)

	// Primary should also have it
	resp, err = http.Get(primary.URL + "/v1/cache/test-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "replica-value", string(body))
}

func TestReplicaCacheMiss(t *testing.T) {
	// Start primary with data
	primary := startTestInstance(t, map[string]string{
		"MODE": "primary",
	})
	defer primary.Cleanup()

	// Add data to primary
	primary.SetKey("shared-key", "shared-value")

	// Start replica
	replica := startTestInstance(t, map[string]string{
		"MODE":        "replica",
		"INSTANCE_ID": "game456",
		"PRIMARY_URL": primary.URL,
	})
	defer replica.Cleanup()

	// Replica cache miss should query primary
	resp, err := http.Get(replica.URL + "/v1/cache/shared-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "shared-value", string(body))

	// Now replica should have it cached
	primary.Stop() // Stop primary to ensure replica serves from cache

	resp, err = http.Get(replica.URL + "/v1/cache/shared-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "shared-value", string(body))
}

func TestHealthEndpoints(t *testing.T) {
	// Test primary health
	primary := startTestInstance(t, map[string]string{
		"MODE": "primary",
	})
	defer primary.Cleanup()

	resp, err := http.Get(primary.URL + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err = decodeJSON(resp.Body, &health)
	resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "primary", health["mode"])

	// Test replica health with primary connectivity
	replica := startTestInstance(t, map[string]string{
		"MODE":        "replica",
		"INSTANCE_ID": "health-test",
		"PRIMARY_URL": primary.URL,
	})
	defer replica.Cleanup()

	resp, err = http.Get(replica.URL + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	err = decodeJSON(resp.Body, &health)
	resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "replica", health["mode"])
	assert.Equal(t, "ok", health["primary_connectivity"])
}

func TestBatchGet(t *testing.T) {
	primary := startTestInstance(t, map[string]string{
		"MODE": "primary",
	})
	defer primary.Cleanup()

	// Add test data
	primary.SetKey("key1", "value1")
	primary.SetKey("key2", "value2")

	// Test batch get
	reqBody := bytes.NewBuffer([]byte(`{"keys": ["key1", "key2", "key3"]}`))
	resp, err := http.Post(
		primary.URL+"/v1/cache/batch/get",
		"application/json",
		reqBody,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Entries map[string]string `json:"entries"`
		Missing []string          `json:"missing"`
	}
	err = decodeJSON(resp.Body, &result)
	resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, "value1", result.Entries["key1"])
	assert.Equal(t, "value2", result.Entries["key2"])
	assert.Contains(t, result.Missing, "key3")
}

func TestWriteConflictResolution(t *testing.T) {
	primary := startTestInstance(t, map[string]string{
		"MODE": "primary",
	})
	defer primary.Cleanup()

	// Create two replicas
	replica1 := startTestInstance(t, map[string]string{
		"MODE":        "replica",
		"INSTANCE_ID": "replica1",
		"PRIMARY_URL": primary.URL,
	})
	defer replica1.Cleanup()

	replica2 := startTestInstance(t, map[string]string{
		"MODE":        "replica",
		"INSTANCE_ID": "replica2",
		"PRIMARY_URL": primary.URL,
	})
	defer replica2.Cleanup()

	// Both replicas write to same key
	go replica1.SetKey("conflict-key", "value-from-replica1")
	go replica2.SetKey("conflict-key", "value-from-replica2")

	// Wait for writes to propagate
	time.Sleep(500 * time.Millisecond)

	// Check final value on primary (last write wins)
	resp, err := http.Get(primary.URL + "/v1/cache/conflict-key")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Value should be from one of the replicas
	value := string(body)
	assert.True(t, value == "value-from-replica1" || value == "value-from-replica2")
}

// Helper functions for tests

func startTestInstance(t *testing.T, env map[string]string) *TestHelper {
	// This would typically use testcontainers or similar
	// For now, it's a placeholder
	t.Helper()
	return &TestHelper{
		URL:    "http://localhost:8080",
		Client: &http.Client{Timeout: 5 * time.Second},
		Cleanup: func() {
			// Cleanup logic
		},
		ClearFn: func() {
			// Clear Redis logic
		},
		StopFn: func() {
			// Stop instance logic
		},
	}
}

func decodeJSON(r io.Reader, v interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
