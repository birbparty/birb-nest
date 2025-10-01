package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/birbparty/birb-nest/internal/api/middleware"
	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/gofiber/fiber/v2"
)

// Handlers manages HTTP request handlers with instance awareness
type Handlers struct {
	cache           cache.Cache         // Original cache interface
	contextCache    *cache.ContextCache // Context-aware cache wrapper
	registry        *instance.Registry  // Instance registry
	asyncWriter     *AsyncWriter        // nil for replicas
	isPrimary       bool
	primaryURL      string // for replicas
	httpClient      *http.Client
	defaultInstance string // default instance ID for legacy support
	mode            string // "primary" or "replica"
}

// NewHandlers creates handlers based on deployment mode
func NewHandlers(cfg *Config, cacheClient cache.Cache, db database.Interface, registry *instance.Registry) *Handlers {
	h := &Handlers{
		cache:           cacheClient,
		contextCache:    cache.NewContextCache(cacheClient),
		registry:        registry,
		isPrimary:       cfg.Mode == "primary",
		mode:            cfg.Mode,
		primaryURL:      cfg.PrimaryURL,
		defaultInstance: cfg.InstanceID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Initialize async writer for primary mode
	if h.isPrimary && db != nil {
		h.asyncWriter = NewAsyncWriter(db, cfg.WriteQueueSize, cfg.WriteWorkers)
		InitializeAsyncMetrics(cfg.InstanceID, cfg.WriteQueueSize)
	}

	return h
}

// Set handles cache set operations with mode-aware behavior
func (h *Handlers) Set(c *fiber.Ctx) error {
	ctx := c.UserContext()
	key := c.Params("key")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "key is required",
		})
	}

	value := c.Body()
	timestamp := time.Now()

	// Extract instance context
	instCtx, hasInstance := middleware.ExtractInstanceContext(c)
	instanceID := h.defaultInstance
	if hasInstance {
		instanceID = instCtx.InstanceID
		// Update activity asynchronously
		go h.registry.UpdateLastActive(ctx, instanceID)
	}

	// Check for timestamp header from replicas
	if tsHeader := c.Get("X-Write-Timestamp"); tsHeader != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, tsHeader); err == nil {
			timestamp = parsed
		}
	}

	// 1. Always write to local Redis first (using context-aware cache)
	if err := h.contextCache.Set(ctx, key, value, 0); err != nil {
		RecordCacheOperation("set", "error", instanceID, h.mode)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to write to cache",
		})
	}
	RecordCacheOperation("set", "success", instanceID, h.mode)

	// 2. Handle based on mode
	if h.isPrimary {
		// Primary: async write to PostgreSQL
		if h.asyncWriter != nil {
			// Extract source instance ID from context or header
			sourceInstance := instanceID
			if instanceHeader := c.Get("X-Instance-ID"); instanceHeader != "" {
				sourceInstance = instanceHeader
			}
			h.asyncWriter.Write(ctx, key, value, sourceInstance)
		}
	} else {
		// Replica: forward to primary asynchronously
		go h.forwardWriteToPrimary(key, value, timestamp, instanceID)
	}

	return c.SendStatus(fiber.StatusOK)
}

// Get handles cache get operations with fallback logic
func (h *Handlers) Get(c *fiber.Ctx) error {
	ctx := c.UserContext()
	key := c.Params("key")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "key is required",
		})
	}

	// Extract instance context
	instCtx, hasInstance := middleware.ExtractInstanceContext(c)
	instanceID := h.defaultInstance
	if hasInstance {
		instanceID = instCtx.InstanceID
		// Update activity asynchronously
		go h.registry.UpdateLastActive(ctx, instanceID)
	}

	// 1. Always try local Redis first (using context-aware cache)
	value, err := h.contextCache.Get(ctx, key)
	if err == nil {
		RecordCacheOperation("get", "hit", instanceID, h.mode)
		return c.Send(value)
	}
	RecordCacheOperation("get", "miss", instanceID, h.mode)

	// 2. Cache miss - handle based on mode
	if h.isPrimary {
		// Primary checks PostgreSQL
		if h.asyncWriter != nil && h.asyncWriter.db != nil {
			value, err = h.asyncWriter.db.GetWithInstance(ctx, key, instanceID)
			if err != nil {
				return c.SendStatus(fiber.StatusNotFound)
			}

			// Repopulate cache
			h.contextCache.Set(ctx, key, value, 0)
			return c.Send(value)
		}
		return c.SendStatus(fiber.StatusNotFound)
	} else {
		// Replica queries primary
		return h.queryPrimary(c, key, instanceID)
	}
}

