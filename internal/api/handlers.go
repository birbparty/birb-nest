package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/gofiber/fiber/v2"
)

// Handler holds all dependencies for API handlers
type Handler struct {
	db        *database.DB
	cache     cache.Cache
	queue     *queue.Client
	cacheRepo *database.CacheRepository
	metrics   *Metrics
}

// NewHandler creates a new handler instance
func NewHandler(db *database.DB, cache cache.Cache, queue *queue.Client, metrics *Metrics) *Handler {
	return &Handler{
		db:        db,
		cache:     cache,
		queue:     queue,
		cacheRepo: database.NewCacheRepository(db),
		metrics:   metrics,
	}
}

// GetCache handles GET /v1/cache/:key
func (h *Handler) GetCache(c *fiber.Ctx) error {
	ctx := c.UserContext()
	key := c.Params("key")

	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			NewErrorResponse("Key parameter is required", ErrCodeInvalidRequest),
		)
	}

	// 1. Try Redis first
	value, err := h.cache.Get(ctx, key)
	if err == nil {
		h.metrics.RecordCacheHit()

		// Parse the cached data
		var entry database.CacheEntry
		if err := json.Unmarshal(value, &entry); err == nil {
			return c.JSON(ConvertToCacheResponse(
				entry.Key,
				entry.Value,
				entry.Version,
				entry.TTL,
				entry.Metadata,
				entry.CreatedAt,
				entry.UpdatedAt,
			))
		}
	}

	// 2. Cache miss - try PostgreSQL
	h.metrics.RecordCacheMiss()
	entry, err := h.cacheRepo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// 3. Not in PostgreSQL either - trigger rehydration
			rehydrationMsg := queue.NewRehydrationMessage(key, queue.PriorityNormal)
			_ = h.queue.PublishRehydration(ctx, rehydrationMsg)

			return c.Status(fiber.StatusNotFound).JSON(
				NewErrorResponse("Cache entry not found", ErrCodeNotFound),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			NewErrorResponse("Failed to retrieve cache entry", ErrCodeInternalError),
		)
	}

	// Found in PostgreSQL - return and async rehydrate to Redis
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Marshal entry for caching
		if data, err := json.Marshal(entry); err == nil {
			ttl := time.Duration(0)
			if entry.TTL != nil && *entry.TTL > 0 {
				ttl = time.Duration(*entry.TTL) * time.Second
			}
			_ = h.cache.Set(ctx, key, data, ttl)
		}
	}()

	return c.JSON(ConvertToCacheResponse(
		entry.Key,
		entry.Value,
		entry.Version,
		entry.TTL,
		entry.Metadata,
		entry.CreatedAt,
		entry.UpdatedAt,
	))
}

// SetCache handles POST /v1/cache/:key
func (h *Handler) SetCache(c *fiber.Ctx) error {
	ctx := c.UserContext()
	key := c.Params("key")

	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			NewErrorResponse("Key parameter is required", ErrCodeInvalidRequest),
		)
	}

	// Parse request body
	var req CacheRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			NewErrorResponse("Invalid request body", ErrCodeInvalidRequest),
		)
	}

	// Convert metadata to JSON
	var metadata json.RawMessage
	if req.Metadata != nil {
		data, err := json.Marshal(req.Metadata)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				NewErrorResponse("Invalid metadata format", ErrCodeInvalidRequest),
			)
		}
		metadata = data
	}

	// Double-write pattern
	// 1. Write to Redis with TTL
	ttlDuration := time.Duration(0)
	if req.TTL != nil && *req.TTL > 0 {
		ttlDuration = time.Duration(*req.TTL) * time.Second
	}

	// Create entry for caching
	entry := database.CacheEntry{
		Key:       key,
		Value:     req.Value,
		Version:   1,
		TTL:       req.TTL,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Marshal entry for Redis
	cacheData, err := json.Marshal(entry)
	if err == nil {
		_ = h.cache.Set(ctx, key, cacheData, ttlDuration)
	}

	// 2. Publish to NATS for persistence
	persistMsg := queue.NewPersistenceMessage(key, req.Value, 1, req.TTL, metadata)
	if err := h.queue.PublishPersistence(ctx, persistMsg); err != nil {
		// Log but don't fail the request
		fmt.Printf("Failed to publish persistence message: %v\n", err)
	}

	// 3. Optionally, write directly to PostgreSQL for immediate consistency
	// (This can be removed if eventual consistency is acceptable)
	if err := h.cacheRepo.Set(ctx, key, req.Value, req.TTL, metadata); err != nil {
		// Log but don't fail since it's already in Redis
		fmt.Printf("Failed to persist to database: %v\n", err)
	}

	return c.Status(fiber.StatusCreated).JSON(ConvertToCacheResponse(
		key,
		req.Value,
		1,
		req.TTL,
		metadata,
		time.Now(),
		time.Now(),
	))
}

