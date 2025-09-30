package middleware

import (
	"net/http"
	"strings"

	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/gofiber/fiber/v2"
)

// InstanceMiddleware extracts and validates instance context from requests
type InstanceMiddleware struct {
	registry          *instance.Registry
	required          bool   // whether instance context is required
	defaultInstanceID string // default instance ID for requests without context
}

// NewInstanceMiddleware creates a new instance middleware
func NewInstanceMiddleware(registry *instance.Registry, required bool) *InstanceMiddleware {
	return &InstanceMiddleware{
		registry:          registry,
		required:          required,
		defaultInstanceID: "global", // Default value, can be overridden
	}
}

// SetDefaultInstanceID sets the default instance ID
func (m *InstanceMiddleware) SetDefaultInstanceID(id string) {
	m.defaultInstanceID = id
}

// Handle is the Fiber middleware function
func (m *InstanceMiddleware) Handle() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract instance ID from header or query parameter
		instanceID := m.extractInstanceID(c)

		// Use default instance ID if none provided
		var usingDefault bool
		if instanceID == "" {
			instanceID = m.defaultInstanceID
			usingDefault = true
		}

		// If no instance ID but required, return error
		if instanceID == "" && m.required {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "instance ID is required",
				"code":  "MISSING_INSTANCE_ID",
			})
		}

		// Load instance context from registry (creates if doesn't exist)
		instCtx, err := m.registry.GetOrCreate(c.Context(), instanceID)
		if err != nil {
			// Check if it's a not found error
			if err == instance.ErrInstanceNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "instance not found",
					"code":  "INSTANCE_NOT_FOUND",
				})
			}
			// Other errors
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to load instance context",
				"code":  "INSTANCE_LOAD_ERROR",
			})
		}

		// Mark global instance as permanent if it was just created
		if instanceID == m.defaultInstanceID && !instCtx.IsPermanent {
			instCtx.IsPermanent = true
			instCtx.Metadata["type"] = "default"
			instCtx.Metadata["created_by"] = "system"
			m.registry.Update(c.Context(), instCtx)
			// Set flag indicating default instance was created
			c.Locals("default_instance_created", true)
		}

		// Validate instance can accept requests
		if !instCtx.CanAcceptRequests() {
			statusCode := fiber.StatusServiceUnavailable
			errorMessage := "instance is not accepting requests"
			errorCode := "INSTANCE_UNAVAILABLE"

			// Specific handling for different statuses
			switch instCtx.Status {
			case instance.StatusDeleting:
				statusCode = fiber.StatusGone
				errorMessage = "instance is being deleted"
				errorCode = "INSTANCE_DELETING"
			case instance.StatusInactive:
				errorMessage = "instance is inactive"
				errorCode = "INSTANCE_INACTIVE"
			case instance.StatusPaused:
				errorMessage = "instance is paused"
				errorCode = "INSTANCE_PAUSED"
			}

			return c.Status(statusCode).JSON(fiber.Map{
				"error":       errorMessage,
				"code":        errorCode,
				"instance_id": instanceID,
				"status":      string(instCtx.Status),
			})
		}

		// Inject context into request context
		ctx := instance.InjectContext(c.Context(), instCtx)
		c.SetUserContext(ctx)

		// Store flag for metrics tracking
		if usingDefault {
			c.Locals("using_default_instance", true)
		}

		// Update last active asynchronously
		go m.registry.UpdateLastActive(c.Context(), instanceID)

		// Continue to next handler
		return c.Next()
	}
}

// extractInstanceID extracts the instance ID from the request
func (m *InstanceMiddleware) extractInstanceID(c *fiber.Ctx) string {
	// 1. Check header first (preferred)
	if instanceID := c.Get("X-Instance-ID"); instanceID != "" {
		return strings.TrimSpace(instanceID)
	}

	// 2. Check query parameter
	if instanceID := c.Query("instance_id"); instanceID != "" {
		return strings.TrimSpace(instanceID)
	}

	// 3. Check alternate query parameter
	if instanceID := c.Query("instanceId"); instanceID != "" {
		return strings.TrimSpace(instanceID)
	}

	return ""
}

// OptionalInstanceMiddleware creates middleware that doesn't require instance context
func OptionalInstanceMiddleware(registry *instance.Registry) fiber.Handler {
	m := NewInstanceMiddleware(registry, false)
	return m.Handle()
}

// RequiredInstanceMiddleware creates middleware that requires instance context
func RequiredInstanceMiddleware(registry *instance.Registry) fiber.Handler {
	m := NewInstanceMiddleware(registry, true)
	return m.Handle()
}

// ExtractInstanceContext is a helper to extract instance context from Fiber context
func ExtractInstanceContext(c *fiber.Ctx) (*instance.Context, bool) {
	return instance.ExtractContext(c.UserContext())
}

// ExtractInstanceID is a helper to extract just the instance ID from Fiber context
func ExtractInstanceID(c *fiber.Ctx) string {
	return instance.ExtractInstanceID(c.UserContext())
}

// MustExtractInstanceContext extracts instance context or panics
func MustExtractInstanceContext(c *fiber.Ctx) *instance.Context {
	ctx, ok := ExtractInstanceContext(c)
	if !ok {
		panic("instance context not found in request")
	}
	return ctx
}

// InstanceMetricsMiddleware adds instance-aware metrics to requests
func InstanceMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract instance ID from request context
		instanceID := instance.ExtractInstanceID(r.Context())
		if instanceID != "" {
			// Add instance ID to response headers for debugging
			w.Header().Set("X-Instance-ID", instanceID)
		}
		next.ServeHTTP(w, r)
	})
}