// Delete handles cache delete operations
func (h *Handlers) Delete(c *fiber.Ctx) error {
	ctx := c.UserContext()
	key := c.Params("key")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "key is required",
		})
	}

	// Extract instance context
	instCtx, hasInstance := middleware.ExtractInstanceContext(c)
	instanceID := h.defaultInstance
	if hasInstance {
		instanceID = instCtx.InstanceID
		// Update activity asynchronously
		go h.registry.UpdateLastActive(ctx, instanceID)
	}

	// Delete from local cache (using context-aware cache)
	err := h.contextCache.Delete(ctx, key)
	if err != nil {
		RecordCacheOperation("delete", "error", instanceID, h.mode)
	} else {
		RecordCacheOperation("delete", "success", instanceID, h.mode)
	}

	// Handle based on mode
	if h.isPrimary {
		// Primary: also delete from PostgreSQL
		if h.asyncWriter != nil && h.asyncWriter.db != nil {
			go func() {
				deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				h.asyncWriter.db.DeleteWithInstance(deleteCtx, key, instanceID)
			}()
		}
	} else {
		// Replica: forward delete to primary
		go h.forwardDeleteToPrimary(key, instanceID)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Health handles health check endpoint with mode awareness
func (h *Handlers) Health(c *fiber.Ctx) error {
	// Extract instance if available (health check doesn't require it)
	instanceID := h.defaultInstance
	if instCtx, ok := middleware.ExtractInstanceContext(c); ok {
		instanceID = instCtx.InstanceID
	}

	health := fiber.Map{
		"status":      "healthy",
		"mode":        h.mode,
		"instance_id": instanceID,
		"timestamp":   time.Now().Unix(),
	}

	healthValue := 1.0

	// Check Redis connectivity using context cache
	if err := h.contextCache.Ping(c.UserContext()); err != nil {
		health["status"] = "unhealthy"
		health["redis_error"] = err.Error()
		healthValue = 0.0
		UpdateHealthMetric(instanceID, h.mode, healthValue)
		return c.Status(fiber.StatusServiceUnavailable).JSON(health)
	}

	if h.isPrimary {
		// Primary: check PostgreSQL and async queue
		if h.asyncWriter != nil {
			stats := h.asyncWriter.Stats()
			health["async_queue"] = stats

			if stats.QueueDepth > int(float64(stats.QueueCapacity)*0.8) {
				health["status"] = "degraded"
				health["warning"] = "async queue near capacity"
				healthValue = 0.5
			}
		}
	} else {
		// Replica: check primary connectivity
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "GET", h.primaryURL+"/health", nil)
		resp, err := h.httpClient.Do(req)

		if err != nil {
			health["status"] = "degraded"
			health["primary_connectivity"] = "failed"
			health["primary_error"] = err.Error()
			healthValue = 0.5
		} else {
			resp.Body.Close()
			health["primary_connectivity"] = "ok"
			health["primary_status_code"] = resp.StatusCode
		}
	}

	UpdateHealthMetric(instanceID, h.mode, healthValue)

	statusCode := fiber.StatusOK
	if health["status"] != "healthy" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(health)
}

// Metrics handles metrics endpoint
func (h *Handlers) Metrics(c *fiber.Ctx) error {
	// Extract instance if available
	instanceID := h.defaultInstance
	if instCtx, ok := middleware.ExtractInstanceContext(c); ok {
		instanceID = instCtx.InstanceID
	}

	metrics := map[string]interface{}{
		"mode":        h.mode,
		"instance_id": instanceID,
	}

	if h.asyncWriter != nil {
		metrics["async_writer"] = h.asyncWriter.Stats()
	}

	return c.JSON(metrics)
}