// UpdateCache handles PUT /v1/cache/:key
func (h *Handler) UpdateCache(c *fiber.Ctx) error {
	// For simplicity, update uses the same logic as set
	return h.SetCache(c)
}

// DeleteCache handles DELETE /v1/cache/:key
func (h *Handler) DeleteCache(c *fiber.Ctx) error {
	ctx := c.UserContext()
	key := c.Params("key")

	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			NewErrorResponse("Key parameter is required", ErrCodeInvalidRequest),
		)
	}

	// Delete from both Redis and PostgreSQL
	redisErr := h.cache.Delete(ctx, key)
	dbErr := h.cacheRepo.Delete(ctx, key)

	// If not found in either, return 404
	if errors.Is(redisErr, cache.ErrKeyNotFound) && errors.Is(dbErr, database.ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(
			NewErrorResponse("Cache entry not found", ErrCodeNotFound),
		)
	}

	// If any deletion succeeded, consider it successful
	return c.SendStatus(fiber.StatusNoContent)
}

// BatchGet handles POST /v1/cache/batch/get
func (h *Handler) BatchGet(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req BatchGetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			NewErrorResponse("Invalid request body", ErrCodeInvalidRequest),
		)
	}

	if len(req.Keys) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			NewErrorResponse("Keys array cannot be empty", ErrCodeInvalidRequest),
		)
	}

	response := &BatchGetResponse{
		Entries: make(map[string]*CacheResponse),
		Missing: []string{},
	}

	// Try Redis first
	redisResults, _ := h.cache.GetMultiple(ctx, req.Keys)

	// Track which keys we found
	foundKeys := make(map[string]bool)

	// Process Redis results
	for key, value := range redisResults {
		var entry database.CacheEntry
		if err := json.Unmarshal(value, &entry); err == nil {
			response.Entries[key] = ConvertToCacheResponse(
				entry.Key,
				entry.Value,
				entry.Version,
				entry.TTL,
				entry.Metadata,
				entry.CreatedAt,
				entry.UpdatedAt,
			)
			foundKeys[key] = true
		}
	}

	// Find missing keys
	var missingKeys []string
	for _, key := range req.Keys {
		if !foundKeys[key] {
			missingKeys = append(missingKeys, key)
		}
	}

	// Try PostgreSQL for missing keys
	if len(missingKeys) > 0 {
		dbEntries, _ := h.cacheRepo.BatchGet(ctx, missingKeys)
		for _, entry := range dbEntries {
			response.Entries[entry.Key] = ConvertToCacheResponse(
				entry.Key,
				entry.Value,
				entry.Version,
				entry.TTL,
				entry.Metadata,
				entry.CreatedAt,
				entry.UpdatedAt,
			)
			foundKeys[entry.Key] = true

			// Async rehydrate to Redis
			go func(e *database.CacheEntry) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if data, err := json.Marshal(e); err == nil {
					ttl := time.Duration(0)
					if e.TTL != nil && *e.TTL > 0 {
						ttl = time.Duration(*e.TTL) * time.Second
					}
					_ = h.cache.Set(ctx, e.Key, data, ttl)
				}
			}(entry)
		}
	}

	// Final missing keys
	for _, key := range req.Keys {
		if !foundKeys[key] {
			response.Missing = append(response.Missing, key)
		}
	}

	return c.JSON(response)
}

// Health handles GET /health
func (h *Handler) Health(c *fiber.Ctx) error {
	ctx := c.UserContext()

	checks := make(map[string]string)

	// Check database
	if err := h.db.Health(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
	} else {
		checks["database"] = "healthy"
	}

	// Check Redis
	if err := h.cache.Ping(ctx); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
	} else {
		checks["redis"] = "healthy"
	}

	// Check NATS
	if err := h.queue.Health(); err != nil {
		checks["nats"] = "unhealthy: " + err.Error()
	} else {
		checks["nats"] = "healthy"
	}

	// Overall status
	status := "healthy"
	for _, check := range checks {
		if check != "healthy" {
			status = "unhealthy"
			break
		}
	}

	response := &HealthResponse{
		Status:  status,
		Service: "birb-nest-api",
		Version: "1.0.0",
		Uptime:  time.Since(startTime).String(),
		Checks:  checks,
	}

	statusCode := fiber.StatusOK
	if status == "unhealthy" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(response)
}

// GetMetrics handles GET /metrics
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	return c.JSON(h.metrics.GetStats())
}

var startTime = time.Now()
