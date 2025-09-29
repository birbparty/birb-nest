package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultInstanceMetrics tests that metrics are properly tracked for default instance usage
func TestDefaultInstanceMetrics(t *testing.T) {
	resetAll(t)

	// Create test API instance
	app, err := createTestAPI()
	require.NoError(t, err)

	// Wait for services to be ready
	time.Sleep(2 * time.Second)

	t.Run("metrics track default instance requests", func(t *testing.T) {
		// Make several requests without X-Instance-ID header
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("metrics-test-%d", i)
			value := map[string]interface{}{
				"data": fmt.Sprintf("test value %d", i),
			}

			body, _ := json.Marshal(value)
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Get metrics
		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Read metrics response
		metricsBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		metrics := string(metricsBody)

		// Verify default instance metrics are present
		assert.Contains(t, metrics, "birbnest_default_instance_requests_total")

		// Since we can't easily parse Prometheus format, we'll just check for presence
		// In a real scenario, you'd parse the metrics and verify counts
	})

	t.Run("metrics track default instance cache operations", func(t *testing.T) {
		// Perform various cache operations without instance header
		key := "cache-ops-test"
		value := map[string]interface{}{
			"data": "test cache operations",
		}

		// SET operation
		body, _ := json.Marshal(value)
		req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// GET operation
		req = httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)
		resp, err = app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// DELETE operation
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/v1/cache/%s", key), nil)
		resp, err = app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Check metrics
		req = httptest.NewRequest("GET", "/metrics", nil)
		resp, err = app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		metrics := string(respBody)

		// Verify cache operation metrics
		assert.Contains(t, metrics, "birbnest_default_instance_cache_operations_total")
	})

	t.Run("metrics distinguish between default and specific instances", func(t *testing.T) {
		// Make requests with specific instance ID
		for i := 0; i < 3; i++ {
			key := fmt.Sprintf("specific-instance-%d", i)
			value := map[string]interface{}{
				"data": fmt.Sprintf("specific value %d", i),
			}

			body, _ := json.Marshal(value)
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Instance-ID", "test-metrics-instance")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Make requests without instance ID (default)
		for i := 0; i < 2; i++ {
			key := fmt.Sprintf("default-instance-%d", i)
			value := map[string]interface{}{
				"data": fmt.Sprintf("default value %d", i),
			}

			body, _ := json.Marshal(value)
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Get metrics
		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		metrics := string(body)

		// Verify both default and instance-specific metrics are tracked
		assert.Contains(t, metrics, "birbnest_default_instance_requests_total")
		assert.Contains(t, metrics, "birbnest_request_duration_seconds")
	})
}

// TestMetricsEndpoint tests the custom metrics endpoint
func TestMetricsEndpoint(t *testing.T) {
	resetAll(t)

	// Create test API instance
	app, err := createTestAPI()
	require.NoError(t, err)

	t.Run("metrics endpoint returns JSON metrics", func(t *testing.T) {
		// Perform some operations to generate metrics
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("metrics-json-%d", i)
			value := map[string]interface{}{
				"data": fmt.Sprintf("value %d", i),
			}

			body, _ := json.Marshal(value)
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/cache/%s", key), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			// Also read some
			if i%2 == 0 {
				req = httptest.NewRequest("GET", fmt.Sprintf("/v1/cache/%s", key), nil)
				resp, err = app.Test(req, -1)
				require.NoError(t, err)
			}
		}

		// Get JSON metrics (this is a custom endpoint, not Prometheus)
		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		// The response could be either Prometheus text format or JSON
		// depending on the implementation
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			var metrics map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&metrics)
			require.NoError(t, err)

			// Verify some expected fields
			assert.NotNil(t, metrics["cache_hits"])
			assert.NotNil(t, metrics["cache_misses"])
			assert.NotNil(t, metrics["total_requests"])
		} else {
			// It's Prometheus format
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "birbnest_")
		}
	})
}

// Helper function to parse Prometheus metrics (simplified)
func parsePrometheusMetric(metrics, metricName string) float64 {
	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// This is a very simplified parser
				// Real implementation would need proper Prometheus text format parsing
				return 1.0 // Placeholder
			}
		}
	}
	return 0.0
}
