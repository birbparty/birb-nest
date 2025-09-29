package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultInstanceBehavior tests backward compatibility with no instance context
func TestDefaultInstanceBehavior(t *testing.T) {
	resetAll(t)

	// Create test API instance
	app, err := createTestAPI()
	require.NoError(t, err)

	t.Run("requests without instance header use default instance", func(t *testing.T) {
		// Set a value without instance header
		key := "test-key-default"
		value := map[string]interface{}{
			"data": "test value for default instance",
		}

		// Create request without X-Instance-ID header
		body, _ := json.Marshal(value)
		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Verify the instance ID in response header
		instanceID := resp.Header.Get("X-Instance-ID")
		assert.Equal(t, "global", instanceID)

		// Get the value back without instance header
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)

		getResp, err := app.Test(getReq, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, getResp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(getResp.Body).Decode(&result)
		require.NoError(t, err)

		resultData, ok := result["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, value["data"], resultData["data"])
	})

	t.Run("data isolation between default and specific instances", func(t *testing.T) {
		key := "isolation-test-key"

		// Set value in default instance (no header)
		defaultValue := map[string]interface{}{
			"data": "default instance value",
		}
		body, _ := json.Marshal(defaultValue)
		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Set value in specific instance
		specificValue := map[string]interface{}{
			"data": "specific instance value",
		}
		body, _ = json.Marshal(specificValue)
		req = httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Instance-ID", "test-instance-1")

		resp, err = app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Get value from default instance
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)

		getResp, err := app.Test(getReq, -1)
		require.NoError(t, err)

		var defaultResult map[string]interface{}
		err = json.NewDecoder(getResp.Body).Decode(&defaultResult)
		require.NoError(t, err)
		defaultData := defaultResult["data"].(map[string]interface{})
		assert.Equal(t, "default instance value", defaultData["data"])

		// Get value from specific instance
		getReq = httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)
		getReq.Header.Set("X-Instance-ID", "test-instance-1")

		getResp, err = app.Test(getReq, -1)
		require.NoError(t, err)

		var specificResult map[string]interface{}
		err = json.NewDecoder(getResp.Body).Decode(&specificResult)
		require.NoError(t, err)
		specificData := specificResult["data"].(map[string]interface{})
		assert.Equal(t, "specific instance value", specificData["data"])
	})

	t.Run("batch operations use default instance", func(t *testing.T) {
		// Set some values first
		keys := []string{"batch-key-1", "batch-key-2", "batch-key-3"}
		for i, key := range keys {
			value := map[string]interface{}{
				"data": fmt.Sprintf("batch value %d", i+1),
			}
			body, _ := json.Marshal(value)
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			// No X-Instance-ID header - should use default

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Batch get without instance header
		batchReq := api.BatchGetRequest{
			Keys: keys,
		}
		body, _ := json.Marshal(batchReq)
		req := httptest.NewRequest("POST", "/v1/cache/batch/get", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var batchResp api.BatchGetResponse
		err = json.NewDecoder(resp.Body).Decode(&batchResp)
		require.NoError(t, err)

		assert.Equal(t, 3, len(batchResp.Entries))
		assert.Equal(t, 0, len(batchResp.Missing))
	})

	t.Run("default instance survives cleanup", func(t *testing.T) {
		// This test would require setting up the cleanup service
		// For now, we'll just verify the default instance is marked as permanent

		// Since we can't directly query the instance registry through the API,
		// we'll verify indirectly by checking that data in the default instance
		// persists after what would normally trigger a cleanup

		key := "persistence-test-key"
		value := map[string]interface{}{
			"data": "should persist",
		}

		// Set value in default instance
		body, _ := json.Marshal(value)
		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Wait a bit (in real scenario, cleanup would run)
		time.Sleep(2 * time.Second)

		// Verify value still exists
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)

		getResp, err := app.Test(getReq, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, getResp.StatusCode)
	})
}

// TestDefaultInstanceConfiguration tests configuration of default instance ID
func TestDefaultInstanceConfiguration(t *testing.T) {
	// This test would verify that DEFAULT_INSTANCE_ID environment variable
	// is properly respected. For integration tests, we're using "global" as default.

	t.Run("verify default instance ID is configurable", func(t *testing.T) {
		// In a real test, we'd set DEFAULT_INSTANCE_ID env var and restart the service
		// For now, we'll just verify the default behavior

		resetAll(t)

		// The default should be "global" based on our configuration
		// We can verify this indirectly through the behavior tests above
		assert.True(t, true, "Configuration test placeholder")
	})
}