// forwardWriteToPrimary asynchronously forwards writes from replica to primary
func (h *Handlers) forwardWriteToPrimary(key string, value []byte, timestamp time.Time, instanceID string) {
	url := fmt.Sprintf("%s/v1/cache/%s", h.primaryURL, key)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(value))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		RecordWriteForward(instanceID, "error")
		return
	}

	// Add instance ID and timestamp headers
	req.Header.Set("X-Instance-ID", instanceID)
	req.Header.Set("X-Write-Timestamp", timestamp.Format(time.RFC3339Nano))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to forward write to primary: %v", err)
		RecordWriteForward(instanceID, "error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Primary returned error status %d: %s", resp.StatusCode, string(body))
		RecordWriteForward(instanceID, "error")
	} else {
		RecordWriteForward(instanceID, "success")
	}
}

// forwardDeleteToPrimary asynchronously forwards delete from replica to primary
func (h *Handlers) forwardDeleteToPrimary(key string, instanceID string) {
	url := fmt.Sprintf("%s/v1/cache/%s", h.primaryURL, key)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("Failed to create delete request: %v", err)
		return
	}

	req.Header.Set("X-Instance-ID", instanceID)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to forward delete to primary: %v", err)
		return
	}
	defer resp.Body.Close()
}

// queryPrimary queries the primary instance on cache miss
func (h *Handlers) queryPrimary(c *fiber.Ctx, key string, instanceID string) error {
	url := fmt.Sprintf("%s/v1/cache/%s", h.primaryURL, key)

	req, err := http.NewRequestWithContext(c.UserContext(), "GET", url, nil)
	if err != nil {
		RecordPrimaryQuery(instanceID, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create primary request",
		})
	}

	req.Header.Set("X-Instance-ID", instanceID)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to query primary: %v", err)
		RecordPrimaryQuery(instanceID, "error")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Primary unavailable",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		RecordPrimaryQuery(instanceID, "not_found")
		return c.SendStatus(fiber.StatusNotFound)
	}

	if resp.StatusCode != http.StatusOK {
		RecordPrimaryQuery(instanceID, "error")
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"error": "Primary returned error",
		})
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		RecordPrimaryQuery(instanceID, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read primary response",
		})
	}

	RecordPrimaryQuery(instanceID, "success")

	// Cache it locally for future reads
	h.contextCache.Set(c.UserContext(), key, body, 0)

	return c.Send(body)
}

// BatchGet handles batch get operations
func (h *Handlers) BatchGet(c *fiber.Ctx) error {
	ctx := c.UserContext()
	var req struct {
		Keys []string `json:"keys"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.Keys) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Keys array cannot be empty",
		})
	}

	// Extract instance context (optional for batch)
	instanceID := h.defaultInstance
	if instCtx, ok := middleware.ExtractInstanceContext(c); ok {
		instanceID = instCtx.InstanceID
		// Update activity asynchronously
		go h.registry.UpdateLastActive(ctx, instanceID)
	}

	results := make(map[string]json.RawMessage)
	missing := []string{}

	// Get from local cache (using context-aware cache)
	cacheResults, _ := h.contextCache.GetMultiple(ctx, req.Keys)

	for _, key := range req.Keys {
		if value, ok := cacheResults[key]; ok {
			results[key] = json.RawMessage(value)
		} else {
			missing = append(missing, key)
		}
	}

	// For missing keys, handle based on mode
	if len(missing) > 0 && !h.isPrimary {
		// Replica: query primary for missing keys
		for _, key := range missing {
			// Could optimize this with a batch endpoint on primary
			queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			url := fmt.Sprintf("%s/v1/cache/%s", h.primaryURL, key)
			req, _ := http.NewRequestWithContext(queryCtx, "GET", url, nil)
			req.Header.Set("X-Instance-ID", instanceID)

			resp, err := h.httpClient.Do(req)
			cancel()

			if err == nil && resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				results[key] = json.RawMessage(body)
				// Cache locally
				h.contextCache.Set(ctx, key, body, 0)
			}
		}
	}

	// Recalculate missing
	finalMissing := []string{}
	for _, key := range req.Keys {
		if _, ok := results[key]; !ok {
			finalMissing = append(finalMissing, key)
		}
	}

	return c.JSON(fiber.Map{
		"entries": results,
		"missing": finalMissing,
	})
}

// Shutdown gracefully shuts down the handlers
func (h *Handlers) Shutdown() {
	if h.asyncWriter != nil {
		h.asyncWriter.Shutdown()
	}
}
